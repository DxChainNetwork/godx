// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"fmt"
	"time"

	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// FakeStoragePeer contains the testType that will be used to simulate the
// the storage connection
type FakeStoragePeer struct {
	// uint64 represents the step key, based on the step key
	// choose to perform the positive or negative test case
	steps map[uint64]struct{}

	// map key represents p2p.Msg.Code
	sendMsg map[uint64]struct{}

	// bool represents if it should be failed or succeed
	msg chan p2p.Msg
}

// NewFakeStoragePeer creates and initializes a new fake storage peer
// used for the test case
func NewFakeStoragePeer() *FakeStoragePeer {
	return &FakeStoragePeer{
		steps:   make(map[uint64]struct{}),
		sendMsg: make(map[uint64]struct{}),
		msg:     make(chan p2p.Msg),
	}
}

// SetSendMsg is used to configure the sendMsg field
func (fs *FakeStoragePeer) SetSendMsg(messages map[uint64]struct{}) {
	fs.sendMsg = messages
}

// SetTestType is used to configure the testTypePositive
func (fs *FakeStoragePeer) SetTestSteps(steps map[uint64]struct{}) {
	fs.steps = steps
}

// ===== ==== ===== ===== WAIT RESPONSES ===== ==== ===== =====

func (fs *FakeStoragePeer) ClientWaitContractResp() (p2p.Msg, error) {
	var message p2p.Msg

	// get the message
	select {
	case message = <-fs.msg:
	case <-time.After(1 * time.Second):
		return p2p.Msg{}, fmt.Errorf("timeout, failed to get the message from the storage host")
	}

	return message, nil
}

func (fs *FakeStoragePeer) FakeSendMsg(msg p2p.Msg) error {
	select {
	case fs.msg <- msg:
		return nil
	default:
		return fmt.Errorf("failed to send message to storage host: host is too busy")
	}
}

func (fs *FakeStoragePeer) HostWaitContractResp() (p2p.Msg, error) {
	// TODO (mzhang): not the final version
	return p2p.Msg{}, nil
}

func (fs *FakeStoragePeer) WaitConfigResp() (p2p.Msg, error) {
	// TODO (mzhang): not the final version
	return p2p.Msg{}, nil
}

// ===== ==== ===== ===== CLIENT CONTRACT CREATE RELATED METHOD ===== ==== ===== =====

// RequestContractCreation is used to simulate the process of the client send message to host
// the difference is that the host will not process the message sent, it will directly return
// to the storage client. It means that the message sent from this function is actually the response
// of the storage host
func (fs *FakeStoragePeer) RequestContractCreation(req storage.ContractCreateRequest) error {
	// if FakeContractCreateRequestSendFailed exist, meaning the method
	// should return an error
	if _, exist := fs.sendMsg[FakeContractCreateRequestSendFailed]; exist {
		return fmt.Errorf("error sending the request contract creation")
	}

	// if FakeContractCreateRequestFailed exists, meaning the host should send the host negotiation
	// error message to client
	if _, exist := fs.sendMsg[FakeContractCreateRequestFailed]; exist {
		msg, _ := NewFakeResponseMsg(storage.HostNegotiateErrorMsg, []byte(""))
		return fs.FakeSendMsg(msg)
	}

	// if FakeContractCreateHostTooBusy exists, meaning the host should send the host too
	// busy message to storage client
	if _, exist := fs.sendMsg[FakeContractCreateHostTooBusy]; exist {
		msg, _ := NewFakeResponseMsg(storage.HostBusyHandleReqMsg, []byte(""))
		return fs.FakeSendMsg(msg)
	}

	// otherwise, the message with correct message code should be sent
	msg, _ := NewFakeResponseMsg(storage.ContractCreateHostSign, []byte("HOST SIGNED STORAGE CONTRACT"))
	return fs.FakeSendMsg(msg)

}

// SendContractCreateClientRevisionSign simulates the client send contract create client revision sign
// process
func (fs *FakeStoragePeer) SendContractCreateClientRevisionSign(revisionSign []byte) error {
	// simulate the client failed to send the contract create revision request
	if _, exist := fs.sendMsg[FakeContractCreateRevisionSendFailed]; exist {
		return fmt.Errorf("error sending the contract create revision")
	}

	// simulate the contract negotiation error
	if _, exist := fs.sendMsg[FakeContractCreateRevisionFailed]; exist {
		msg, _ := NewFakeResponseMsg(storage.HostNegotiateErrorMsg, []byte(""))
		return fs.FakeSendMsg(msg)
	}

	// simulate the host is too busy to handle the request
	if _, exist := fs.sendMsg[FakeContractCreateRevisionHostTooBusy]; exist {
		msg, _ := NewFakeResponseMsg(storage.HostBusyHandleReqMsg, []byte(""))
		return fs.FakeSendMsg(msg)
	}

	// otherwise, simulate the negotiation success situation
	msg, _ := NewFakeResponseMsg(storage.ContractCreateRevisionSign, []byte("HOST SIGNED STORAGE CONTRACT REVISION"))
	return fs.FakeSendMsg(msg)
}

func (fs *FakeStoragePeer) SendClientCommitSuccessMsg() error {
	// simulate once the client successfully sent the commit success message
	// the host failed to send back the acknowledgement message
	if _, exist := fs.sendMsg[FakeClientCommitSuccessFailed]; exist {
		msg, _ := NewFakeResponseMsg(storage.HostNegotiateErrorMsg, []byte(""))
		return fs.FakeSendMsg(msg)
	}

	// simulate client received the host acknowledgement message
	msg, _ := NewFakeResponseMsg(storage.HostAckMsg, []byte(""))
	return fs.FakeSendMsg(msg)
}

func (fs *FakeStoragePeer) SendClientNegotiateErrorMsg() error {
	// in case of client contract create negotiation, it does not matter
	// if the client successfully sent the client negotiation error message
	return nil
}

func (fs *FakeStoragePeer) SendClientCommitFailedMsg() error {
	// in case of client contract create negotiation, it does not matter
	// if the client successfully send the client commit failed message
	return nil
}

func (fs *FakeStoragePeer) SendClientAckMsg() error {
	// in case of client contract create negotiation, it does not matter
	// if the client successfully send the client acknowledge message
	return nil
}

func (fs *FakeStoragePeer) PeerNode() *enode.Node {
	// since the return variable is used by CheckAndUpdateConnection method
	// defined in the fakecmbackend, therefore, there is no need to return
	// meaningful value
	return nil
}

// ===== ==== ===== ===== HOST CONTRACT CREATE RELATED METHOD ==== ==== ==== ====

func (fs *FakeStoragePeer) SendContractCreationHostSign(contractSign []byte) error {
	return nil
}

func (fs *FakeStoragePeer) SendContractCreationHostRevisionSign(revisionSign []byte) error {
	return nil
}

// ===== ==== ===== ===== CLIENT UPLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) RequestContractUpload(req storage.UploadRequest) error {
	return nil
}

func (fs *FakeStoragePeer) SendContractUploadClientRevisionSign(revisionSign []byte) error {
	return nil
}

// ===== ==== ===== ===== HOST UPLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) SendUploadMerkleProof(merkleProof storage.UploadMerkleProof) error {
	return nil
}

func (fs *FakeStoragePeer) SendUploadHostRevisionSign(revisionSign []byte) error {
	return nil
}

// ===== ==== ===== ===== CLIENT DOWNLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) RequestContractDownload(req storage.DownloadRequest) error {
	return nil
}

// ===== ==== ===== ===== HOST DOWNLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) SendContractDownloadData(resp storage.DownloadResponse) error {
	return nil
}

// ===== ==== ===== ===== OTHER HOSTS METHODS ===== ==== ===== =====

func (fs *FakeStoragePeer) SendHostCommitFailedMsg() error {
	return nil
}

func (fs *FakeStoragePeer) SendHostAckMsg() error {
	return nil
}

func (fs *FakeStoragePeer) SendHostNegotiateErrorMsg() error {
	return nil
}

// ===== ==== ===== ===== OTHER METHODS ===== ==== ===== =====

func (fs *FakeStoragePeer) TriggerError(error) {}

func (fs *FakeStoragePeer) SendStorageHostConfig(config storage.HostExtConfig) error {
	return nil
}

func (fs *FakeStoragePeer) RequestStorageHostConfig() error {
	return nil
}

func (fs *FakeStoragePeer) SendHostBusyHandleRequestErr() error {
	return nil
}

func (fs *FakeStoragePeer) TryToRenewOrRevise() bool {
	return false
}

func (fs *FakeStoragePeer) RevisionOrRenewingDone() {}

func (fs *FakeStoragePeer) TryRequestHostConfig() error {
	return nil
}

func (fs *FakeStoragePeer) RequestHostConfigDone() {}

func (fs *FakeStoragePeer) IsStaticConn() bool {
	return true
}
