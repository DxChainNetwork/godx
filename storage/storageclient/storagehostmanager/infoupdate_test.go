// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

//func TestStorageHostManager_IncrementSuccessfulInteractions(t *testing.T) {
//	shm := newHostManagerTestData()
//	hi := hostInfoGenerator()
//
//	if err := shm.insert(hi); err != nil {
//		t.Fatalf("failed to insert data into the storage host tree")
//	}
//
//	shm.IncrementSuccessfulInteractions(hi.EnodeID)
//	hiUpdated, exists := shm.storageHostTree.RetrieveHostInfo(hi.EnodeID)
//	if !exists {
//		t.Fatalf("failed to retrieve the storage host information with id %s", hi.EnodeID)
//	}
//
//	if hiUpdated.RecentSuccessfulInteractions != hi.RecentSuccessfulInteractions+1 {
//		t.Errorf("failed to increament the recent successful interactions, expected %v, got %v",
//			hiUpdated.RecentSuccessfulInteractions, hi.RecentSuccessfulInteractions+1)
//	}
//
//}
//
//func TestStorageHostManager_IncrementFailedInteractions(t *testing.T) {
//	shm := newHostManagerTestData()
//	hi := hostInfoGenerator()
//
//	if err := shm.insert(hi); err != nil {
//		t.Fatalf("failed to insert data into the storage host tree")
//	}
//
//	shm.IncrementFailedInteractions(hi.EnodeID)
//	hiUpdated, exists := shm.storageHostTree.RetrieveHostInfo(hi.EnodeID)
//	if !exists {
//		t.Fatalf("failed to retrieve the storage host information with id %s", hi.EnodeID)
//	}
//
//	if hiUpdated.RecentFailedInteractions != hi.RecentFailedInteractions+1 {
//		t.Errorf("failed to increament the recent failed interactions, expected %v, got %v",
//			hiUpdated.RecentFailedInteractions, hi.RecentFailedInteractions+1)
//	}
//}
