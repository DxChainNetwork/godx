// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"container/heap"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/p2p/netutil"
)

const (
	// This is the amount of time spent waiting in between
	// redialing a certain node.
	dialHistoryExpiration = 30 * time.Second

	// Discovery lookups are throttled and can only run
	// once every few seconds.
	lookupInterval = 4 * time.Second

	// If no peers are found for this amount of time, the initial bootnodes are
	// attempted to be connected.
	fallbackInterval = 20 * time.Second

	// Endpoint resolution is throttled with bounded backoff.
	initialResolveDelay = 60 * time.Second
	maxResolveDelay     = time.Hour
)

// NodeDialer is used to connect to nodes in the network, typically by using
// an underlying net.Dialer but also using net.Pipe in tests
type NodeDialer interface {
	Dial(*enode.Node) (net.Conn, error)
}

// TCPDialer implements the NodeDialer interface by using a net.Dialer to
// create TCP connections to nodes in the network
type TCPDialer struct {
	*net.Dialer
}

// Dial creates a TCP connection to the node
// based on the node information stored in the Node Object
func (t TCPDialer) Dial(dest *enode.Node) (net.Conn, error) {
	// set up TCP address
	addr := &net.TCPAddr{IP: dest.IP(), Port: dest.TCP()}
	return t.Dialer.Dial("tcp", addr.String())
}

// dialstate schedules dials and discovery lookups.
// it get's a chance to compute new tasks on every iteration
// of the main loop in Server.run.
type dialstate struct {
	// max dynamic dials allowed to make
	maxDynDials int

	// table defined in the discover protocol, nodes within certain distance
	// will be added to table
	ntab discoverTable

	// list of IP network, it is a whitelist. if it is not empty and the node's IP
	// is not contained inside, then, localhost is not allowed to establish connection
	// to the node
	netrestrict *netutil.Netlist

	// localhost Node ID
	self enode.ID

	lookupRunning bool
	dialing       map[enode.ID]connFlag

	// current discovery lookup results
	lookupBuf []*enode.Node

	// filled from Table
	randomNodes []*enode.Node
	static      map[enode.ID]*dialTask
	storage     map[enode.ID]*dialTask
	hist        *dialHistory

	// time when the dialer was first used
	start time.Time

	// default dials when there are no peers
	bootnodes []*enode.Node
}

type discoverTable interface {
	Close()
	Resolve(*enode.Node) *enode.Node
	LookupRandom() []*enode.Node
	ReadRandomNodes([]*enode.Node) int
}

// the dial history remembers recent dials.
type dialHistory []pastDial

// pastDial is an entry in the dial history.
type pastDial struct {
	id  enode.ID  // node id
	exp time.Time // expire time
}

type task interface {
	Do(*Server)
}

// A dialTask is generated for each node that is dialed. Its
// fields cannot be accessed while the task is running.
type dialTask struct {
	flags        connFlag    // connection flag
	dest         *enode.Node // node trying to establish the connection to
	lastResolved time.Time
	resolveDelay time.Duration
}

// discoverTask runs discovery table operations.
// Only one discoverTask is active at any time.
// discoverTask.Do performs a random lookup.
//
// results stored the random nodes found in the network
type discoverTask struct {
	results []*enode.Node
}

// A waitExpireTask is generated if there are no other tasks
// to keep the loop in Server.run ticking.
type waitExpireTask struct {
	time.Duration
}

func newDialState(self enode.ID, static []*enode.Node, bootnodes []*enode.Node, ntab discoverTable, maxdyn int, netrestrict *netutil.Netlist) *dialstate {
	s := &dialstate{
		maxDynDials: maxdyn,
		ntab:        ntab,
		self:        self,
		netrestrict: netrestrict,
		static:      make(map[enode.ID]*dialTask),
		storage:     make(map[enode.ID]*dialTask),
		dialing:     make(map[enode.ID]connFlag),
		bootnodes:   make([]*enode.Node, len(bootnodes)),
		randomNodes: make([]*enode.Node, maxdyn/2),
		hist:        new(dialHistory),
	}
	copy(s.bootnodes, bootnodes)
	for _, n := range static {
		s.addStatic(n)
	}
	return s
}

// Add dialTask to static field
// The dialTask consist connection flag and the destination Node structure
func (s *dialstate) addStatic(n *enode.Node) {
	// This overwrites the task instead of updating an existing
	// entry, giving users the opportunity to force a resolve operation.
	s.static[n.ID()] = &dialTask{flags: staticDialedConn, dest: n}
}

// remove the node from both dialTask and dial history
func (s *dialstate) removeStatic(n *enode.Node) {
	// This removes a task so future attempts to connect will not be made.
	delete(s.static, n.ID())
	// This removes a previous dial timestamp so that application
	// can force a server to reconnect with chosen peer immediately.
	s.hist.remove(n.ID())
}

// Add storage dialTask
// The storage independent peer with eth peer(connection)
// This is the method that avoids the single peer between two enode restriction
func (s *dialstate) addStorage(n *enode.Node) {
	s.storage[n.ID()] = &dialTask{flags: storageContractConn | storageClientConn, dest: n}
}

// remove the storage dial task
func (s *dialstate) removeStorage(n *enode.Node) {
	delete(s.storage, n.ID())
	s.hist.remove(n.ID())
}

// there are three type of tasks:
// 1. dial task
// 2. discover task
// 3. waitExpireTask
//
// Create and add the type of tasks mentioned above to the newtasks list and return
func (s *dialstate) newTasks(nRunning int, peers map[enode.ID]*Peer, storagePeers map[enode.ID]*Peer, now time.Time) []task {
	// if the dialer has not been used, assign the current time to it
	if s.start.IsZero() {
		s.start = now
	}

	var newtasks []task

	// check and add a node to dialing list
	// creating a dialTask and add it to newtasks list
	// returns if a node is successfully added to the dialing list
	addDial := func(flag connFlag, n *enode.Node) bool {
		// check if dial to the node is allowed
		t := &dialTask{flags: flag, dest: n}
		if err := s.checkDial(t, peers, storagePeers); err != nil {
			log.Trace("Skipping dial candidate", "id", n.ID(), "addr", &net.TCPAddr{IP: n.IP(), Port: n.TCP()}, "err", err)
			return false
		}

		// if allowed, add the node to the dialing list along with the flag
		s.dialing[n.ID()] = flag

		// create a dialTask and added it to the newtasks
		newtasks = append(newtasks, &dialTask{flags: flag, dest: n})
		return true
	}

	// Compute number of dynamic dials necessary at this point.
	// by subtracting the dynamic dials contained in peers with dials in dialing
	// to get how many dynamic dials have left
	needDynDials := s.maxDynDials
	for _, p := range peers {
		// if the peer node's connection is dynDialedConn, decrease needDynDials
		if p.rw.is(dynDialedConn) {
			needDynDials--
		}
	}

	for _, flag := range s.dialing {
		// flag&dynDialedConn == 0 only if flag is staticDialedConn 10
		// if flag in the node dialing list is not staticDialedConn, decrease the needDynDials
		if flag&dynDialedConn != 0 {
			needDynDials--
		}
	}

	// Expire the dial history on every invocation.
	// remove all expired node from dialhistory
	s.hist.expire(now)

	// Create dials for static nodes if they are not connected.
	// adding the qualified node from static to the dialing list and adding
	// the dialtask to newtasks
	for id, t := range s.static {
		err := s.checkDial(t, peers, storagePeers)
		switch err {
		case errNotWhitelisted, errSelf:
			log.Warn("Removing static dial candidate", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP(), Port: t.dest.TCP()}, "err", err)
			delete(s.static, t.dest.ID())
		case nil:
			s.dialing[id] = t.flags
			newtasks = append(newtasks, t)
		}
	}

	for id, t := range s.storage {
		log.Info("[STORAGE] peer task", "enodeURL", t.dest.String(), "flag", t.flags.String())
		err := s.checkDial(t, peers, storagePeers)
		switch err {
		case errNotWhitelisted, errSelf:
			log.Error("Removing storage dial candidate", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP(), Port: t.dest.TCP()}, "err", err)
			delete(s.storage, t.dest.ID())
		case nil:
			s.dialing[id] = t.flags
			newtasks = append(newtasks, t)
			delete(s.storage, t.dest.ID())
		}
	}

	// If we don't have any peers whatsoever, try to dial a random bootnode. This
	// scenario is useful for the testnet (and private networks) where the discovery
	// table might be full of mostly bad peers, making it hard to find good ones.
	//
	// if peers cannot be found within 20 seconds, and bootstrap nodes are defined, and there are
	// dynamic dials available
	// the first bootnode will be used and then be placed at the end of the list of bootnode
	if len(peers) == 0 && len(s.bootnodes) > 0 && needDynDials > 0 && now.Sub(s.start) > fallbackInterval {
		bootnode := s.bootnodes[0]
		s.bootnodes = append(s.bootnodes[:0], s.bootnodes[1:]...)
		s.bootnodes = append(s.bootnodes, bootnode)

		// adding the bootnode to dialing and newtasks
		if addDial(dynDialedConn, bootnode) {
			needDynDials--
		}
	}
	// Use random nodes from the table for half of the necessary
	// dynamic dials.
	randomCandidates := needDynDials / 2
	if randomCandidates > 0 {
		// fills s.randomNodes slice with nodes randomly picked from the discoverTable
		n := s.ntab.ReadRandomNodes(s.randomNodes)
		// add those nodes to dialing list and newtasks
		for i := 0; i < randomCandidates && i < n; i++ {
			if addDial(dynDialedConn, s.randomNodes[i]) {
				needDynDials--
			}
		}
	}
	// Create dynamic dials from random lookup results, removing tried
	// items from the result buffer.
	i := 0
	// Three Conditions:
	// 1. items left in lookupBuf, but no more needDynDials
	// 2. needDynDials is still available, but no items left in the lookupBuf
	// 3. no more needDynDials, no more items in lookupBuf
	for ; i < len(s.lookupBuf) && needDynDials > 0; i++ {
		if addDial(dynDialedConn, s.lookupBuf[i]) {
			needDynDials--
		}
	}
	// update lookupBuf, remove those added to the dialing list and newtasks
	s.lookupBuf = s.lookupBuf[:copy(s.lookupBuf, s.lookupBuf[i:])]

	// Launch a discovery lookup if more candidates are needed.
	// it means needDynDials is still not 0, set lookupRunning to be true
	// asking for more candidates. Adding discoverTask to newtasks
	if len(s.lookupBuf) < needDynDials && !s.lookupRunning {
		s.lookupRunning = true
		newtasks = append(newtasks, &discoverTask{})
	}

	// Launch a timer to wait for the next node to expire if all
	// candidates have been tried and no task is currently active.
	// This should prevent cases where the dialer logic is not ticked
	// because there are no pending events.
	//
	// if there are no newtasks, add a task for waiting for the nodes contained in dial history to be expired
	// used to prevent the dialer logic is not ticked, always give it some tasks
	if nRunning == 0 && len(newtasks) == 0 && s.hist.Len() > 0 {
		t := &waitExpireTask{s.hist.min().exp.Sub(now)}
		newtasks = append(newtasks, t)
	}
	return newtasks
}

// defined a bunch of checkDial errors
var (
	errSelf                    = errors.New("is self")
	errAlreadyDialing          = errors.New("already dialing")
	errEthAlreadyConnected     = errors.New("already eth connected")
	errStorageAlreadyConnected = errors.New("already storage connected")
	errRecentlyDialed          = errors.New("recently dialed")
	errNotWhitelisted          = errors.New("not contained in netrestrict whitelist")
)

// Error Checking before adding a node to the dial list
// Check if a node is dial-able
func (s *dialstate) checkDial(t *dialTask, peers map[enode.ID]*Peer, storagePeers map[enode.ID]*Peer) error {
	n := t.dest
	flag := t.flags
	// check if the node existed in the dialing list
	_, dialing := s.dialing[n.ID()]
	switch {
	// if currently dialing to that node, return error
	case dialing:
		return errAlreadyDialing
	// if the node can be found within peers, return error
	case flag&storageContractConn == 0 && peers[n.ID()] != nil:
		return errEthAlreadyConnected
	// if the node is about storage, we check previous the log
	case flag&storageContractConn != 0 && storagePeers[n.ID()] != nil:
		return errStorageAlreadyConnected
	// if the node id is same as my node id, return error
	case n.ID() == s.self:
		return errSelf
	// netrestrict list is not empty and the node's IP is not contained inside, return error
	case s.netrestrict != nil && !s.netrestrict.Contains(n.IP()):
		return errNotWhitelisted
	// if the node is contained in dial history, return error
	case flag&storageContractConn == 0 && s.hist.contains(n.ID()):
		return errRecentlyDialed
	}
	return nil
}

// based on the task type, handle the results after task finished
// only two of three task type will be handled, which are:
// dialTask and discoverTask
func (s *dialstate) taskDone(t task, now time.Time) {
	switch t := t.(type) {
	// if a dialTask was done, add the node to the dialhistory with expiration 30 seconds
	// meaning re-dial can only happen after 30 seconds. Delete from the dialing list
	case *dialTask:
		s.hist.add(t.dest.ID(), now.Add(dialHistoryExpiration))
		delete(s.dialing, t.dest.ID())
	// if a discoverTask was done, set the lookupRunning to be false
	// and add the results to the lookupBuf
	case *discoverTask:
		s.lookupRunning = false
		s.lookupBuf = append(s.lookupBuf, t.results...)
	}
}

// establish connection to the Node secured by the RLPx protocol
func (t *dialTask) Do(srv *Server) {
	// check if the node is incomplete (no IP address)
	if t.dest.Incomplete() {
		if !t.resolve(srv) {
			return
		}
	}
	err := t.dial(srv, t.dest)
	if err != nil {
		log.Trace("Dial error", "task", t, "err", err)
		// Try resolving the ID of static nodes if dialing failed.
		// Static IP is coming from a configuration file which may changed
		if _, ok := err.(*dialError); ok && t.flags&staticDialedConn != 0 {
			if t.resolve(srv) {
				t.dial(srv, t.dest)
			}
		}
	}
}

// resolve attempts to find the current endpoint for the destination
// using discovery.
//
// Resolve operations are throttled with backoff to avoid flooding the
// discovery network with useless queries for nodes that don't exist.
// The backoff delay resets when the node is found.
func (t *dialTask) resolve(srv *Server) bool {
	// if discover table is empty, then the endpoint for the dest node cannot be found with discovery
	// return false
	if srv.ntab == nil {
		log.Debug("Can't resolve node", "id", t.dest.ID, "err", "discovery is disabled")
		return false
	}
	// set resolveDelay to 1 min if it was not set
	if t.resolveDelay == 0 {
		t.resolveDelay = initialResolveDelay
	}
	// if the resolve function was called within resolveDelay time, return false
	if time.Since(t.lastResolved) < t.resolveDelay {
		return false
	}
	// returns the Node object with given node id. The node information was retrieved from the discover table
	resolved := srv.ntab.Resolve(t.dest)

	// set the lastResolved time to be current time
	t.lastResolved = time.Now()

	// is resolved is nil meaning node cannot be found from the table
	// then double the resolveDelay (the max delay is an hour)
	// return false
	if resolved == nil {
		t.resolveDelay *= 2
		if t.resolveDelay > maxResolveDelay {
			t.resolveDelay = maxResolveDelay
		}
		log.Debug("Resolving node failed", "id", t.dest.ID, "newdelay", t.resolveDelay)
		return false
	}
	// The node was found. set the delay to be 1 min, set the node to the one received from the table, return true
	t.resolveDelay = initialResolveDelay
	t.dest = resolved
	log.Debug("Resolved node", "id", t.dest.ID, "addr", &net.TCPAddr{IP: t.dest.IP(), Port: t.dest.TCP()})
	return true
}

type dialError struct {
	error
}

// dial performs the actual connection attempt.
func (t *dialTask) dial(srv *Server, dest *enode.Node) error {
	// returns the tcp connection, if TCP dialer was used
	fd, err := srv.Dialer.Dial(dest)
	if err != nil {
		return &dialError{err}
	}
	mfd := newMeteredConn(fd, false, dest.IP())
	return srv.SetupConn(mfd, t.flags, dest)
}

// returns connection Flags, first 7 bytes of connection id, IP address and TCP port
// on the node trying to establish the connection
func (t *dialTask) String() string {
	id := t.dest.ID()
	return fmt.Sprintf("%v %x %v:%d", t.flags, id[:8], t.dest.IP(), t.dest.TCP())
}

// find random nodes in the network and store them into discoverTask.result field
func (t *discoverTask) Do(srv *Server) {
	// newTasks generates a lookup task whenever dynamic dials are
	// necessary. Lookups need to take some time, otherwise the
	// event loop spins too fast.
	//
	// next lookup time is 4 seconds after the last lookup time
	next := srv.lastLookup.Add(lookupInterval)

	// within in 4 seconds, sleep until 4 seconds reached
	if now := time.Now(); now.Before(next) {
		time.Sleep(next.Sub(now))
	}

	// set the last lookup time to the current time and start to lookup
	// random nodes in the network
	srv.lastLookup = time.Now()
	t.results = srv.ntab.LookupRandom()
}

// returns a string contained the number of nodes randomly found in the network
func (t *discoverTask) String() string {
	s := "discovery lookup"
	if len(t.results) > 0 {
		s += fmt.Sprintf(" (%d results)", len(t.results))
	}
	return s
}

// when there are no active tasks, add waitExpireTask
// put it to sleep until the next object in the dialerHistory reached
// its' expiration time
func (t waitExpireTask) Do(*Server) {
	time.Sleep(t.Duration)
}

// string returns how much time will be wait for dial history expire
func (t waitExpireTask) String() string {
	return fmt.Sprintf("wait for dial hist expire (%v)", t.Duration)
}

// Use only these methods to access or modify dialHistory.

// returns the pastDial object from dialHistory with the minimum expiration time
// which is the first object of the list
func (h dialHistory) min() pastDial {
	return h[0]
}

// add the node to the history
func (h *dialHistory) add(id enode.ID, exp time.Time) {
	heap.Push(h, pastDial{id, exp})

}

// remove the node from the history
func (h *dialHistory) remove(id enode.ID) bool {
	for i, v := range *h {
		if v.id == id {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}

// check if a node contained in the dial history
func (h dialHistory) contains(id enode.ID) bool {
	for _, v := range h {
		if v.id == id {
			return true
		}
	}
	return false
}

// if the node contained in the dialhistory expired
// remove it from the dialhistory list
// the smaller the index is, the earlier it added to the dialhistory (dialed)
func (h *dialHistory) expire(now time.Time) {
	for h.Len() > 0 && h.min().exp.Before(now) {
		heap.Pop(h)
	}
}

// heap.Interface boilerplate
func (h dialHistory) Len() int           { return len(h) }
func (h dialHistory) Less(i, j int) bool { return h[i].exp.Before(h[j].exp) }
func (h dialHistory) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

// stores a pastDial record to the end of the dialHistory list
func (h *dialHistory) Push(x interface{}) {
	*h = append(*h, x.(pastDial))
}

// pop the first pastDial from the dialHistory list
func (h *dialHistory) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
