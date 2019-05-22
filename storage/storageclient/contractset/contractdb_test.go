// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"crypto/rand"
	mathRand "math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

var testDB = "./testdata/contractsetdb"
var ch = contractHeaderGenerator()

func TestDB_StoreFetchContractHeader(t *testing.T) {
	db, err := OpenDB(testDB)
	if err != nil {
		t.Fatalf("failed to open / create a contractset database: %s", err.Error())
	}
	defer db.Close()

	// store the contract header
	if err := db.StoreContractHeader(ch); err != nil {
		t.Fatalf("failed to save the storage contract header information into database: %s", err.Error())
	}

	// fetch the contract header
	ch1, err := db.FetchContractHeader(ch.ID)
	if err != nil {
		t.Fatalf("failed to fetch the storage contract header: %s", err.Error())
	}

	if ch1.EnodeID != ch.EnodeID {
		t.Errorf("fetch failed, expected enodeid %s, got %s", ch.EnodeID, ch1.EnodeID)
	}
	if ch1.StartHeight != 100 {
		t.Errorf("fetch failed, expected start height 100, got %v", ch1.StartHeight)
	}
	if ch1.LatestContractRevision.NewRevisionNumber != 15 {
		t.Errorf("fetch failed, expected revision number to be 15, got %v", ch1.LatestContractRevision.NewRevisionNumber)
	}
}

func TestDB_StoreContractRoots(t *testing.T) {
	db, err := OpenDB(testDB)
	if err != nil {
		t.Fatalf("failed to open / create a contractset database: %s", err.Error())
	}
	defer db.Close()

	roots := contractRootsGenerator()

	// store the contract roots
	if err := db.StoreContractRoots(ch.ID, roots); err != nil {
		t.Fatalf("failed to save the storage contract roots information into database: %s", err.Error())
	}

	// fetch the contract roots
	roots1, err := db.FetchContractRoots(ch.ID)
	if err != nil {
		t.Fatalf("failed to fetch the storage contract roots: %s", err.Error())
	}

	if roots1[0] != roots[0] {
		t.Errorf("fetch failed, expected contract root index 0 to be %v, got %v",
			roots[0], roots1[0])
	}
}

func contractHeaderGenerator() (ch ContractHeader) {
	// generate the private key
	ch = ContractHeader{
		ID:      storageContractIDGenerator(),
		EnodeID: enodeIDGenerator(),
		LatestContractRevision: types.StorageContractRevision{
			ParentID:          randomHashGenerator(),
			NewRevisionNumber: 15,
		},
		PrivateKey:   "12345",
		StartHeight:  100,
		DownloadCost: common.RandomBigInt(),
		UploadCost:   common.RandomBigInt(),
		TotalCost:    common.RandomBigInt(),
	}

	return
}

func contractRootsGenerator() (roots []common.Hash) {
	mathRand.Seed(time.Now().UnixNano())
	randIndex := mathRand.Intn(20)

	for i := 0; i < randIndex; i++ {
		roots = append(roots, randomHashGenerator())
	}
	return
}

func enodeIDGenerator() (id enode.ID) {
	rand.Read(id[:])
	return
}

func storageContractIDGenerator() (id storage.ContractID) {
	rand.Read(id[:])
	return
}

func randomHashGenerator() (h common.Hash) {
	rand.Read(h[:])
	return
}
