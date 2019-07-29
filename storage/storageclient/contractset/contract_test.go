// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
)

func TestContract_UpdateStatus(t *testing.T) {
	contract, err := newContract()
	if err != nil {
		t.Fatalf("failed to generate new contract: %s", err.Error())
	}

	defer contract.db.Close()
	defer contract.db.EmptyDB()

	tables := []struct {
		upload   bool
		renew    bool
		canceled bool
	}{
		{true, true, true},
		{false, false, false},
		{false, true, false},
	}

	for _, table := range tables {
		status := contractStatusGenerator(table.upload, table.renew, table.canceled)
		err := contract.UpdateStatus(status)
		if err != nil {
			t.Fatalf("error updating the contract status: %s", err.Error())
		}
		fetchedStatus := contract.Status()
		if fetchedStatus != status {
			t.Fatalf("error updating the contract status: expected %v, got %v",
				fetchedStatus, status)
		}
	}
}

func TestContract_RecordUploadPreRev(t *testing.T) {
	contract, err := newContract()
	if err != nil {
		t.Fatalf("failed to generate new contract: %s", err.Error())
	}

	defer contract.db.Close()
	defer contract.db.EmptyDB()

	//storageCost := common.RandomBigInt()
	//bandWidthCost := common.RandomBigInt()
	//root := randomRootGenerator()

	wt, err := contract.UndoRevisionLog(contract.header)
	if err != nil {
		t.Fatalf("failed to record the upload pre revision: %s", err.Error())
	}

	defer wt.Release()

	// load from the wal
	_, txns, err := writeaheadlog.New("./testdata/contractset.wal")
	if err != nil {
		return
	}

	// verify number of transactions
	if len(txns) != 1 {
		t.Fatalf("Expected one transaction is recorded, instead, got %v", len(txns))
	}

	// verify the number of operations
	if len(txns[0].Operations) != len(wt.Operations) {
		t.Fatalf("Expected %v operations, instead got %v", len(wt.Operations),
			txns[0].Operations)
	}

	var walCh1 walContractHeaderEntry
	var walCh2 walContractHeaderEntry
	var walR1 walRootsEntry
	var walR2 walRootsEntry

	for i, op := range txns[0].Operations {
		switch op.Name {
		case dbContractHeader:
			if err := json.Unmarshal(op.Data, &walCh1); err != nil {
				t.Fatalf("failed to decode contract header 1: %s", err.Error())
			}
			if err := json.Unmarshal(wt.Operations[i].Data, &walCh2); err != nil {
				t.Fatalf("failed to decode contract header 2: %s", err.Error())
			}

			if err := contractHeaderComparator(walCh1.Header, walCh2.Header); err != nil {
				t.Fatalf("two contract headers are not equal to each other: %s", err.Error())
			}
		case dbMerkleRoot:
			if err := json.Unmarshal(op.Data, &walR1); err != nil {
				t.Fatalf("failed to decode roots information 1: %s", err.Error())
			}

			if err := json.Unmarshal(wt.Operations[i].Data, &walR2); err != nil {
				t.Fatalf("failed to decode roots information 2: %s", err.Error())
			}

			if walR1.Root != walR2.Root {
				t.Fatalf("two roots are not equal to each other: expected %v, got %v",
					walR2.Root, walR1.Root)
			}
		}
	}
}

func TestContract_CommitUpload(t *testing.T) {
	contract, err := newContract()
	if err != nil {
		t.Fatalf("failed to generate new contract: %s", err.Error())
	}

	defer contract.db.Close()
	defer contract.db.EmptyDB()

	storageCost := common.RandomBigInt()
	bandWidthCost := common.RandomBigInt()
	root := randomRootGenerator()

	wt, err := contract.UndoRevisionLog(contract.header)
	if err != nil {
		t.Fatalf("failed to record the upload pre revision: %s", err.Error())
	}

	if err := contract.CommitUpload(wt, storageContractRevisionGenerator(), storageCost, bandWidthCost); err != nil {
		t.Fatalf("failed to commit upload: %s", err.Error())
	}

	if err := contract.merkleRoots.push(root); err != nil {
		t.Fatalf("push merkle roots failed: %s", err.Error())
	}

	// verify the correct root has been inserted into the db
	r, err := contract.db.FetchMerkleRoots(contract.header.ID)
	if err != nil {
		t.Fatalf("failed to fetch the merkle root information from the db: %s", err.Error())
	}

	if len(r) != 1 {
		t.Fatalf("the expected length of the roots is 1, instead got %v with value %v", len(r), r)
	}

	if r[0] != root {
		t.Fatalf("root getting from the db does not match with the root inserted. Expected %v, got %v",
			root, r[0])
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

func newContract() (c *Contract, err error) {
	// initialize wal transaction
	wal, _, err := writeaheadlog.New("./testdata/contractset.wal")
	if err != nil {
		return
	}

	// remove the existed values first before initializing the new db
	os.RemoveAll("./testdata/contractsetdb")

	// initialize new db
	db, err := OpenDB("./testdata/contractsetdb")
	if err != nil {
		return
	}

	// initialize contract header
	ch := contractHeaderGenerator()

	// initialize merkle roots object
	mk := newMerkleRoots(db, ch.ID)

	// initialize contract
	c = &Contract{
		header:      ch,
		wal:         wal,
		db:          db,
		merkleRoots: mk,
	}

	return
}

func contractHeaderComparator(ch1, ch2 ContractHeader) (err error) {
	if ch1.ID != ch2.ID {
		return fmt.Errorf("expected id %v, got id %v",
			ch2.ID, ch1.ID)
	}

	if ch1.EnodeID != ch2.EnodeID {
		return fmt.Errorf("expected enodeid %v, got enodeid %v",
			ch2.EnodeID, ch1.EnodeID)
	}

	if ch1.PrivateKey != ch2.PrivateKey {
		return fmt.Errorf("expected pravate key %v, got %v", ch2.PrivateKey, ch1.PrivateKey)
	}

	if ch1.StartHeight != ch2.StartHeight {
		return fmt.Errorf("expected start height %v, got start height %v",
			ch2.StartHeight, ch1.StartHeight)
	}

	if ch1.UploadCost.Cmp(ch2.UploadCost) != 0 {
		return fmt.Errorf("expected upload cost %v, got %v", ch2.UploadCost, ch1.UploadCost)
	}

	return
}

func storageContractRevisionGenerator() types.StorageContractRevision {
	return types.StorageContractRevision{
		NewWindowStart: 100,
		NewValidProofOutputs: []types.DxcoinCharge{
			{randomAddress(), big.NewInt(100)},
			{randomAddress(), big.NewInt(5000)},
		},
	}
}

func contractStatusGenerator(upload, renew, canceled bool) (cs storage.ContractStatus) {
	return storage.ContractStatus{
		UploadAbility: upload,
		RenewAbility:  renew,
		Canceled:      canceled,
	}
}

func randomRootGenerator() (r common.Hash) {
	rand.Seed(time.Now().UnixNano())
	rand.Read(r[:])
	return
}

func randomAddress() (addr common.Address) {
	rand.Seed(time.Now().UnixNano())
	rand.Read(addr[:])
	return
}
