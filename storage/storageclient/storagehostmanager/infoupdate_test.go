// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"errors"
	"testing"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

// fakeOnlineBackend is the fake backend used for testing in hostInfoUpdate
type fakeOnlineBackend struct{}

// Online method of the fakeOnlineBackend always returns true
func (fob *fakeOnlineBackend) Online() bool {
	return true
}

// fakeOfflineBackend is the fake backend used for testing in hostInfoUpdate
type fakeOfflineBackend struct{}

// Online method of the fakeOfflineBackend always return false
func (fob *fakeOfflineBackend) Online() bool {
	return false
}

// TestStorageHostManager_hostInfoUpdate_insert test the scenario of inserting the host info
// into the storage host manager
func TestStorageHostManager_hostInfoUpdate_insert(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	shm := &StorageHostManager{blockHeight: 1000000}
	evaluator := newDefaultEvaluator(shm, storage.RentPayment{})
	shm.hostEvaluator = evaluator
	shm.storageHostTree = storagehosttree.New(evaluator)

	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
		},
		EnodeID:             enodeID,
		FirstSeen:           0,
		AccumulatedUptime:   30,
		AccumulatedDowntime: 0,
	}
	if err := shm.hostInfoUpdate(info, &fakeOnlineBackend{}, nil); err != nil {
		t.Fatalf("cannot update the host info: %v", err)
	}
	info, exists := shm.RetrieveHostInfo(enodeID)
	if !exists {
		t.Fatalf("after host info update, does not insert")
	}
	if !info.AcceptingContracts {
		t.Fatalf("info update does not have accepting contracts true")
	}
}

// TestStorageHostManager_hostInfoUpdate_modify test the scenario of modifying the host info
// in the storage host manager
func TestStorageHostManager_hostInfoUpdate_modify(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	shm := &StorageHostManager{blockHeight: 1000000}
	evaluator := newDefaultEvaluator(shm, storage.RentPayment{})
	shm.hostEvaluator = evaluator
	shm.storageHostTree = storagehosttree.New(evaluator)

	if err := shm.storageHostTree.Insert(storage.HostInfo{EnodeID: enodeID}); err != nil {
		t.Fatalf("cannot insert the host info")
	}

	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
		},
		EnodeID:             enodeID,
		FirstSeen:           0,
		AccumulatedUptime:   30,
		AccumulatedDowntime: 0,
	}
	if err := shm.hostInfoUpdate(info, &fakeOnlineBackend{}, nil); err != nil {
		t.Fatalf("cannot update the host info: %v", err)
	}
	info, exists := shm.RetrieveHostInfo(enodeID)
	if !exists {
		t.Fatalf("after host info update, does not insert")
	}
	if !info.AcceptingContracts {
		t.Fatalf("info update does not have accepting contracts true")
	}
}

// TestStorageHostManager_hostInfoUpdate_remove test the scenario of removing the host info
// from the storage host manager
func TestStorageHostManager_hostInfoUpdate_remove(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	shm := &StorageHostManager{blockHeight: 1000000}
	evaluator := newDefaultEvaluator(shm, storage.RentPayment{})
	shm.hostEvaluator = evaluator
	shm.storageHostTree = storagehosttree.New(evaluator)

	info := storage.HostInfo{EnodeID: enodeID, FirstSeen: 0, AccumulatedUptime: 30, AccumulatedDowntime: 0}
	if err := shm.storageHostTree.Insert(info); err != nil {
		t.Fatalf("cannot insert into the storageHostTree: %v", err)
	}
	newInfo := storage.HostInfo{EnodeID: enodeID}
	if err := shm.hostInfoUpdate(newInfo, &fakeOnlineBackend{}, errors.New("")); err != nil {
		t.Fatalf("cannot update the host info: %v", err)
	}
	// the host should be removed from the storage host tree
	if _, exists := shm.RetrieveHostInfo(enodeID); exists {
		t.Fatalf("host info exist in storage host tree after update")
	}
}

// TestStorageHostManager_hostInfoUpdate_offline test the scenario of offline and returning an error.
// No update is expected.
func TestStorageHostManager_hostInfoUpdate_offline(t *testing.T) {
	enodeID := enode.ID{1, 2, 3, 4}
	shm := &StorageHostManager{blockHeight: 1000000}
	evaluator := newDefaultEvaluator(shm, storage.RentPayment{})
	shm.hostEvaluator = evaluator
	shm.storageHostTree = storagehosttree.New(evaluator)

	info := storage.HostInfo{EnodeID: enodeID, FirstSeen: 0, AccumulatedUptime: 30, AccumulatedDowntime: 0}
	if err := shm.storageHostTree.Insert(info); err != nil {
		t.Fatalf("cannot insert into the storageHostTree: %v", err)
	}

	newInfo := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
		},
		EnodeID:             enodeID,
		FirstSeen:           0,
		AccumulatedUptime:   30,
		AccumulatedDowntime: 0,
	}
	if err := shm.hostInfoUpdate(newInfo, &fakeOfflineBackend{}, errors.New("")); err != nil {
		t.Fatalf("cannot update the host info: %v", err)
	}
	_, exists := shm.RetrieveHostInfo(enodeID)
	if !exists {
		t.Fatalf("offline backend shall not remove the host")
	}
}
