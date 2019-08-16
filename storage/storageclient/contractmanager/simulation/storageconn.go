// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// FakeStoragePeer contains the testType that will be used to simulate the
// the storage connection
type FakeStoragePeer struct {
	TestType string
	MsgType  uint64
}

func (fs *FakeStoragePeer) ClientWaitContractResp() (p2p.Msg, error) {
	if fs.TestType == "negative" {
		return p2p.Msg{}, fmt.Errorf("client failed to get response from the storage host")
	}

	// otherwise, try with positive test case, fake the host response
	msg, err := NewFakeResponseMsg(fs.MsgType, []byte("HOST SIGNATURE"))
	if err != nil {
		return p2p.Msg{}, fmt.Errorf("failed to fake the host response message: %s", err.Error())
	}

	return msg, nil
}

func (fs *FakeStoragePeer) HostWaitContractResp() (p2p.Msg, error) {
	if fs.TestType == "negative" {
		return p2p.Msg{}, fmt.Errorf("host failed to get response from the storage client")
	}

	// otherwise, try with the positive test case, fake the client response
	msg, err := NewFakeResponseMsg(fs.MsgType, []byte("CLIENT SIGNATURE"))
	if err != nil {
		return p2p.Msg{}, fmt.Errorf("failed to fake the client response message: %s", err.Error())
	}

	return msg, nil
}

func (fs *FakeStoragePeer) WaitConfigResp() (p2p.Msg, error) {
	if fs.TestType == "negative" {
		return p2p.Msg{}, fmt.Errorf("failed to wait for the config requese from the storage host")
	}

	// otherwise, try with the positive test case, fake the host response
	msg, err := NewFakeResponseMsg(fs.MsgType, storage.HostInfo{IP: "0191934894"})
	if err != nil {
		return p2p.Msg{}, fmt.Errorf("failed to fake the host config response message: %s", err.Error())
	}

	return msg, nil
}

// ===== ==== ===== ===== CLIENT CONTRACT CREATE RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) RequestContractCreation(req storage.ContractCreateRequest) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to request contract creation")
}

func (fs *FakeStoragePeer) SendContractCreateClientRevisionSign(revisionSign []byte) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("contract create client revision sign cannot be sign")
}

func (fs *FakeStoragePeer) SendClientNegotiateErrorMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send client negotiate error message")
}

func (fs *FakeStoragePeer) SendClientCommitFailedMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send client commit failed message")
}

func (fs *FakeStoragePeer) SendClientAckMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send client ack message")
}

func (fs *FakeStoragePeer) PeerNode() *enode.Node {
	// since the return variable is used by CheckAndUpdateConnection method
	// defined in the fakecmbackend, therefore, there is no need to return
	// meaningful value
	return nil
}

func (fs *FakeStoragePeer) SendClientCommitSuccessMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send client commit success messsage")
}

// ===== ==== ===== ===== HOST CONTRACT CREATE RELATED METHOD ==== ==== ==== ====

func (fs *FakeStoragePeer) SendContractCreationHostSign(contractSign []byte) error {
	if fs.TestType == "positive" {
		return nil
	}
	return fmt.Errorf("failed to send contract creation host sign")
}

func (fs *FakeStoragePeer) SendContractCreationHostRevisionSign(revisionSign []byte) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send contract creation host revision sign")
}

// ===== ==== ===== ===== CLIENT UPLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) RequestContractUpload(req storage.UploadRequest) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("faield to request contract upload opeartion")
}

func (fs *FakeStoragePeer) SendContractUploadClientRevisionSign(revisionSign []byte) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send contract upload client revision sign")
}

// ===== ==== ===== ===== HOST UPLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) SendUploadMerkleProof(merkleProof storage.UploadMerkleProof) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send upload merkle proof")
}

func (fs *FakeStoragePeer) SendUploadHostRevisionSign(revisionSign []byte) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send upload host revision sign")
}

// ===== ==== ===== ===== CLIENT DOWNLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) RequestContractDownload(req storage.DownloadRequest) error {
	if fs.TestType == "positive" {
		return nil
	}
	return fmt.Errorf("failed to request contract dowload operation")
}

// ===== ==== ===== ===== HOST DOWNLOAD RELATED METHOD ===== ==== ===== =====

func (fs *FakeStoragePeer) SendContractDownloadData(resp storage.DownloadResponse) error {
	if fs.TestType == "positive" {
		return nil
	}
	return fmt.Errorf("failed to send contract download data")
}

// ===== ==== ===== ===== OTHER HOSTS METHODS ===== ==== ===== =====

func (fs *FakeStoragePeer) SendHostCommitFailedMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send storage host commit failed message")
}

func (fs *FakeStoragePeer) SendHostAckMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send storage host ack message")
}

func (fs *FakeStoragePeer) SendHostNegotiateErrorMsg() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send storage host negotiate error message")
}

// ===== ==== ===== ===== OTHER METHODS ===== ==== ===== =====

func (fs *FakeStoragePeer) TriggerError(error) {}

func (fs *FakeStoragePeer) SendStorageHostConfig(config storage.HostExtConfig) error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to send storage host configuration")
}

func (fs *FakeStoragePeer) RequestStorageHostConfig() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("failed to request storage host config")
}

func (fs *FakeStoragePeer) SendHostBusyHandleRequestErr() error {
	if fs.TestType == "positive" {
		return nil
	}
	return fmt.Errorf("failed to send host busy handle request error message")
}

func (fs *FakeStoragePeer) TryToRenewOrRevise() bool {
	if fs.TestType == "positive" {
		return false
	}

	return true
}

func (fs *FakeStoragePeer) RevisionOrRenewingDone() {}

func (fs *FakeStoragePeer) TryRequestHostConfig() error {
	if fs.TestType == "positive" {
		return nil
	}

	return fmt.Errorf("faield to try to request storage host configuration")
}

func (fs *FakeStoragePeer) RequestHostConfigDone() {}
func (fs *FakeStoragePeer) IsStaticConn() bool {
	if fs.TestType == "positive" {
		return true
	}

	return false
}
