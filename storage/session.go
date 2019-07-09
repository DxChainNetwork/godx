// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
)

var (
	ErrClientDisconnect = errors.New("storage client disconnect proactively")
	ErrNegotiateTerminate = errors.New("client or host terminate in negotiate process")
	ErrSendNegotiateStartTimeout = errors.New("receive client negotiate start msg timeout")
)

const (
	IDLE = 0
	BUSY = 1

	HostSettingMsg         = 0x20
	HostSettingResponseMsg = 0x21

	// Storage Contract Negotiate Protocol belonging to eth/64
	// Storage Contract Creation/Renew Code Msg
	StorageContractCreationMsg                   = 0x22
	StorageContractCreationHostSignMsg           = 0x23
	StorageContractCreationClientRevisionSignMsg = 0x24
	StorageContractCreationHostRevisionSignMsg   = 0x25

	// Upload Data Segment Code Msg
	StorageContractUploadRequestMsg         = 0x26
	StorageContractUploadMerkleRootProofMsg = 0x27
	StorageContractUploadClientRevisionMsg  = 0x28
	StorageContractUploadHostRevisionMsg    = 0x29

	// Download Data Segment Code Msg
	StorageContractDownloadRequestMsg      = 0x30
	StorageContractDownloadDataMsg         = 0x31

	// error msg code
	NegotiationErrorMsg = 0x32

	// stop msg code
	NegotiationTerminateMsg = 0x33

	// Storage Protocol Host Flag
	StorageHostStartMsg   = 0x34
	StorageClientStartMsg = 0x35
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

	host       *HostInfo
	clientDisc chan error

	// indicate this session is busy, it is true when uploading or downloading
	busy int32

	// upload and download tash is done, signal the renew goroutine
	revisionDone chan struct{}

	// client negotiate start chan
	clientNegotiateStart chan struct{}

	// storage negotiate protocol done
	negotiateDone chan struct{}
}

func NewSession(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Session {
	return &Session{
		id:                   fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		Peer:                 p,
		rw:                   rw,
		version:              version,
		clientDisc:           make(chan error),
		revisionDone:         make(chan struct{}, 1),
		negotiateDone:        make(chan struct{}, 1),
		clientNegotiateStart: make(chan struct{}, 1),
	}
}

func (s *Session) StopConnection() bool {
	select {
	case s.clientDisc <- ErrClientDisconnect:
	case <-s.ClosedChan():
	default:
		return false
	}
	return true
}

func (s *Session) ClientNegotiateStartChan() chan struct{} {
	return s.clientNegotiateStart
}

func (s *Session) ClientNegotiateDoneChan() chan struct{} {
	return s.negotiateDone
}

func (s *Session) ClientDiscChan() chan error {
	return s.clientDisc
}

func (s *Session) SetHostInfo(hi *HostInfo) {
	s.host = hi
}

func (s *Session) HostInfo() *HostInfo {
	return s.host
}

func (s *Session) SetBusy() bool {
	return atomic.CompareAndSwapInt32(&s.busy, IDLE, BUSY)
}

func (s *Session) ResetBusy() bool {
	return atomic.CompareAndSwapInt32(&s.busy, BUSY, IDLE)
}

func (s *Session) IsBusy() bool {
	return atomic.LoadInt32(&s.busy) == BUSY
}

// when we renew but upload or download now, we wait for the revision done
func (s *Session) RevisionDone() chan struct{} {
	return s.revisionDone
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

// RW() and SetRW() for only test
func (s *Session) RW() p2p.MsgReadWriter {
	return s.rw
}

func (s *Session) SetRW(rw p2p.MsgReadWriter) {
	s.rw = rw
}

func (s *Session) SendHostExtSettingsRequest(data interface{}) error {
	if err := s.SendStorageNegotiateHostStartMsg(struct{}{}); err != nil {
		return err
	}

	select {
	case <-s.ClientNegotiateStartChan():
		//log.Error("Client receive host start chan")
	case <-time.After(1 * time.Second):
		return errors.New("receive client negotiate start msg timeout")
	}

	return p2p.Send(s.rw, HostSettingMsg, data)
}

func (s *Session) SendHostExtSettingsResponse(data interface{}) error {
	s.Log().Debug("Sending host settings response from host", "msg", data)
	return p2p.Send(s.rw, HostSettingResponseMsg, data)
}

func (s *Session) SendStorageContractCreation(data interface{}) error {
	s.Log().Debug("Sending storage contract creation tx to host from client", "tx", data)

	if err := s.SendStorageNegotiateHostStartMsg(struct{}{}); err != nil {
		return err
	}

	select {
	case <-s.ClientNegotiateStartChan():
	case <-time.After(1 * time.Second):
		return errors.New("receive client negotiate start msg timeout")
	}

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
	if err := s.SendStorageNegotiateHostStartMsg(struct{}{}); err != nil {
		return err
	}

	select {
	case <-s.ClientNegotiateStartChan():
	case <-time.After(1 * time.Second):
		return ErrSendNegotiateStartTimeout
	}

	return p2p.Send(s.rw, StorageContractUploadRequestMsg, data)
}

func (s *Session) SendStorageContractUploadMerkleProof(data interface{}) error {
	return p2p.Send(s.rw, StorageContractUploadMerkleRootProofMsg, data)
}

func (s *Session) SendStorageContractUploadClientRevisionSign(data interface{}) error {
	return p2p.Send(s.rw, StorageContractUploadClientRevisionMsg, data)
}

func (s *Session) SendStorageContractUploadHostRevisionSign(data interface{}) error {
	return p2p.Send(s.rw, StorageContractUploadHostRevisionMsg, data)
}

// download protocol
func (s *Session) SendStorageContractDownloadRequest(data interface{}) error {
	if err := s.SendStorageNegotiateHostStartMsg(struct{}{}); err != nil {
		log.Error("SendStorageNegotiateHostStartMsg Failed", "err", err)
		return err
	}

	select {
	case <-s.ClientNegotiateStartChan():
		log.Error("receive download start chan")
	case <-time.After(1 * time.Second):
		log.Error("receive download start chan TIMEOUT")
		return ErrSendNegotiateStartTimeout
	}

	return p2p.Send(s.rw, StorageContractDownloadRequestMsg, data)
}

func (s *Session) SendStorageContractDownloadData(data interface{}) error {
	return p2p.Send(s.rw, StorageContractDownloadDataMsg, data)
}

func (s *Session) SendStorageNegotiateHostStartMsg(data interface{}) error {
	return p2p.Send(s.rw, StorageHostStartMsg, data)
}

func (s *Session) SendStorageNegotiateClientStartMsg(data interface{}) error {
	return p2p.Send(s.rw, StorageClientStartMsg, data)
}

func (s *Session) ReadMsg() (*p2p.Msg, error) {
	msg, err := s.rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	return &msg, err
}

// send this msg to notify the other node that we want stop the negotiation
func (s *Session) SendNegotiateTerminateMsg(data interface{}) {
	if err := p2p.Send(s.rw, NegotiationTerminateMsg, data); err != nil {
		log.Error("send NegotiationTerminateMsg failed", "err", err)
	}
}

func (s *Session) IsClosed() bool {
	return s.Peer.IsClosed()
}

func (s *Session) CheckNegotiateTerminateMsg(msg *p2p.Msg) bool {
	if msg != nil && msg.Code == NegotiationTerminateMsg {
		return true
	}
	return false
}