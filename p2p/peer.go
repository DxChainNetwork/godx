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

package p2p

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common/mclock"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/p2p/enr"
	"github.com/DxChainNetwork/godx/rlp"
)

var (
	ErrShuttingDown = errors.New("shutting down")
)

const (
	baseProtocolVersion    = 5
	baseProtocolLength     = uint64(16)
	baseProtocolMaxMsgSize = 2 * 1024

	snappyProtocolVersion = 5

	pingInterval = 15 * time.Second
)

const (
	// devp2p message codes
	handshakeMsg = 0x00
	discMsg      = 0x01
	pingMsg      = 0x02
	pongMsg      = 0x03
)

const (
	// FullNodeProtocol is the protocol supported by full synced nodes
	FullNodeProtocol = "dx"

	// LightNodeProtocol is the protocol supported by light synced nodes
	LightNodeProtocol = "lightdx"
)

// protoHandshake is the RLP structure of the protocol handshake.
type protoHandshake struct {
	Version    uint64 // 5 by default
	Name       string // node name of the server
	Caps       []Cap  // list of protocols supported
	ListenPort uint64
	ID         []byte // secp256k1 public key
	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// PeerEventType is the type of peer events emitted by a p2p.Server
type PeerEventType string

const (
	// PeerEventTypeAdd is the type of event emitted when a peer is added
	// to a p2p.Server
	PeerEventTypeAdd PeerEventType = "add"

	// PeerEventTypeDrop is the type of event emitted when a peer is
	// dropped from a p2p.Server
	PeerEventTypeDrop PeerEventType = "drop"

	// PeerEventTypeMsgSend is the type of event emitted when a
	// message is successfully sent to a peer
	PeerEventTypeMsgSend PeerEventType = "msgsend"

	// PeerEventTypeMsgRecv is the type of event emitted when a
	// message is received from a peer
	PeerEventTypeMsgRecv PeerEventType = "msgrecv"
)

// PeerEvent is an event, which will be emitted when:
// 1. peers added or dropped from a p2p.Server
// 2. message is sent or received on a peer connection
type PeerEvent struct {
	Type     PeerEventType `json:"type"`
	Peer     enode.ID      `json:"peer"`
	Error    string        `json:"error,omitempty"`
	Protocol string        `json:"protocol,omitempty"`
	MsgCode  *uint64       `json:"msg_code,omitempty"`
	MsgSize  *uint32       `json:"msg_size,omitempty"`
}

// Peer represents a connected remote node.
type Peer struct {
	rw      *conn               // p2p server connection type
	running map[string]*protoRW // contains matched protocols between connection and peer node
	log     log.Logger
	created mclock.AbsTime // when the peer object got created / connected

	wg       sync.WaitGroup
	protoErr chan error
	closed   chan struct{}   // used to indicate if the peer is closed already
	disc     chan DiscReason // channel used to indicate the disconnection from the peer
	stopChan chan struct{}

	// events receives message send / receive events if set
	events *event.Feed
}

// NewPeer returns a peer for testing purposes.
func NewPeer(id enode.ID, name string, caps []Cap) *Peer {
	// create a pipe connection and a null Node object
	pipe, _ := net.Pipe()
	node := enode.SignNull(new(enr.Record), id)
	conn := &conn{fd: pipe, transport: nil, node: node, caps: caps, name: name}
	peer := newPeer(conn, nil)
	close(peer.closed) // ensures Disconnect doesn't block
	return peer
}

// StopChan returns the stop channel which used to indicate
// that the program is exiting and the peer should be stopped
func (p *Peer) StopChan() chan struct{} {
	return p.stopChan
}

// Stop indicates that peer should be stopped
func (p *Peer) Stop() {
	close(p.stopChan)
}

// ID returns the peer node's public key. (Node ID)
func (p *Peer) ID() enode.ID {
	return p.rw.node.ID()
}

// Node returns the peer's node descriptor.
// including node ID and node record
func (p *Peer) Node() *enode.Node {
	return p.rw.node
}

// Name returns the peer node name that the remote node advertised.
func (p *Peer) Name() string {
	return p.rw.name
}

// Caps returns the capabilities (supported subprotocols) of the remote peer.
func (p *Peer) Caps() []Cap {
	return p.rw.caps
}

// RemoteAddr returns the peer's remote address of the network connection.
// returns network address which include the name of the network (ex: tcp)
// and the address of the network (ex: 127.0.0.1:8000)
// PUBLIC IP ADDRESS of a peer node
func (p *Peer) RemoteAddr() net.Addr {
	return p.rw.fd.RemoteAddr()
}

// LocalAddr returns the local address of the network connection.
// INTERNAL IP ADDRESS of a peer node
func (p *Peer) LocalAddr() net.Addr {
	return p.rw.fd.LocalAddr()
}

// Disconnect terminates the peer connection with the given reason.
// It returns immediately and does not wait until the connection is closed.
// disconnect with a peer
func (p *Peer) Disconnect(reason DiscReason) {
	select {
	case p.disc <- reason:
	case <-p.closed:
	}
}

// String implements fmt.Stringer.
// returns peer's first 7 byte ID and public IP address
func (p *Peer) String() string {
	id := p.ID()
	return fmt.Sprintf("Peer %x %v", id[:8], p.RemoteAddr())
}

// Inbound returns true if the peer is an inbound connection
// Inbound Connection: you are running a server, you are accepting connections initiated by someone else
// if peer is inbound connection, meaning that I am trying to connect to the peer
func (p *Peer) Inbound() bool {
	return p.rw.is(inboundConn)
}

// create and initialize new peer object
// protocols: a list of protocols supported by the peer
func newPeer(conn *conn, protocols []Protocol) *Peer {
	protomap := matchProtocols(protocols, conn.caps, conn)
	p := &Peer{
		rw:       conn,
		running:  protomap,
		created:  mclock.Now(),
		disc:     make(chan DiscReason),
		protoErr: make(chan error, len(protomap)+1), // protocols + pingLoop
		closed:   make(chan struct{}),
		stopChan: make(chan struct{}),
		log:      log.New("id", conn.node.ID(), "conn", conn.flags), // contains node id and the connFlag
	}
	return p
}

// returns the peer's log
func (p *Peer) Log() log.Logger {
	return p.log
}

// for only test
func (p *Peer) Set(f connFlag, val bool) {
	p.rw.set(f, val)
}

// run the protocol
// it will wait until an error occurred and two go routines finished execution
func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		writeErr   = make(chan error, 1)
		readErr    = make(chan error, 1)
		reason     DiscReason // sent to the peer
	)
	p.wg.Add(2) // add 2 go routine to wait group

	// read and handle message from the connection
	go p.readLoop(readErr)

	// ping destination node every 15 seconds
	go p.pingLoop()

	// Start all protocol handlers.
	writeStart <- struct{}{}
	p.startProtocols(writeStart, writeErr)

	// Wait for an error or disconnect.
loop:
	for {
		select {
		case err = <-writeErr:
			// A write finished. Allow the next write to start if
			// there was no error.
			if err != nil {
				reason = DiscNetworkError
				break loop
			}
			writeStart <- struct{}{}
		case err = <-readErr:
			if r, ok := err.(DiscReason); ok {
				remoteRequested = true
				reason = r
			} else {
				reason = DiscNetworkError
			}
			break loop
		case err = <-p.protoErr:
			reason = discReasonForError(err)
			break loop
		case err = <-p.disc:
			reason = discReasonForError(err)
			break loop
		}
	}

	close(p.closed)
	p.rw.close(reason)
	p.wg.Wait() // wait until readLoop and pingLoop existed
	return remoteRequested, err
}

// Send ping message to destination node every 15 seconds
func (p *Peer) pingLoop() {
	// timer with 15 seconds interval
	ping := time.NewTimer(pingInterval)
	defer p.wg.Done()
	defer ping.Stop()
	for {
		select {
		case <-ping.C:
			if err := SendItems(p.rw, pingMsg); err != nil {
				p.protoErr <- err
				return
			}
			ping.Reset(pingInterval)
		case <-p.closed:
			return
		}
	}
}

// keep reading message from the connection
// unless error occurred during message reading or message handling
// message will be read from the connection
func (p *Peer) readLoop(errc chan<- error) {
	defer p.wg.Done()
	for {
		// read message from the connection
		// NOTE: the ReadMsg is implemented by rlpx ReadMsg method
		// used encrypted connection defined in rlpxrw
		msg, err := p.rw.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		// set the time received the message
		msg.ReceivedAt = time.Now()

		// handle the message based on message code
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
	}
}

// message handling
// 1. pingMsg: send pong message back
// 2. discMsg: return the disconnection reason
// 3. baseProtocolLength: ignored
// 4. default: sub-protocol message, the the protocolRW, and send the message to the in channel
func (p *Peer) handle(msg Msg) error {
	switch {
	// if the message is a pingMsg, then delete message
	// and send peers pong message
	case msg.Code == pingMsg:
		msg.Discard()
		go SendItems(p.rw, pongMsg)

	// if the message is disconnection request message
	// decode the message and return the first reason
	case msg.Code == discMsg:
		var reason [1]DiscReason
		// This is the last message. We don't need to discard or
		// check errors because, the connection will be closed after it.
		rlp.Decode(msg.Payload, &reason)
		return reason[0]

	// if the message is regarding base protocol length, discard message,
	case msg.Code < baseProtocolLength:
		// ignore other base protocol messages
		return msg.Discard()

	// sub-protocol message, get the protocol based on the message code
	// put the message to the protocol in channel
	default:
		proto, err := p.getProto(msg.Code)
		if err != nil {
			return fmt.Errorf("msg code out of range: %v", msg.Code)
		}
		select {
		case proto.in <- msg:
			return nil
		case <-p.closed:
			return io.EOF
		}
	}
	return nil
}

// returns the number of matched protocols between server and peers
func countMatchingProtocols(protocols []Protocol, caps []Cap) int {
	n := 0
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
				n++
			}
		}
	}
	return n
}

// matchProtocols creates structures for matching named subprotocols.
// from left to right: protocols supported by peer, protocols supported by the connection,
// connection itself
//
// for each matched protocol, create the key using the protocol name with value protoRW
// included the protocol itself, offset, channel, and rw connection
func matchProtocols(protocols []Protocol, caps []Cap, rw MsgReadWriter) map[string]*protoRW {
	// convert caps to capsByNameAndVersion, and then sort it
	// from small to big. Protocol name, if name are same, then version
	sort.Sort(capsByNameAndVersion(caps))
	offset := baseProtocolLength // 16
	result := make(map[string]*protoRW)

outer:
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
				// If an old protocol version matched, revert it
				if old := result[cap.Name]; old != nil {
					offset -= old.Length
				}
				// Assign the new match
				result[cap.Name] = &protoRW{Protocol: proto, offset: offset, in: make(chan Msg), w: rw}
				offset += proto.Length

				continue outer
			}
		}
	}
	return result
}

// protocolExclusion checks if the node supports both fullNode
// and lightNode protocols. If so, the lightNodeProtocol must
// be excluded
func (p *Peer) protocolExclusion() bool {
	if len(p.running) <= 1 {
		return false
	}

	// otherwise, check the existence of each protocol
	var ethProto, lesProto bool
	for _, proto := range p.running {
		if proto.Name == FullNodeProtocol {
			ethProto = true
		} else if proto.Name == LightNodeProtocol {
			lesProto = true
		}
	}

	// check and return
	return ethProto && lesProto
}

func (p *Peer) startProtocols(writeStart <-chan struct{}, writeErr chan<- error) {
	p.wg.Add(len(p.running))

	exclusion := p.protocolExclusion()

	// loop through all matched protocol
	for _, proto := range p.running {
		// if exclusion is needed and the protocol is lightNodeProtocol
		// then exclude and do not run the lightNodeProtocol
		if exclusion && proto.Name == LightNodeProtocol {
			log.Error("light node protocol excluded with peer", "id", p.ID())
			continue
		}

		// initialize protoRW
		proto := proto
		proto.closed = p.closed
		proto.wstart = writeStart
		proto.werr = writeErr

		// rw now inherited ReadMsg and WriteMsg methods from the protoRW
		var rw MsgReadWriter = proto

		// if p.events is defined, the change MsgReadWriter
		if p.events != nil {
			// change the MsgReaderWriter to the one used for sending message event
			// which not only read message, but also broadcasting message
			// other than broadcasting, exactly same operation as protoRW
			rw = newMsgEventer(rw, p.events, p.ID(), proto.Name)
		}

		p.log.Trace(fmt.Sprintf("Starting protocol %s/%d", proto.Name, proto.Version))

		// go routine runs the protocol, calls the run function defined in the protoRW
		// peer and rw was passed in. rw is used to call ReadMsg method
		go func() {
			err := proto.Run(p, rw)
			if err == nil {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d returned", proto.Name, proto.Version))
				err = errProtocolReturned
			} else if err != io.EOF {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d failed", proto.Name, proto.Version), "err", err)
			}
			p.protoErr <- err
			p.wg.Done()
		}()
	}
}

// getProto finds the protocol responsible for handling
// the given message code.
func (p *Peer) getProto(code uint64) (*protoRW, error) {
	for _, proto := range p.running {
		if code >= proto.offset && code < proto.offset+proto.Length {
			return proto, nil
		}
	}
	return nil, newPeerError(errInvalidMsgCode, "%d", code)
}

// protocol wrapper
type protoRW struct {
	Protocol
	in     chan Msg        // receives read messages
	closed <-chan struct{} // receives when peer is shutting down
	wstart <-chan struct{} // receives when write may start
	werr   chan<- error    // for write results
	offset uint64
	w      MsgWriter
}

func (rw *protoRW) WriteMsg(msg Msg) (err error) {
	if msg.Code >= rw.Length {
		log.Error("code size large than rw length", "code", msg.Code)
		return newPeerError(errInvalidMsgCode, "not handled")
	}
	msg.Code += rw.offset
	select {
	case <-rw.wstart:
		err = rw.w.WriteMsg(msg)
		// Report write status back to Peer.run. It will initiate
		// shutdown if the error is non-nil and unblock the next write
		// otherwise. The calling protocol code should exit for errors
		// as well but we don't want to rely on that.
		rw.werr <- err
	case <-rw.closed:
		err = ErrShuttingDown
	}
	return err
}

func (rw *protoRW) ReadMsg() (Msg, error) {
	select {
	case msg := <-rw.in:
		msg.Code -= rw.offset
		return msg, nil
	case <-rw.closed:
		return Msg{}, io.EOF
	}
}

// PeerInfo represents a short summary of the information known about a connected
// peer. Sub-protocol independent fields are contained and initialized here, with
// protocol specifics delegated to all connected sub-protocols.
type PeerInfo struct {
	Enode   string   `json:"enode"` // Node URL
	ID      string   `json:"id"`    // Unique node identifier
	Name    string   `json:"name"`  // Name of the node, including client type, version, OS, custom data
	Caps    []string `json:"caps"`  // Protocols advertised by this peer
	Network struct {
		LocalAddress  string `json:"localAddress"`  // Local endpoint of the TCP data connection
		RemoteAddress string `json:"remoteAddress"` // Remote endpoint of the TCP data connection
		Inbound       bool   `json:"inbound"`
		Trusted       bool   `json:"trusted"`
		Static        bool   `json:"static"`
	} `json:"network"`
	Protocols map[string]interface{} `json:"protocols"` // Sub-protocol specific metadata fields
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *Peer) Info() *PeerInfo {
	// Gather the protocol capabilities
	var caps []string
	for _, cap := range p.Caps() {
		caps = append(caps, cap.String())
	}
	// Assemble the generic peer metadata
	info := &PeerInfo{
		Enode:     p.Node().String(),
		ID:        p.ID().String(),
		Name:      p.Name(),
		Caps:      caps,
		Protocols: make(map[string]interface{}),
	}
	info.Network.LocalAddress = p.LocalAddr().String()
	info.Network.RemoteAddress = p.RemoteAddr().String()
	info.Network.Inbound = p.rw.is(inboundConn)
	info.Network.Trusted = p.rw.is(trustedConn)
	info.Network.Static = p.rw.is(staticDialedConn)

	// Gather all the running protocol infos
	for _, proto := range p.running {
		protoInfo := interface{}("unknown")
		if query := proto.Protocol.PeerInfo; query != nil {
			if metadata := query(p.ID()); metadata != nil {
				protoInfo = metadata
			} else {
				protoInfo = "handshake"
			}
		}
		info.Protocols[proto.Name] = protoInfo
	}
	return info
}
