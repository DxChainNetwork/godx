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

	"github.com/DxChainNetwork/godx/p2p"
)

var (
	ErrClientDisconnect = errors.New("storage client disconnect proactively")
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
	StorageContractDownloadHostRevisionMsg = 0x32

	// error msg code
	NegotiationErrorMsg = 0x33

	// stop msg code
	NegotiationStopMsg = 0x34
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
}

func NewSession(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Session {
	return &Session{
		id:           fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		Peer:         p,
		rw:           rw,
		version:      version,
		clientDisc:   make(chan error),
		revisionDone: make(chan struct{}, 1),
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
	return p2p.Send(s.rw, HostSettingMsg, data)
}

func (s *Session) SendHostExtSettingsResponse(data interface{}) error {
	s.Log().Debug("Sending host settings response from host", "msg", data)
	return p2p.Send(s.rw, HostSettingResponseMsg, data)
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
	return p2p.Send(s.rw, StorageContractDownloadRequestMsg, data)
}

func (s *Session) SendStorageContractDownloadData(data interface{}) error {
	return p2p.Send(s.rw, StorageContractDownloadDataMsg, data)
}

func (s *Session) ReadMsg() (*p2p.Msg, error) {
	msg, err := s.rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	return &msg, err
}

// send this msg to notify the other node that we want stop the negotiation
func (s *Session) SendStopMsg() error {
	s.Log().Debug("Sending negotiation stop msg")
	return p2p.Send(s.rw, NegotiationStopMsg, "revision stop")
}

func (s *Session) IsClosed() bool {
	return s.Peer.IsClosed()
}
