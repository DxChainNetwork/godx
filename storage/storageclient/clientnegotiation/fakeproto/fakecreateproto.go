// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package fakeproto

import (
	"fmt"
	"os"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

const fakeContractCreateProtoDir = "testdata/fakecontractcreateproto"

type FakeContractCreateProto struct {
	steps              map[uint64]struct{}
	sendMsg            map[uint64]struct{}
	contractSet        *contractset.StorageContractSet
	failedInteraction  map[enode.ID]uint64
	successInteraction map[enode.ID]uint64
}

func NewFakeContractCreateProto() (*FakeContractCreateProto, error) {
	clearOldData()

	proto := FakeContractCreateProto{
		steps:              make(map[uint64]struct{}),
		sendMsg:            make(map[uint64]struct{}),
		failedInteraction:  make(map[enode.ID]uint64),
		successInteraction: make(map[enode.ID]uint64),
	}

	var err error
	proto.contractSet, err = contractset.New(fakeContractCreateProtoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage contract set: %s", err.Error())
	}

	return &proto, nil
}

func (fc *FakeContractCreateProto) SetSteps(steps map[uint64]struct{}) {
	fc.steps = steps
}

func (fc *FakeContractCreateProto) SetSendMsg(msgCodes []uint64) {
	for _, msgCode := range msgCodes {
		fc.sendMsg[msgCode] = struct{}{}
	}
}

func (fc *FakeContractCreateProto) GetFailedInteractionCount(id enode.ID) uint64 {
	return fc.failedInteraction[id]
}

func (fc *FakeContractCreateProto) GetSuccessInteractionCount(id enode.ID) uint64 {
	return fc.successInteraction[id]
}

func (fc *FakeContractCreateProto) RetrieveContract(contractID storage.ContractID) (contract storage.ContractMetaData, exists bool) {
	return fc.contractSet.RetrieveContractMetaData(contractID)
}

// clearOldData will clear the previously saved old data
func clearOldData() {
	_ = os.RemoveAll(fakeContractCreateProtoDir)
}

// ===== ===== ===== BACKEND METHODS ===== ===== ====

func (fc *FakeContractCreateProto) FindWallet(account accounts.Account) (accounts.Wallet, error) {
	am := &FakeAccountManager{}
	return am.Find(account)
}

func (fc *FakeContractCreateProto) SetupConnection(enodeURL string) (storage.Peer, error) {
	// simulate the situation on failed to set up the storage connection
	if _, exist := fc.steps[FakeSetUpConnectionFailed]; exist {
		return nil, fmt.Errorf("storage client failed to setup the storage connection")
	}

	// create a fake storage peer used for simulating the storage connection
	fs := NewFakeStoragePeer()
	fs.SetSendMsg(fc.sendMsg)
	fs.SetTestSteps(fs.steps)
	return fs, nil
}

func (fc *FakeContractCreateProto) GetStorageContractSet() (contractSet *contractset.StorageContractSet) {
	return fc.contractSet
}

func (fc *FakeContractCreateProto) InsertContract(ch contractset.ContractHeader, roots []common.Hash) (storage.ContractMetaData, error) {
	return fc.contractSet.InsertContract(ch, roots)
}

func (fc *FakeContractCreateProto) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (fc *FakeContractCreateProto) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (fc *FakeContractCreateProto) IncrementSuccessfulInteractions(id enode.ID, interactionType storagehostmanager.InteractionType) {
	fc.successInteraction[id] += 1
}

func (fc *FakeContractCreateProto) IncrementFailedInteractions(id enode.ID, interactionType storagehostmanager.InteractionType) {
	fc.failedInteraction[id] += 1
}
