// Copyright 2014 The go-ethereum Authors
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

// Package p2p implements the Ethereum p2p network protocols.
package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/mclock"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/discover"
	"github.com/DxChainNetwork/godx/p2p/discv5"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/p2p/enr"
	"github.com/DxChainNetwork/godx/p2p/nat"
	"github.com/DxChainNetwork/godx/p2p/netutil"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	defaultDialTimeout = 15 * time.Second

	// Connectivity defaults.
	maxActiveDialTasks     = 16
	defaultMaxPendingPeers = 50
	defaultDialRatio       = 3

	// Maximum time allowed for reading a complete message.
	// This is effectively the amount of time a connection can be idle.
	frameReadTimeout = 30 * time.Second

	// Maximum amount of time allowed for writing a complete message.
	frameWriteTimeout = 20 * time.Second
)

var errServerStopped = errors.New("server stopped")

// Config holds Server options.
type Config struct {
	// This field must be set to a valid secp256k1 private key.
	//REQUIRED
	PrivateKey *ecdsa.PrivateKey `toml:"-"`

	// MaxPeers is the maximum number of peers that can be
	// connected. It must be greater than zero.
	MaxPeers int

	// MaxPendingPeers is the maximum number of peers that can be pending in the
	// handshake phase, counted separately for inbound and outbound connections.
	// Zero defaults to preset values.
	MaxPendingPeers int `toml:",omitempty"`

	// DialRatio controls the ratio of inbound to dialed connections.
	// Example: a DialRatio of 2 allows 1/2 of connections to be dialed.
	// Setting DialRatio to zero defaults it to 3.
	DialRatio int `toml:",omitempty"`

	// NoDiscovery can be used to disable the peer discovery mechanism.
	// Disabling is useful for protocol debugging (manual topology).
	NoDiscovery bool

	// DiscoveryV5 specifies whether the new topic-discovery based V5 discovery
	// protocol should be started or not.
	DiscoveryV5 bool `toml:",omitempty"`

	// Name sets the node name of this server.
	// Use common.MakeName to create a name that follows existing conventions.
	Name string `toml:"-"`

	// BootstrapNodes are used to establish connectivity
	// with the rest of the network.
	BootstrapNodes []*enode.Node

	// BootstrapNodesV5 are used to establish connectivity
	// with the rest of the network using the V5 discovery
	// protocol.
	BootstrapNodesV5 []*discv5.Node `toml:",omitempty"`

	// Static nodes are used as pre-configured connections which are always
	// maintained and re-connected on disconnects.
	StaticNodes []*enode.Node

	// Trusted nodes are used as pre-configured connections which are always
	// allowed to connect, even above the peer limit.
	TrustedNodes []*enode.Node

	// Connectivity can be restricted to certain IP networks.
	// If this option is set to a non-nil value, only hosts which match one of the
	// IP networks contained in the list are considered.
	NetRestrict *netutil.Netlist `toml:",omitempty"`

	// NodeDatabase is the path to the database containing the previously seen
	// live nodes in the network.
	NodeDatabase string `toml:",omitempty"`

	// Protocols should contain the protocols supported
	// by the server. Matching protocols are launched for
	// each peer.
	Protocols []Protocol `toml:"-"`

	// If ListenAddr is set to a non-nil address, the server
	// will listen for incoming connections.
	//
	// If the port is zero, the operating system will pick a port. The
	// ListenAddr field will be updated with the actual address when
	// the server is started.
	ListenAddr string

	// If set to a non-nil value, the given NAT port mapper
	// is used to make the listening port available to the
	// Internet.
	NAT nat.Interface `toml:",omitempty"`

	// If Dialer is set to a non-nil value, the given Dialer
	// is used to dial outbound peer connections.
	Dialer NodeDialer `toml:"-"`

	// If NoDial is true, the server will not dial any peers.
	NoDial bool `toml:",omitempty"`

	// If EnableMsgEvents is set then the server will emit PeerEvents
	// whenever a message is sent to or received from a peer
	EnableMsgEvents bool

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`
}

// Server manages all peer connections.
type Server struct {
	// Config fields may not be modified while the server is running.
	Config

	// Hooks for testing. These are useful because we can inhibit
	// the whole protocol stack.
	newTransport func(net.Conn) transport
	newPeerHook  func(*Peer)

	lock    sync.Mutex // protects running
	running bool       // indicate if the server is currently running

	nodedb       *enode.DB // node db created using the NodeDatabase defined in configuration
	localnode    *enode.LocalNode
	ntab         discoverTable   // discovery table contains neighbor nodes
	listener     net.Listener    // connection listener
	ourHandshake *protoHandshake // serve as a wrapper and wrapped the protocols supported by the server
	lastLookup   time.Time       // last time that server run the discovery task for looking up random nodes
	DiscV5       *discv5.Network

	// These are for Peers, PeerCount (and nothing else).
	peerOp     chan peerOpFunc
	peerOpDone chan struct{}

	// bunch of channels used for communication among different go routines
	quit                  chan struct{}
	addstatic             chan *enode.Node // used for adding static peers
	removestatic          chan *enode.Node // used for static peer disconnection
	addtrusted            chan *enode.Node // used to add a node to trusted peer
	removetrusted         chan *enode.Node // used to remove a node from trusted peer
	addStorageContract    chan *enode.Node // used to add storage contract peer
	removeStorageContract chan *enode.Node // used to remove storage contract peer
	addStorageClient      chan *enode.Node // used to add storage contract client peer
	removeStorageClient   chan *enode.Node // used to remove storage contract client peer
	storagePeerDoneMap    map[enode.ID]*sync.WaitGroup
	posthandshake         chan *conn
	addpeer               chan *conn
	delpeer               chan peerDrop

	loopWG   sync.WaitGroup // loop, listenLoop
	peerFeed event.Feed
	log      log.Logger
}

type peerOpFunc func(map[enode.ID]*Peer)

type peerDrop struct {
	*Peer
	err       error
	requested bool // true if signaled by the peer
}

type connFlag uint32

const (
	dynDialedConn connFlag = 1 << iota
	staticDialedConn
	inboundConn
	trustedConn
	storageContractConn
	storageClientConn
)

// conn wraps a network connection with information gathered
// during the two handshakes.
type conn struct {
	fd net.Conn
	transport
	node  *enode.Node // node that I am trying to establish the connection to
	flags connFlag
	cont  chan error // The run loop uses cont to signal errors to SetupConn.

	// caps contain protocol supported by the peer node
	caps []Cap  // valid after the protocol handshake
	name string // valid after the protocol handshake
}

type transport interface {
	// The two handshakes.
	doEncHandshake(prv *ecdsa.PrivateKey, dialDest *ecdsa.PublicKey) (*ecdsa.PublicKey, error)
	doProtoHandshake(our *protoHandshake) (*protoHandshake, error)
	// The MsgReadWriter can only be used after the encryption
	// handshake has completed. The code uses conn.id to track this
	// by setting it to a non-nil value after the encryption handshake.
	MsgReadWriter
	// transports must provide Close because we use MsgPipe in some of
	// the tests. Closing the actual network connection doesn't do
	// anything in those tests because MsgPipe doesn't use it.
	close(err error)
}

// returns connection information
// including connection flag, node id if not empty, and node's public address
func (c *conn) String() string {
	// stringified connection flag
	s := c.flags.String()
	// if node id is not empty, then add the node id to string
	if (c.node.ID() != enode.ID{}) {
		s += " " + c.node.ID().String()
	}
	// add the remote address to string
	// RemoteAddr == public address
	s += " " + c.fd.RemoteAddr().String()
	return s
}

// returns connection flag in string format
func (f connFlag) String() string {
	s := ""
	if f&trustedConn != 0 {
		s += "-trusted"
	}
	if f&dynDialedConn != 0 {
		s += "-dyndial"
	}
	if f&staticDialedConn != 0 {
		s += "-staticdial"
	}
	if f&inboundConn != 0 {
		s += "-inbound"
	}
	if f&storageContractConn != 0 {
		s += "-storageContractConn"
	}
	if f&storageClientConn != 0 {
		s += "-storageClientConn"
	}
	if s != "" {
		s = s[1:]
	}
	return s
}

// check if the connection Flags are equivalent
func (c *conn) is(f connFlag) bool {
	flags := connFlag(atomic.LoadUint32((*uint32)(&c.flags)))
	return flags&f != 0
}

// mark/unmark the connection flag
func (c *conn) set(f connFlag, val bool) {
	for {
		oldFlags := connFlag(atomic.LoadUint32((*uint32)(&c.flags)))
		flags := oldFlags
		if val {
			flags |= f
		} else {
			flags &= ^f
		}
		if atomic.CompareAndSwapUint32((*uint32)(&c.flags), uint32(oldFlags), uint32(flags)) {
			return
		}
	}
}

func (c *conn) GetNetConn() net.Conn {
	return c.fd
}

// Peers returns all connected peers.
// by passing the peerOp function to srv.peerOp channel
// what peerOp function did here is take a map of peers and
// put them into the list
func (srv *Server) Peers() []*Peer {
	var ps []*Peer
	select {
	// Note: We'd love to put this function into a variable but
	// that seems to cause a weird compiler error in some
	// environments.
	case srv.peerOp <- func(peers map[enode.ID]*Peer) {
		for _, p := range peers {
			ps = append(ps, p)
		}
	}:
		<-srv.peerOpDone
	case <-srv.quit:
	}
	return ps
}

// PeerCount returns the number of connected peers.
func (srv *Server) PeerCount() int {
	var count int
	select {
	case srv.peerOp <- func(ps map[enode.ID]*Peer) { count = len(ps) }:
		<-srv.peerOpDone
	case <-srv.quit:
	}
	return count
}

// AddPeer connects to the given node and maintains the connection until the
// server is shut down. If the connection fails for any reason, the server will
// attempt to reconnect the peer.
//
// Peer added is a static peer because the connection will be maintained
func (srv *Server) AddPeer(node *enode.Node) {
	select {
	case srv.addstatic <- node:
	case <-srv.quit:
	}
}

// RemovePeer disconnects from the given node
func (srv *Server) RemovePeer(node *enode.Node) {
	select {
	case srv.removestatic <- node:
	case <-srv.quit:
	}
}

// AddTrustedPeer adds the given node to a reserved whitelist which allows the
// node to always connect, even if the slot are full.
func (srv *Server) AddTrustedPeer(node *enode.Node) {
	select {
	case srv.addtrusted <- node:
	case <-srv.quit:
	}
}

// RemoveTrustedPeer removes the given node from the trusted peer set.
func (srv *Server) RemoveTrustedPeer(node *enode.Node) {
	select {
	case srv.removetrusted <- node:
	case <-srv.quit:
	}
}

// add storage contract and client flag
func (srv *Server) AddStorageContractPeer(node *enode.Node) {
	nodeID := node.ID()

	wg, ok := srv.storagePeerDoneMap[nodeID]
	if !ok {
		_wg := &sync.WaitGroup{}
		_wg.Add(2)
		srv.storagePeerDoneMap[nodeID] = _wg
		wg = _wg
	}

	select {
	case srv.addStorageContract <- node:
		srv.log.Warn("AddStorageContractPeer", "chanContent", node.String())
	case <-srv.quit:
	}

	select {
	case srv.addStorageClient <- node:
		srv.log.Warn("AddStorageClientPeer", "chanContent", node.String())
	case <-srv.quit:
	}

	// wait for add done
	wg.Wait()
	delete(srv.storagePeerDoneMap, nodeID)
}

// RemoveTrustedPeer removes the given node from the trusted peer set.
func (srv *Server) RemoveStorageContractPeer(node *enode.Node) {
	select {
	case srv.removeStorageContract <- node:
	case <-srv.quit:
	}

	select {
	case srv.removeStorageClient <- node:
	case <-srv.quit:
	}
}

// SubscribePeers subscribes the given channel to peer events
// Subscribed a peerEvent, this event will be emitted when
// peers added or dropped from the p2p.Server or message is sent or received
// on a peer connection. The message will be passed through the channel object
func (srv *Server) SubscribeEvents(ch chan *PeerEvent) event.Subscription {
	return srv.peerFeed.Subscribe(ch)
}

// Self returns the local node's endpoint information.
// localnode represent myself
func (srv *Server) Self() *enode.Node {
	srv.lock.Lock()
	ln := srv.localnode
	srv.lock.Unlock()

	if ln == nil {
		return enode.NewV4(&srv.PrivateKey.PublicKey, net.ParseIP("0.0.0.0"), 0, 0)
	}
	return ln.Node()
}

// Stop terminates the server and all active peer connections.
// It blocks until all active connections have been closed.
func (srv *Server) Stop() {
	srv.lock.Lock()
	if !srv.running {
		srv.lock.Unlock()
		return
	}
	srv.running = false
	if srv.listener != nil {
		// this unblocks listener Accept
		srv.listener.Close()
	}
	close(srv.quit)
	srv.lock.Unlock()
	srv.loopWG.Wait()
}

// sharedUDPConn implements a shared connection. Write sends messages to the underlying connection while read returns
// messages that were found unprocessable and sent to the unhandled channel by the primary listener.
type sharedUDPConn struct {
	*net.UDPConn
	unhandled chan discover.ReadPacket
}

// ReadFromUDP implements discv5.conn
func (s *sharedUDPConn) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	packet, ok := <-s.unhandled
	if !ok {
		return 0, nil, errors.New("Connection was closed")
	}
	l := len(packet.Data)
	if l > len(b) {
		l = len(b)
	}
	copy(b[:l], packet.Data[:l])
	return l, packet.Addr, nil
}

// Close implements discv5.conn
func (s *sharedUDPConn) Close() error {
	return nil
}

// Start starts running the server.
// Servers can not be re-used after stopping.
func (srv *Server) Start() (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()

	// checking if the server is currently running
	if srv.running {
		return errors.New("server already running")
	}
	srv.running = true

	// try to get the logger from the server configuration first
	srv.log = srv.Config.Logger
	if srv.log == nil {
		srv.log = log.New()
	}

	// useless server if it is not dialing nor listing
	// meaning not sending message nor accepting message
	if srv.NoDial && srv.ListenAddr == "" {
		srv.log.Warn("P2P server will be useless, neither dialing nor listening")
	}

	// static fields, server's private key must be set in the configuration
	if srv.PrivateKey == nil {
		return errors.New("Server.PrivateKey must be set to a non-nil key")
	}

	// if the transport function is not set
	// then use newRLPx transport protocol for secure communication
	// NOTE: THIS IS A FUNCTION, NOT FUNCTION CALL
	if srv.newTransport == nil {
		srv.newTransport = newRLPX
	}

	// if Dialer is not defined, then use the DCP dialer
	// with 15 seconds timeout
	if srv.Dialer == nil {
		srv.Dialer = TCPDialer{&net.Dialer{Timeout: defaultDialTimeout}}
	}

	// declaring unbuffered channels
	srv.quit = make(chan struct{})
	srv.addpeer = make(chan *conn)
	srv.delpeer = make(chan peerDrop)
	srv.posthandshake = make(chan *conn)
	srv.addstatic = make(chan *enode.Node)
	srv.removestatic = make(chan *enode.Node)
	srv.addtrusted = make(chan *enode.Node)
	srv.removetrusted = make(chan *enode.Node)
	srv.addStorageContract = make(chan *enode.Node)
	srv.removeStorageContract = make(chan *enode.Node)
	srv.addStorageClient = make(chan *enode.Node)
	srv.removeStorageClient = make(chan *enode.Node)
	srv.storagePeerDoneMap = make(map[enode.ID]*sync.WaitGroup)
	srv.peerOp = make(chan peerOpFunc)
	srv.peerOpDone = make(chan struct{})

	// setupLocalNode, setupListening, and setupDiscovery
	if err := srv.setupLocalNode(); err != nil {
		return err
	}

	// listen to the address, set up connection for any inbound connections
	// (connections requested from another node)
	if srv.ListenAddr != "" {
		if err := srv.setupListening(); err != nil {
			return err
		}
	}

	// setupDiscovery
	if err := srv.setupDiscovery(); err != nil {
		return err
	}

	// set the max outbound connection allowed
	dynPeers := srv.maxDialedConns()

	// this dialer schedules dials and discovery lookups.
	// where dynPeers specified max number of dials allowed
	dialer := newDialState(srv.localnode.ID(), srv.StaticNodes, srv.BootstrapNodes, srv.ntab, dynPeers, srv.NetRestrict)
	srv.loopWG.Add(1)
	go srv.run(dialer)
	return nil
}

// initialized a bunch of staffs
// 1. server's ourHandshake field, which stores all the protocols supported by the server (sorted)
// 2. create and assigned nodeDB to server's nodedb field
// 3. create localNode object, assign it to server's localNode field
// 4. set up fallbackIP, node record (supported protocols, protocol attributes, static IP if NAT configured)
func (srv *Server) setupLocalNode() error {
	// Create the devp2p handshake.

	// encrypt localNode's public key
	pubkey := crypto.FromECDSAPub(&srv.PrivateKey.PublicKey)

	// create protoHandshake object with the version, name, and ID
	srv.ourHandshake = &protoHandshake{Version: baseProtocolVersion, Name: srv.Name, ID: pubkey[1:]}

	// assign list of protocols supported to protoHandshake Caps
	// and sort them in order
	// based on debugging: the server will only contain one protocol, which is eth with latest version 63
	for _, p := range srv.Protocols {
		srv.ourHandshake.Caps = append(srv.ourHandshake.Caps, p.cap())
	}
	sort.Sort(capsByNameAndVersion(srv.ourHandshake.Caps))

	// Create the local node based on the path provided in the configuration
	db, err := enode.OpenDB(srv.Config.NodeDatabase)
	if err != nil {
		return err
	}
	srv.nodedb = db

	// create and initialize new localDB object with fallBackIP be 127.0.0.1
	// and store the supported protocols information into node record
	srv.localnode = enode.NewLocalNode(db, srv.PrivateKey)
	srv.localnode.SetFallbackIP(net.IP{127, 0, 0, 1})
	srv.localnode.Set(capsByNameAndVersion(srv.ourHandshake.Caps))

	// TODO: check conflicts
	// for each protocol's each attribute will be stored in the node record as well
	for _, p := range srv.Protocols {
		for _, e := range p.Attributes {
			srv.localnode.Set(e)
		}
	}

	// if NAT was used, set the static IP address
	// based on the type of NAT used
	// otherwise, do nothing (meaning static IP will not be set)
	switch srv.NAT.(type) {
	case nil:
		// No NAT interface, do nothing.
	case nat.ExtIP:
		// ExtIP doesn't block, set the IP right away.
		ip, _ := srv.NAT.ExternalIP()
		srv.localnode.SetStaticIP(ip)
	default:
		// Ask the router about the IP. This takes a while and blocks startup,
		// do it in the background.
		srv.loopWG.Add(1)
		go func() {
			defer srv.loopWG.Done()
			if ip, err := srv.NAT.ExternalIP(); err == nil {
				srv.localnode.SetStaticIP(ip)
			}
		}()
	}
	return nil
}

// establish UDP connection, get and set the discovery table
// set the discovery table to server ntab field
// FallBackUDP port was set for the localnode object
func (srv *Server) setupDiscovery() error {
	// if disallowed running discovery protocol (V4)
	// and discoveryV5 is not set, return
	if srv.NoDiscovery && !srv.DiscoveryV5 {
		return nil
	}

	// convert the listening address to UDP address format
	// default ListenAddr: :30303
	addr, err := net.ResolveUDPAddr("udp", srv.ListenAddr)
	if err != nil {
		return err
	}

	// listen to the address over udp network
	// even it returned UDPConn, but it serves as listener
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	// get the internal IP address in UDPAddr format
	realaddr := conn.LocalAddr().(*net.UDPAddr)
	srv.log.Debug("UDP listener up", "addr", realaddr)

	// map the address to nat if the IP is not loopback
	if srv.NAT != nil {
		if !realaddr.IP.IsLoopback() {
			go nat.Map(srv.NAT, srv.quit, "udp", realaddr.Port, realaddr.Port, "ethereum discovery")
		}
	}
	// set up localnode's fallback UDP port
	srv.localnode.SetFallbackUDP(realaddr.Port)

	// Discovery V4
	var unhandled chan discover.ReadPacket
	var sconn *sharedUDPConn
	if !srv.NoDiscovery {
		// IGNORE THIS
		if srv.DiscoveryV5 {
			unhandled = make(chan discover.ReadPacket, 100)
			sconn = &sharedUDPConn{conn, unhandled}
		}
		// START HERE
		cfg := discover.Config{
			PrivateKey:  srv.PrivateKey,
			NetRestrict: srv.NetRestrict,
			Bootnodes:   srv.BootstrapNodes,
			Unhandled:   unhandled,
		}
		ntab, err := discover.ListenUDP(conn, srv.localnode, cfg)
		if err != nil {
			return err
		}
		srv.ntab = ntab
	}

	// Discovery V5
	if srv.DiscoveryV5 {
		var ntab *discv5.Network
		var err error
		if sconn != nil {
			ntab, err = discv5.ListenUDP(srv.PrivateKey, sconn, "", srv.NetRestrict)
		} else {
			ntab, err = discv5.ListenUDP(srv.PrivateKey, conn, "", srv.NetRestrict)
		}
		if err != nil {
			return err
		}
		if err := ntab.SetFallbackNodes(srv.BootstrapNodesV5); err != nil {
			return err
		}
		srv.DiscV5 = ntab
	}
	return nil
}

// listen to the ListenAddr stored in the server configuration
func (srv *Server) setupListening() error {
	// Launch the TCP listener.
	listener, err := net.Listen("tcp", srv.ListenAddr)
	if err != nil {
		return err
	}

	// convert the listen address to TCPAddr format, and then update the ListenAddr in the configuration
	// localnode Node Record: set TCP port
	laddr := listener.Addr().(*net.TCPAddr)
	srv.ListenAddr = laddr.String()
	srv.listener = listener
	srv.localnode.Set(enr.TCP(laddr.Port))

	// add one too wait group
	srv.loopWG.Add(1)

	// handle inbound connection
	// there should have another go routine established connection to and inbound connection
	go srv.listenLoop()

	// Map the TCP listening port if NAT is configured
	// and the IP address is not loopback ip address (127.0.0.1)
	if !laddr.IP.IsLoopback() && srv.NAT != nil {
		srv.loopWG.Add(1)
		go func() {
			nat.Map(srv.NAT, srv.quit, "tcp", laddr.Port, laddr.Port, "ethereum p2p")
			srv.loopWG.Done()
		}()
	}
	return nil
}

type dialer interface {
	newTasks(running int, peers map[enode.ID]*Peer, now time.Time) []task
	taskDone(task, time.Time)
	addStatic(*enode.Node)
	removeStatic(*enode.Node)
}

func (srv *Server) run(dialstate dialer) {
	srv.log.Info("Started P2P networking", "self", srv.localnode.Node())
	defer srv.loopWG.Done()
	defer srv.nodedb.Close()

	var (
		peers           = make(map[enode.ID]*Peer)
		inboundCount    = 0 // number of inbound connections
		trusted         = make(map[enode.ID]bool, len(srv.TrustedNodes))
		storageContract = make(map[enode.ID]bool, 10)
		storageClient   = make(map[enode.ID]bool, 10)
		taskdone        = make(chan task, maxActiveDialTasks)
		runningTasks    []task
		queuedTasks     []task // tasks that can't run yet
	)

	// Put trusted nodes into a map to speed up checks.
	// Trusted peers are loaded on startup or added via AddTrustedPeer RPC.
	for _, n := range srv.TrustedNodes {
		trusted[n.ID()] = true
	}

	// removes task t from runningTasks
	delTask := func(t task) {
		for i := range runningTasks {
			if runningTasks[i] == t {
				runningTasks = append(runningTasks[:i], runningTasks[i+1:]...)
				break
			}
		}
	}

	// starts until max number of active tasks is satisfied
	// max active dial task will 16
	// ts contains a list of task that are not started
	startTasks := func(ts []task) (rest []task) {
		i := 0
		// start task (note: there are three different tasks)
		// add the task to the runningTasks list
		// return the rest of tasks
		for ; len(runningTasks) < maxActiveDialTasks && i < len(ts); i++ {
			t := ts[i]
			srv.log.Trace("New dial task", "task", t)
			go func() { t.Do(srv); taskdone <- t }()
			runningTasks = append(runningTasks, t)
		}
		return ts[i:]
	}

	// queued the tasks that does not has a chance to start
	// or newly created tasks from dialer if current amount of running
	// task is not reach to max yet
	scheduleTasks := func() {
		// put the tasks that doe not have a chance to run yet (16 max running task)
		// to queued task. NOTE: queuedTasks got emptied before placing remaining
		// tasks
		queuedTasks = append(queuedTasks[:0], startTasks(queuedTasks)...)

		// if runningTasks is not reach to the max number, asking dialer to create
		// new tasks (as many as possible, reach to the max active dial tasks)
		// append those tasks to qeuedtask as well
		if len(runningTasks) < maxActiveDialTasks {
			nt := dialstate.newTasks(len(runningTasks)+len(queuedTasks), peers, time.Now())
			queuedTasks = append(queuedTasks, startTasks(nt)...)
		}
	}

running:
	for {
		// when it first runs, it will ask dialer to creat a bunch of new tasks
		// stored in the queuedTasks list
		scheduleTasks()

		select {
		case <-srv.quit:
			// The server was stopped. Run the cleanup logic.
			break running

		// node information was passed in through AddPeer function
		// which will be added to the dialstate object static field
		case n := <-srv.addstatic:
			// This channel is used by AddPeer to add to the
			// ephemeral static peer list. Add it to the dialer,
			// it will keep the node connected.
			// add node to dialstate static field, which contains a list of static fields
			// waiting for the connection
			dialstate.addStatic(n)

		// node information was passed in through RemovePeer function
		// node will be removed from both dialstate static nodes list
		// and dialed history. Moreover, it will send peer disconnect
		// request to disconnect with that node
		case n := <-srv.removestatic:
			// This channel is used by RemovePeer to send a
			// disconnect request to a peer and begin the
			// stop keeping the node connected.
			dialstate.removeStatic(n)
			if p, ok := peers[n.ID()]; ok {
				p.Disconnect(DiscRequested)
			}

		// Add node to trusted node list
		// node information will be acquired from AddTrustedPeer function
		// check if the node already connected, if so, modify the peer's connection flag
		case n := <-srv.addtrusted:
			// This channel is used by AddTrustedPeer to add an enode
			// to the trusted node set.
			trusted[n.ID()] = true
			// Mark any already-connected peer as trusted
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(trustedConn, true)
			}

		// remove the node from trusted node list
		// if the node already connected, unmark it
		case n := <-srv.removetrusted:
			// This channel is used by RemoveTrustedPeer to remove an enode
			// from the trusted node set.
			if _, ok := trusted[n.ID()]; ok {
				delete(trusted, n.ID())
			}
			// Unmark any already-connected peer as trusted
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(trustedConn, false)
			}

		case n := <-srv.addStorageContract:
			storageContract[n.ID()] = true
			// Mark any already-connected peer as storageContractConn
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(storageContractConn, true)
			}
			srv.storagePeerDoneMap[n.ID()].Done()
		case n := <-srv.removeStorageContract:
			if _, ok := storageContract[n.ID()]; ok {
				delete(storageContract, n.ID())
			}
			// Unmark any already-connected peer as storageContractConn
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(storageContractConn, false)
			}

		case n := <-srv.addStorageClient:
			storageClient[n.ID()] = true
			// Mark any already-connected peer as storageClientConn
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(storageClientConn, true)
			}
			srv.storagePeerDoneMap[n.ID()].Done()

		case n := <-srv.removeStorageClient:
			if _, ok := storageContract[n.ID()]; ok {
				delete(storageContract, n.ID())
			}
			// Unmark any already-connected peer as storageClientConn
			if p, ok := peers[n.ID()]; ok {
				p.rw.set(storageClientConn, false)
			}

		// it will either get peer count or peers
		// run the function, and send the done signal
		// to notify the result is ready
		case op := <-srv.peerOp:
			// This channel is used by Peers and PeerCount.
			op(peers)
			srv.peerOpDone <- struct{}{}

		// signal will be received from the startTask function
		// where another go routine is used to do the task and
		// signal the taskdone channel
		// task will be deleted from runningtask list and handled by taskDone function
		case t := <-taskdone:
			// A task got done. Tell dialstate about it so it
			// can update its state and remove it from the active
			// tasks list.
			srv.log.Trace("Dial task done", "task", t)
			dialstate.taskDone(t, time.Now())
			delTask(t)

		// checking connection after encHandshake
		// if the node connected is in trusted node list, then,
		// make sure its' flag is set. Then do the encode handshake
		// check, mainly check if the connection is allowed to be connected
		// by checking if exceed max amount of connection, and etc.
		case c := <-srv.posthandshake:
			// A connection has passed the encryption handshake so
			// the remote identity is known (but hasn't been verified yet).
			if trusted[c.node.ID()] {
				// Ensure that the trusted flag is set before checking against MaxPeers.
				c.flags |= trustedConn
			}
			if storageContract[c.node.ID()] {
				c.flags |= storageContractConn
			}
			if storageClient[c.node.ID()] {
				c.flags |= storageClientConn
			}
			// TODO: track in-progress inbound node IDs (pre-Peer) to avoid dialing them.
			select {
			case c.cont <- srv.encHandshakeChecks(peers, inboundCount, c):
			case <-srv.quit:
				break running
			}

		// after the protoHandshake, check the connection, create peer, run peer with
		// another go routine, add to peer list, and increase the inboundCount if it is
		// inbound connection
		case c := <-srv.addpeer:
			// At this point the connection is past the protocol handshake.
			// Its capabilities are known and the remote identity is verified.
			err := srv.protoHandshakeChecks(peers, inboundCount, c)

			// create new peer, run peer, add it to peer list, and increase the inboundCount if
			// it is inbound connection
			if err == nil {
				// The handshakes are done and it passed all checks.
				p := newPeer(c, srv.Protocols)
				// If message events are enabled, pass the peerFeed
				// to the peer
				if srv.EnableMsgEvents {
					p.events = &srv.peerFeed
				}
				name := truncateName(c.name)
				srv.log.Debug("Adding p2p peer", "name", name, "addr", c.fd.RemoteAddr(), "peers", len(peers)+1)
				go srv.runPeer(p)
				peers[c.node.ID()] = p
				if p.Inbound() {
					inboundCount++
				}
			}
			// The dialer logic relies on the assumption that
			// dial tasks complete after the peer has been added or
			// discarded. Unblock the task last.
			select {
			// assumption is that before added peer, the dial task must complete
			// cont channel space is allocated in the setup connection function
			// and setup connection function is called while doing dial task
			case c.cont <- err:
			case <-srv.quit:
				break running
			}

		// This message will be received from runPeer function
		// which will only return if error or disconnect request was received
		case pd := <-srv.delpeer:
			// A peer disconnected.
			d := common.PrettyDuration(mclock.Now() - pd.created)
			pd.log.Debug("Removing p2p peer", "duration", d, "peers", len(peers)-1, "req", pd.requested, "err", pd.err)
			delete(peers, pd.ID())
			if pd.Inbound() {
				inboundCount--
			}
		}
	}

	srv.log.Trace("P2P networking is spinning down")

	// Terminate discovery. If there is a running lookup it will terminate soon.
	if srv.ntab != nil {
		srv.ntab.Close()
	}
	if srv.DiscV5 != nil {
		srv.DiscV5.Close()
	}
	// Disconnect all peers.
	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}
	// Wait for peers to shut down. Pending connections and tasks are
	// not handled here and will terminate soon-ish because srv.quit
	// is closed.
	for len(peers) > 0 {
		p := <-srv.delpeer
		p.log.Trace("<-delpeer (spindown)", "remainingTasks", len(runningTasks))
		delete(peers, p.ID())
	}
}

// if none of server's protocol match the connection protocol, then return error
// otherwise, repeat the encHandshake check again
func (srv *Server) protoHandshakeChecks(peers map[enode.ID]*Peer, inboundCount int, c *conn) error {
	// Drop connections with no matching protocols.
	if len(srv.Protocols) > 0 && countMatchingProtocols(srv.Protocols, c.caps) == 0 {
		return DiscUselessPeer
	}
	// Repeat the encryption handshake checks because the
	// peer set might have changed between the handshakes.
	return srv.encHandshakeChecks(peers, inboundCount, c)
}

// Checking Criteria:
// 1. if the connection flag is neither trusted nor static and reached max peers allowed to connection
// 2. if the connection flag is not trusted inbound connection, and reached to max inbound connection allowed
// 3. if the node is already connected
// 4. if the node id is local node id
func (srv *Server) encHandshakeChecks(peers map[enode.ID]*Peer, inboundCount int, c *conn) error {
	switch {
	case !c.is(trustedConn|staticDialedConn|storageContractConn) && len(peers) >= srv.MaxPeers:
		return DiscTooManyPeers
	case !c.is(trustedConn) && !c.is(storageContractConn) && c.is(inboundConn) && inboundCount >= srv.maxInboundConns():
		return DiscTooManyPeers
	case peers[c.node.ID()] != nil:
		return DiscAlreadyConnected
	case c.node.ID() == srv.localnode.ID():
		return DiscSelf
	default:
		return nil
	}
}

// max inbound connection == max peers allowed to connect - max outbound connection allowed
func (srv *Server) maxInboundConns() int {
	return srv.MaxPeers - srv.maxDialedConns()
}

// max outbound connection == max peers allowed to connect / dial ratio
func (srv *Server) maxDialedConns() int {
	if srv.NoDiscovery || srv.NoDial {
		return 0
	}
	r := srv.DialRatio
	if r == 0 {
		r = defaultDialRatio
	}
	return srv.MaxPeers / r
}

// listenLoop runs in its own goroutine and accepts
// inbound connections.
//
// 1. make handshake slots and fill in with empty data
// 2. get the inbound connection from the listener, check if the IP address is in the whitelist
// 3. convert the connection to meteredConn, and call SetupConn function, at the end,
// fill in the slot
func (srv *Server) listenLoop() {
	defer srv.loopWG.Done()
	srv.log.Debug("TCP listener up", "addr", srv.listener.Addr())

	// if maxPendingPeers was not configured in the server configuration
	// then it will use the default value which is 50
	tokens := defaultMaxPendingPeers
	if srv.MaxPendingPeers > 0 {
		tokens = srv.MaxPendingPeers
	}
	slots := make(chan struct{}, tokens)
	for i := 0; i < tokens; i++ {
		slots <- struct{}{}
	}

	for {
		// Wait for a handshake slot before accepting.
		<-slots

		var (
			fd  net.Conn
			err error
		)
		for {
			// get the inbound connection from listener
			// if a temporary error occurred, try get the
			// inbound connection from the listener again
			fd, err = srv.listener.Accept()
			if netutil.IsTemporaryError(err) {
				srv.log.Debug("Temporary read error", "err", err)
				continue
			} else if err != nil {
				srv.log.Debug("Read error", "err", err)
				return
			}
			break
		}

		// Reject connections that do not match NetRestrict.
		// If the connection got rejected, send fill the handshake slot by one again
		if srv.NetRestrict != nil {
			if tcp, ok := fd.RemoteAddr().(*net.TCPAddr); ok && !srv.NetRestrict.Contains(tcp.IP) {
				srv.log.Debug("Rejected conn (not whitelisted in NetRestrict)", "addr", fd.RemoteAddr())
				fd.Close()
				slots <- struct{}{}
				continue
			}
		}

		var ip net.IP

		// get the public address of the node trying to make connection to localnode
		// and convert the connection to meteredConn object
		if tcp, ok := fd.RemoteAddr().(*net.TCPAddr); ok {
			ip = tcp.IP
		}
		fd = newMeteredConn(fd, true, ip)
		srv.log.Trace("Accepted connection", "addr", fd.RemoteAddr())

		// once the connection got accepted, then run another go routine to establish the
		// inbound connection with other nodes
		go func() {
			srv.SetupConn(fd, inboundConn, nil)
			// after the finished setting up the connection
			// feed in the handshake slot
			slots <- struct{}{}
		}()
	}
}

// SetupConn runs the handshakes and attempts to add the connection
// as a peer. It returns when the connection has been added as a peer
// or the handshakes have failed.
//
// This function was used in two places:
// 1. Dial (outbound connection)
// 2. listenloop (inbound connection)
func (srv *Server) SetupConn(fd net.Conn, flags connFlag, dialDest *enode.Node) error {
	// set up server connection
	// if not specified, newTransport uses newRLPX function
	// NOTE: fd are meteredConn
	c := &conn{fd: fd, transport: srv.newTransport(fd), flags: flags, cont: make(chan error)}
	err := srv.setupConn(c, flags, dialDest)
	if err != nil {
		c.close(err)
		srv.log.Trace("Setting up connection failed", "addr", fd.RemoteAddr(), "err", err)
	}
	return err
}

// set up the connection (modification to the conn object)
func (srv *Server) setupConn(c *conn, flags connFlag, dialDest *enode.Node) error {
	// Prevent leftover pending conns from entering the handshake.
	srv.lock.Lock()
	running := srv.running
	srv.lock.Unlock()

	// check if the server is currently running
	if !running {
		return errServerStopped
	}

	// If dialing, figure out the remote public key.
	// get the node's public key from the node record entries
	var dialPubkey *ecdsa.PublicKey
	if dialDest != nil {
		dialPubkey = new(ecdsa.PublicKey)
		if err := dialDest.Load((*enode.Secp256k1)(dialPubkey)); err != nil {
			return errors.New("dial destination doesn't have a secp256k1 public key")
		}
	}

	// Run the encryption handshake, get the remote public key (destination node's public key)
	// the rlpxFrameRw is also initialized
	remotePubkey, err := c.doEncHandshake(srv.PrivateKey, dialPubkey)
	if err != nil {
		srv.log.Trace("Failed RLPx handshake", "addr", c.fd.RemoteAddr(), "conn", c.flags, "err", err)
		return err
	}

	// Check if the destination node's public key is same as one returned
	if dialDest != nil {
		// For dialed connections, check that the remote public key matches.
		// Update the node in the connection with the node I am trying to connect to (destnode)
		if dialPubkey.X.Cmp(remotePubkey.X) != 0 || dialPubkey.Y.Cmp(remotePubkey.Y) != 0 {
			return DiscUnexpectedIdentity
		}
		c.node = dialDest
	} else {
		// For inbound connection, create node object
		// based on the connection and the public key (derived from doEncHandshake)
		c.node = nodeFromConn(remotePubkey, c.fd)
	}

	// if the fd is metered connection, then call handshakeDone
	if conn, ok := c.fd.(*meteredConn); ok {
		conn.handshakeDone(c.node.ID())
	}

	clog := srv.log.New("id", c.node.ID(), "addr", c.fd.RemoteAddr(), "conn", c.flags)
	// send the connection to the posthandshake channel to check
	// if the connection is performed correctly after the handshake
	// changed field: node, fd (fd changed through function handshakeDone)
	// if error occurred, return the error directly before protocol handshake
	err = srv.checkpoint(c, srv.posthandshake)
	if err != nil {
		clog.Trace("Rejected peer before protocol handshake", "err", err)
		return err
	}

	// Run the protocol handshake with passed in ourHandshake object (protoHandshake)
	// a list of protocols supported by the destination node will be returned
	srv.ourHandshake.Flags = c.flags

	phs, err := c.doProtoHandshake(srv.ourHandshake)

	if err != nil {
		clog.Trace("Failed proto handshake", "err", err)
		return err
	}

	// check the node ID against the ID contained in the protoHandshake ID
	// both should be derived from the public key
	if id := c.node.ID(); !bytes.Equal(crypto.Keccak256(phs.ID), id[:]) {
		clog.Trace("Wrong devp2p handshake identity", "phsid", hex.EncodeToString(phs.ID))
		return DiscUnexpectedIdentity
	}

	// destination node supported protocol
	if phs.Flags&storageContractConn != 0 {
		c.set(storageContractConn, true)
	}
	c.caps, c.name = phs.Caps, phs.Name

	// check the connection again
	err = srv.checkpoint(c, srv.addpeer)
	if err != nil {
		clog.Trace("Rejected peer", "err", err)
		return err
	}

	// If the checks completed successfully, runPeer has now been
	// launched by run.
	clog.Trace("connection set up", "inbound", dialDest == nil)
	return nil
}

// Create node object based on the public key and the connection
// NOTE: the public key is derived from the EncHandshake
func nodeFromConn(pubkey *ecdsa.PublicKey, conn net.Conn) *enode.Node {
	var ip net.IP
	var port int
	// getting the IP address and port from the connection
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		ip = tcp.IP
		port = tcp.Port
	}
	// create the node object based on the public key
	// ip address and port number
	return enode.NewV4(pubkey, ip, port, port)
}

// if the name is longer than 20 characters
// replace them with ...
func truncateName(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}

// checkpoint sends the conn to run, which performs the
// post-handshake checks for the stage (posthandshake, addpeer).
func (srv *Server) checkpoint(c *conn, stage chan<- *conn) error {
	select {
	case stage <- c:
	case <-srv.quit:
		return errServerStopped
	}
	select {
	case err := <-c.cont:
		return err
	case <-srv.quit:
		return errServerStopped
	}
}

// runPeer runs in its own goroutine for each peer.
// it waits until the Peer logic returns and removes
// the peer.
func (srv *Server) runPeer(p *Peer) {
	// used for testing purpose
	if srv.newPeerHook != nil {
		srv.newPeerHook(p)
	}

	// broadcast peer add
	srv.peerFeed.Send(&PeerEvent{
		Type: PeerEventTypeAdd,
		Peer: p.ID(),
	})

	// run the peer
	// it will block until the peer disconnected
	// if remoteRequested == true, it means the peer disconnection is requested
	remoteRequested, err := p.run()

	// broadcast peer drop once peer disconnected
	srv.peerFeed.Send(&PeerEvent{
		Type:  PeerEventTypeDrop,
		Peer:  p.ID(),
		Error: err.Error(),
	})

	// Note: run waits for existing peers to be sent on srv.delpeer
	// before returning, so this send should not select on srv.quit.
	srv.delpeer <- peerDrop{p, err, remoteRequested}
}

// NodeInfo represents a short summary of the information known about the host.
type NodeInfo struct {
	ID    string `json:"id"`    // Unique node identifier (also the encryption key)
	Name  string `json:"name"`  // Name of the node, including client type, version, OS, custom data
	Enode string `json:"enode"` // Enode URL for adding this peer from remote peers
	ENR   string `json:"enr"`   // Ethereum Node Record
	IP    string `json:"ip"`    // IP address of the node
	Ports struct {
		Discovery int `json:"discovery"` // UDP listening port for discovery protocol
		Listener  int `json:"listener"`  // TCP listening port for RLPx
	} `json:"ports"`
	ListenAddr string                 `json:"listenAddr"`
	Protocols  map[string]interface{} `json:"protocols"`
}

// NodeInfo gathers and returns a collection of metadata known about the host.
func (srv *Server) NodeInfo() *NodeInfo {
	// Gather and assemble the generic node infos
	node := srv.Self()
	info := &NodeInfo{
		Name:       srv.Name,
		Enode:      node.String(),
		ID:         node.ID().String(),
		IP:         node.IP().String(),
		ListenAddr: srv.ListenAddr,
		Protocols:  make(map[string]interface{}),
	}
	info.Ports.Discovery = node.UDP()
	info.Ports.Listener = node.TCP()
	if enc, err := rlp.EncodeToBytes(node.Record()); err == nil {
		info.ENR = "0x" + hex.EncodeToString(enc)
	}

	// Gather all the running protocol infos (only once per protocol type)
	for _, proto := range srv.Protocols {
		if _, ok := info.Protocols[proto.Name]; !ok {
			nodeInfo := interface{}("unknown")
			if query := proto.NodeInfo; query != nil {
				nodeInfo = proto.NodeInfo()
			}
			info.Protocols[proto.Name] = nodeInfo
		}
	}
	return info
}

// PeersInfo returns an array of metadata objects describing connected peers.
func (srv *Server) PeersInfo() []*PeerInfo {
	// Gather all the generic and sub-protocol specific infos
	infos := make([]*PeerInfo, 0, srv.PeerCount())
	for _, peer := range srv.Peers() {
		if peer != nil {
			infos = append(infos, peer.Info())
		}
	}
	// Sort the result array alphabetically by node identifier
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[i].ID > infos[j].ID {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}
	return infos
}
