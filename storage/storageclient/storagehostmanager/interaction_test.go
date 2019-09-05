// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

func TestInteractionName(t *testing.T) {
	tests := []struct {
		it   InteractionType
		name string
	}{
		{InteractionGetConfig, "host config scan"},
		{InteractionCreateContract, "create contract"},
		{InteractionRenewContract, "renew contract"},
		{InteractionUpload, "upload"},
		{InteractionDownload, "download"},
	}
	for index, test := range tests {
		name := test.it.String()
		if name != test.name {
			t.Errorf("test %d interaction name not expected. Got %v, Expect %v", index, name, test.name)
		}
		it := InteractionNameToType(test.name)
		if it != test.it {
			t.Errorf("test %d interaction type not expected. Got %v, Expect %v", index, it, test.it)
		}
	}
}

func TestInteractionNameInvalid(t *testing.T) {
	invalidName := "ssss"
	if InteractionNameToType(invalidName) != InteractionInvalid {
		t.Errorf("invalid name does not yield expected result")
	}
	if InteractionInvalid.String() != "" {
		t.Errorf("invalid type does not yield empty name")
	}
	if (InteractionType(100)).String() != "" {
		t.Errorf("invalid type does not yield empty name")
	}
}

func TestInteractionWeight(t *testing.T) {
	tests := []struct {
		it             InteractionType
		expectedWeight float64
	}{
		{InteractionInvalid, 0},
		{InteractionGetConfig, 1},
		{InteractionCreateContract, 2},
		{InteractionRenewContract, 5},
		{InteractionUpload, 5},
		{InteractionDownload, 10},
	}
	for _, test := range tests {
		res := interactionWeight(test.it)
		if res != test.expectedWeight {
			t.Errorf("test %v weight not expected", test.it)
		}
	}
}

func TestInteractionInitiate(t *testing.T) {
	tests := []struct {
		successBefore   float64
		failedBefore    float64
		expectedSuccess float64
		expectedFailed  float64
	}{
		{1, 0, 1, 0},
		{0, 1, 0, 1},
		{0, 0, initialSuccessfulInteractionFactor, initialFailedInteractionFactor},
	}
	for _, test := range tests {
		info := storage.HostInfo{
			SuccessfulInteractionFactor: test.successBefore,
			FailedInteractionFactor:     test.failedBefore,
		}
		interactionInitiate(&info)
		if info.SuccessfulInteractionFactor != test.expectedSuccess {
			t.Errorf("successful interaction not expected")
		}
		if info.FailedInteractionFactor != test.expectedFailed {
			t.Errorf("failed interaction not expected")
		}
	}
}

func TestUpdateInteractionRecord(t *testing.T) {
	tests := []struct {
		recordSize int
	}{
		{0}, {1}, {maxNumInteractionRecord},
	}
	for _, test := range tests {
		info := storage.HostInfo{}
		for i := 0; i != test.recordSize; i++ {
			info.InteractionRecords = append(info.InteractionRecords, storage.HostInteractionRecord{
				Time:            time.Unix(0, 0),
				InteractionType: "test interaction",
				Success:         true,
			})
		}
		updateInteractionRecord(&info, InteractionGetConfig, true, 0)
		size := len(info.InteractionRecords)
		if test.recordSize >= maxNumInteractionRecord {
			if size != maxNumInteractionRecord {
				t.Errorf("after update, interaction record size not expected. Got %v, Expect %v", size, maxNumInteractionRecord)
			}
		} else {
			if size != test.recordSize+1 {
				t.Errorf("after update, interaction record size not expected. Got %v, Expect %v", size, test.recordSize+1)
			}
		}
	}
}

// TestStorageHostManager_IncrementSuccessfulInteractions test StorageHostManager.IncrementSuccessfulInteractions
func TestStorageHostManager_IncrementSuccessfulInteractions(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	info := storage.HostInfo{EnodeID: enodeID, SuccessfulInteractionFactor: 10, FailedInteractionFactor: 10}
	shm := &StorageHostManager{}
	shm.hostEvaluator = newDefaultEvaluator(shm, storage.RentPayment{})
	shm.storageHostTree = storagehosttree.New()
	score := shm.hostEvaluator.Evaluate(info)
	if err := shm.storageHostTree.Insert(info, score); err != nil {
		t.Fatal("cannot insert into the storage host tree: ", err)
	}
	prevSc := interactionScoreCalc(info)

	shm.IncrementSuccessfulInteractions(enodeID, InteractionGetConfig)
	newInfo, _, exist := shm.storageHostTree.RetrieveHostInfo(enodeID)
	if !exist {
		t.Fatalf("node %v not exist", enodeID)
	}
	newSc := interactionScoreCalc(newInfo)
	if prevSc >= newSc {
		t.Errorf("After success update, interaction not increasing: %v -> %v", prevSc, newSc)
	}
}

// TestStorageHostManager_IncrementFailedInteractions test StorageHostManager.IncrementSuccessfulInteractions
func TestStorageHostManager_IncrementFailedInteractions(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	info := storage.HostInfo{EnodeID: enodeID, SuccessfulInteractionFactor: 10, FailedInteractionFactor: 10}
	shm := &StorageHostManager{}
	shm.hostEvaluator = newDefaultEvaluator(shm, storage.RentPayment{})
	shm.storageHostTree = storagehosttree.New()
	score := shm.hostEvaluator.Evaluate(info)
	if err := shm.storageHostTree.Insert(info, score); err != nil {
		t.Fatal("cannot insert into the storage host tree: ", err)
	}
	prevSc := interactionScoreCalc(info)

	shm.IncrementFailedInteractions(enodeID, InteractionGetConfig)
	newInfo, _, exist := shm.storageHostTree.RetrieveHostInfo(enodeID)
	if !exist {
		t.Fatalf("node %v not exist", enodeID)
	}
	newSc := interactionScoreCalc(newInfo)
	if prevSc <= newSc {
		t.Errorf("After success update, interaction not increasing: %v -> %v", prevSc, newSc)
	}
}
