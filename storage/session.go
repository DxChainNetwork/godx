// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/p2p"
)

const (
	// Storage Contract Negotiate Protocol belonging to eth/64
	// Storage Contract Creation/Renew Code Msg
	StorageContractCreationMsg                   = 0x11
	StorageContractCreationHostSignMsg           = 0x12
	StorageContractCreationClientRevisionSignMsg = 0x13
	StorageContractCreationHostRevisionSignMsg   = 0x14

	// Upload Data Segment Code Msg
	StorageContractUploadRequestMsg         = 0x15
	StorageContractUploadMerkleRootProofMsg = 0x16
	StorageContractUploadClientRevisionMsg  = 0x17
	StorageContractUploadHostRevisionMsg    = 0x18

	// Download Data Segment Code Msg
	StorageContractDownloadRequestMsg      = 0x19
	StorageContractDownloadDataMsg         = 0x20
	StorageContractDownloadHostRevisionMsg = 0x21

	// error msg code
	NegotiationErrorMsg = 0x22

	// stop msg code
	NegotiationStopMsg = 0x23
)

type SessionSet struct {
	sessions map[string]*Session
	lock     sync.RWMutex
	closed   bool
}

func NewSessionSet() *SessionSet {
	return &SessionSet{
		sessions: make(map[string]*Session),
	}
}

func (st *SessionSet) Register(s *Session) error {
	st.lock.Lock()
	defer st.lock.Unlock()

	if st.closed {
		return errors.New("session is closed")
	}

	if _, ok := st.sessions[s.id]; ok {
		return errors.New("session is already registered")
	}
	st.sessions[s.id] = s

	return nil
}

func (st *SessionSet) Unregister(id string) error {
	st.lock.Lock()
	defer st.lock.Unlock()

	_, ok := st.sessions[id]
	if !ok {
		return errors.New("session is not registered")
	}
	delete(st.sessions, id)

	return nil
}

func (st *SessionSet) Close() {
	st.lock.Lock()
	defer st.lock.Unlock()

	for _, p := range st.sessions {
		p.Disconnect(p2p.DiscQuitting)
	}
	st.closed = true
}

// Peer retrieves the registered peer with the given id.
func (st *SessionSet) Session(id string) *Session {
	st.lock.RLock()
	defer st.lock.RUnlock()

	return st.sessions[id]
}

type Session struct {
	id string

	*p2p.Peer
	rw p2p.MsgReadWriter

	version int

	host       HostInfo
	clientDisc chan error
}

func NewSession(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Session {
	return &Session{
		id:         fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		Peer:       p,
		rw:         rw,
		version:    version,
		clientDisc: make(chan error),
	}
}
func (s *Session) ClientDiscChan() chan error {
	return s.clientDisc
}

func (s *Session) HostInfo() *HostInfo {
	return &s.host
}

func (s *Session) getConn() net.Conn {
	return s.Peer.GetConn()
}

// set the read and write deadline in connection
func (s *Session) SetDeadLine(d time.Duration) error {
	conn := s.getConn()
	err := conn.SetDeadline(time.Now().Add(d))
	if err != nil {
		return err
	}
	return nil
}

// form contract protocol

// RW() and SetRW() for only test
func (s *Session) RW() p2p.MsgReadWriter {
	return s.rw
}

func (s *Session) SetRW(rw p2p.MsgReadWriter) {
	s.rw = rw
}

func (s *Session) SendStorageContractCreation(data interface{}) error {
	s.Log().Debug("Sending storage contract creation tx to host from client", "tx", data)
	return p2p.Send(s.rw, StorageContractCreationMsg, data)
}

func (s *Session) SendStorageContractCreationHostSign(data interface{}) error {
	s.Log().Debug("Sending storage contract create host signatures for storage client", "signature", data)
	return p2p.Send(s.rw, StorageContractCreationHostSignMsg, data)
}

func (s *Session) SendStorageContractCreationClientRevisionSign(data interface{}) error {
	s.Log().Debug("Sending storage contract update to storage host by storage client", "data", data)
	return p2p.Send(s.rw, StorageContractCreationClientRevisionSignMsg, data)
}

func (s *Session) SendStorageContractCreationHostRevisionSign(data interface{}) error {
	s.Log().Debug("Sending storage contract update host signatures", "signature", data)
	return p2p.Send(s.rw, StorageContractCreationHostRevisionSignMsg, data)
}

// upload protocol

func (s *Session) SendStorageContractUploadRequest(data interface{}) error {
	s.Log().Debug("Sending storage contract upload request", "request", data)
	return p2p.Send(s.rw, StorageContractUploadRequestMsg, data)
}

func (s *Session) SendStorageContractUploadMerkleProof(data interface{}) error {
	s.Log().Debug("Sending storage contract upload proof", "proof", data)
	return p2p.Send(s.rw, StorageContractUploadMerkleRootProofMsg, data)
}

func (s *Session) SendStorageContractUploadClientRevisionSign(data interface{}) error {
	s.Log().Debug("Sending storage contract upload client revision sign", "sign", data)
	return p2p.Send(s.rw, StorageContractUploadClientRevisionMsg, data)
}

func (s *Session) SendStorageContractUploadHostRevisionSign(data interface{}) error {
	s.Log().Debug("Sending storage host revision sign", "sign", data)
	return p2p.Send(s.rw, StorageContractUploadHostRevisionMsg, data)
}

// download protocol

func (s *Session) SendStorageContractDownloadRequest(data interface{}) error {
	s.Log().Debug("Sending storage contract download request", "request", data)
	return p2p.Send(s.rw, StorageContractDownloadRequestMsg, data)
}

func (s *Session) SendStorageContractDownloadData(data interface{}) error {
	s.Log().Debug("Sending storage contract download data", "data", data)
	return p2p.Send(s.rw, StorageContractDownloadDataMsg, data)
}

func (s *Session) ReadMsg() (*p2p.Msg, error) {
	msg, err := s.rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	return &msg, err
}

// if error occurs in host's negotiation, should send this msg
func (s *Session) SendErrorMsg(err error) error {
	s.Log().Debug("Sending negotiation error msg", "error_info", err)
	return p2p.Send(s.rw, NegotiationErrorMsg, err)
}

// send this msg to notify the other node that we want stop the negotiation
func (s *Session) SendStopMsg() error {
	s.Log().Debug("Sending negotiation stop msg")
	return p2p.Send(s.rw, NegotiationStopMsg, nil)
}