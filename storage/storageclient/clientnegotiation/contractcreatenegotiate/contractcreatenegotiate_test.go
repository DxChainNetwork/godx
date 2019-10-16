// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiate

import (
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/clientnegotiation/fakeproto"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

func TestContractManager_ContractCreateNegotiate(t *testing.T) {
	params := simulation.ContractParamsGenerator()
	cp, err := fakeproto.NewFakeContractCreateProto()
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}

	meta, err := Handler(cp, params)
	if err != nil {
		t.Fatalf("contract create negotiation failed: %s", err.Error())
	}

	// check the meta data
	rmeta, exist := cp.RetrieveContract(meta.ID)
	if !exist {
		t.Fatalf("based on the contract ID returned from the contract create negotiate function, failed to get the contract information from database")
	}

	if !reflect.DeepEqual(rmeta, meta) {
		t.Fatalf("the contract received is not equivlent to the contract saved in db")
	}

	// check the interactions
	if count := cp.GetSuccessInteractionCount(params.Host.EnodeID); count != 1 {
		t.Errorf("expected success interaction to be increased by one")
	}
}

func TestContractManager_HostFailedToCommit(t *testing.T) {
	// set up storage connection for roll back operation
	negotiationMsg := []uint64{fakeproto.FakeClientCommitSuccessFailed}

	// create a fake contract manager
	cp, err := fakeproto.NewFakeContractCreateProto()
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}
	cp.SetSendMsg(negotiationMsg)

	// create contract parameter and insert host information
	params := simulation.ContractParamsGenerator()

	// call the ContractCreteNegotiate
	meta, err := Handler(cp, params)
	if err != storage.ErrHostCommit {
		t.Fatalf("error: expected error to be %s, got %s", storage.ErrHostCommit.Error(), err.Error())
	}

	// check to see if the contract is saved in the db
	if _, exist := cp.RetrieveContract(meta.ID); exist {
		t.Fatalf("rollback failed: expected the contract is not saved in the database persistently")
	}

	if count := cp.GetFailedInteractionCount(params.Host.EnodeID); count != 1 {
		t.Fatalf("expected failed interaction to be increased by one")
	}

	if count := cp.GetSuccessInteractionCount(params.Host.EnodeID); count != 0 {
		t.Errorf("expected success interaction to be remain the same, which is 0")
	}

}

//func TestContractManager_HostNegotiationError(t *testing.T) {
//	// set up storage connection for roll back operation
//	negotiationMsg := []uint64{simulation.FakeContractCreateRevisionFailed}
//	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()
//	negotiateTestBackend.SetSendMsg(negotiationMsg)
//
//	// create a fake contract manager
//	cm, err := NewFakeContractManager(negotiateTestBackend)
//	if err != nil {
//		t.Fatalf("failed to fake the contract manager: %s", err.Error())
//	}
//
//	// create contract parameter and insert host information
//	params := simulation.ContractParamsGenerator()
//	originalFailedInteraction := params.Host.RecentFailedInteractions
//	if err := insertStorageHostInfo(cm, params.Host); err != nil {
//		t.Fatalf("failed to insert host information: %s", err.Error())
//	}
//
//	// call the ContractCreteNegotiate
//	meta, err := cm.ContractCreateNegotiate(params)
//	if !common.ErrContains(err, storage.ErrHostNegotiate) {
//		t.Fatalf("error: expected error to contain %s, got %s", storage.ErrHostNegotiate.Error(), err.Error())
//	}
//
//	// check to see if the contract is saved in the db
//	if _, exist := cm.RetrieveActiveContract(meta.ID); exist {
//		t.Fatalf("negotiation failed, the contract should not be saved into database")
//	}
//
//	// check if the failed interaction is increased, negotiation failed, failed
//	// interaction is expected to be increased by 1
//	updatedInfo, exist := cm.hostManager.RetrieveHostInfo(params.Host.EnodeID)
//	if !exist {
//		t.Fatalf("failed to get the storage host information")
//	}
//
//	if updatedInfo.RecentFailedInteractions-originalFailedInteraction != 1 {
//		t.Fatalf("failed to increase the host's failed interaction after got host commit error -> original: %v, updated: %v",
//			originalFailedInteraction, updatedInfo.RecentFailedInteractions)
//	}
//}
//
//func TestContractManager_ClientNegotiationError(t *testing.T) {
//	// set up storage connection for roll back operation
//	negotiationMsg := []uint64{simulation.FakeContractCreateRevisionSendFailed}
//	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()
//	negotiateTestBackend.SetSendMsg(negotiationMsg)
//
//	// create a fake contract manager
//	cm, err := NewFakeContractManager(negotiateTestBackend)
//	if err != nil {
//		t.Fatalf("failed to fake the contract manager: %s", err.Error())
//	}
//
//	// create contract parameter and insert host information
//	params := simulation.ContractParamsGenerator()
//	originalFailedInteraction := params.Host.RecentFailedInteractions
//	if err := insertStorageHostInfo(cm, params.Host); err != nil {
//		t.Fatalf("failed to insert host information: %s", err.Error())
//	}
//
//	// call the ContractCreteNegotiate
//	meta, err := cm.ContractCreateNegotiate(params)
//	if !common.ErrContains(err, storage.ErrClientNegotiate) {
//		t.Fatalf("error: expected error to contain %s, got %s", storage.ErrClientNegotiate.Error(), err.Error())
//	}
//
//	// check to see if the contract is saved in the db
//	if _, exist := cm.RetrieveActiveContract(meta.ID); exist {
//		t.Fatalf("negotiation failed, the contract should not be saved into database")
//	}
//
//	// check if the failed interaction is increased, negotiation failed, failed
//	// interaction is expected to be increased by 1
//	updatedInfo, exist := cm.hostManager.RetrieveHostInfo(params.Host.EnodeID)
//	if !exist {
//		t.Fatalf("failed to get the storage host information")
//	}
//
//	if updatedInfo.RecentFailedInteractions-originalFailedInteraction != 0 {
//		t.Fatalf("cient error should not increase the host's failed interactions")
//	}
//}
