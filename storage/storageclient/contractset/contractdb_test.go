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

func TestDB_StoreFetchDeleteContractHeader(t *testing.T) {
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

	// delete the contract header information
	if err := db.DeleteContractHeader(ch.ID); err != nil {
		t.Fatalf("failed to delete contract header from the db: %s", err.Error())
	}

	if _, err := db.FetchContractHeader(ch.ID); err == nil {
		t.Errorf("error: the contract header info should be deleted from the db")
	}
}

func TestDB_StoreFetchDeleteContractRoots(t *testing.T) {
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

	if !hashSliceComparator(roots1, roots) {
		t.Errorf("fetch failed, expected contract root index 0 to be %v, got %v",
			roots[0], roots1[0])
	}

	// delete the contract roots
	if err := db.DeleteContractRoots(ch.ID); err != nil {
		t.Fatalf("failed to delete the contract roots information from the db: %s", err.Error())
	}

	if _, err := db.FetchContractRoots(ch.ID); err == nil {
		t.Errorf("error: the contract roots info should be deleted from the db")
	}
}

func TestDB_StoreFetchDeleteAll(t *testing.T) {
	db, err := OpenDB(testDB)
	if err != nil {
		t.Fatalf("failed to open / create a contractset database: %s", err.Error())
	}
	defer db.Close()

	// store all
	roots := contractRootsGenerator()
	if err := db.StoreAll(ch, roots); err != nil {
		t.Fatalf("failed to save information into database: %s", err.Error())
	}

	// fetch all
	ch1, roots1, err := db.FetchAll(ch.ID)
	if err != nil {
		t.Fatalf("failed to retrieve information from the database: %s", err.Error())
	}

	// validate contract header information
	if ch1.EnodeID != ch.EnodeID {
		t.Errorf("fetch failed, expected enodeid %s, got %s", ch.EnodeID, ch1.EnodeID)
	}
	if ch1.StartHeight != 100 {
		t.Errorf("fetch failed, expected start height 100, got %v", ch1.StartHeight)
	}
	if ch1.LatestContractRevision.NewRevisionNumber != 15 {
		t.Errorf("fetch failed, expected revision number to be 15, got %v", ch1.LatestContractRevision.NewRevisionNumber)
	}

	// validate the contract roots information
	if !hashSliceComparator(roots1, roots) {
		t.Errorf("fetch failed, expected contract root index 0 to be %v, got %v",
			roots[0], roots1[0])
	}

	// delete all
	if err := db.DeleteAll(ch.ID); err != nil {
		t.Errorf("failed to delete information from the database: %s", err.Error())
	}

	// validation
	if _, _, err := db.FetchAll(ch.ID); err == nil {
		t.Errorf("information are not supposed to be fetched, they are all deleted")
	}
}

/*
 _____  _____  _______      __  _______ ______      ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|    |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__       | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____     | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|    |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

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

func hashSliceComparator(a []common.Hash, b []common.Hash) (equal bool) {
	for index, data := range a {
		if data != b[index] {
			return false
		}
	}
	return true
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
