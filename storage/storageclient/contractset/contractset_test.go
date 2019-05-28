package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"path/filepath"
	"sync"
	"testing"
)

var persistDir = "./testdata/"

func TestStorageContractSet_New(t *testing.T) {
	// insert 200 contracts
	for i := 1; i < 200; i++ {
		// insert data into the database
		chs, _, err := fillDB(persistDir, i, 148)
		if err != nil {
			t.Fatalf("error inserting the contract into db: %s", err.Error())
		}

		// initialize storage contract set
		scs, err := New(persistDir)
		if err != nil {
			t.Fatalf("failed to initialize storage contract set: %s", err.Error())
		}

		// in case this test case went wrong, it will not affect the test cases
		// that in going to run later on
		defer scs.Close()
		defer scs.db.EmptyDB()

		// validation number of contracts and hostToContractID mapping
		if len(scs.contracts) != i {
			t.Fatalf("expected number of contracts %v, got %v", i, len(scs.contracts))
		}

		if len(scs.hostToContractID) != i {
			t.Fatalf("expected number of hostToContractID mapping is %v, got %v", i,
				len(scs.hostToContractID))
		}

		// validation contract id
		for _, ch := range chs {
			if _, exists := scs.contracts[ch.ID]; !exists {
				t.Fatalf("the contract id %v is not stored in the contracts", ch.ID)
			}

			id, exists := scs.hostToContractID[ch.EnodeID]
			if !exists {
				t.Fatalf("the enodeID %v should be stored in the hostToContractID mapping",
					ch.EnodeID)
			}

			if id != ch.ID {
				t.Fatalf("contract id stored in the hostToContractID does not match with the one inserted. Expected %v, got %v",
					ch.ID, id)
			}

			// verify the roots
			if scs.contracts[ch.ID].merkleRoots.id != ch.ID {
				t.Fatalf("roots validation failed, expected id %v, got id %v",
					ch.ID, scs.contracts[ch.ID].merkleRoots.id)
			}

			if scs.contracts[ch.ID].merkleRoots.numMerkleRoots != 148 {
				t.Fatalf("roots validation failed, expected numMerkleRoots %v, got %v",
					148, scs.contracts[ch.ID].merkleRoots.numMerkleRoots)
			}

			if len(scs.contracts[ch.ID].merkleRoots.uncachedRoots) != 148-128 {
				t.Fatalf("roots validataion failed, expected uncachedRoots length %v, got %v",
					148-128, len(scs.contracts[ch.ID].merkleRoots.uncachedRoots))
			}

			if len(scs.contracts[ch.ID].merkleRoots.cachedSubTrees) != 1 {
				t.Fatalf("roots validation failed, expected length of cachedSubTrees %v, got %v",
					1, len(scs.contracts[ch.ID].merkleRoots.cachedSubTrees))
			}

		}

		// emptyDB
		scs.db.EmptyDB()
		scs.db.Close()

	}
}

func TestStorageContractSet_InsertContract(t *testing.T) {
	scs, err := New(persistDir)
	if err != nil {
		t.Fatalf("failed to initialize storage contract set: %s", err.Error())
	}
	defer scs.Close()
	defer scs.db.EmptyDB()

	// insert contract
	for i := 0; i < 200; i++ {
		ch := contractHeaderGenerator()
		rts := rootsGenerator(158)

		if _, err := scs.InsertContract(ch, rts); err != nil {
			t.Fatalf("failed to insert the contract: %s", err.Error())
		}

		// validation
		if len(scs.contracts) != i+1 {
			t.Fatalf("the contract length is expected to be %v, instead got %v",
				i, len(scs.contracts))
		}

		// check db contract header
		fetchedHeader, err := scs.db.FetchContractHeader(ch.ID)
		if err != nil {
			t.Fatalf("failed to get the contract header information from the db %s", err.Error())
		}
		if fetchedHeader.EnodeID != ch.EnodeID {
			t.Fatalf("the fetched header does not match the header inserted. Expected %v, got %v",
				ch.EnodeID, fetchedHeader.EnodeID)
		}

		// check db contract root
		fetchedRoot, err := scs.db.FetchMerkleRoots(ch.ID)
		if err != nil {
			t.Fatalf("failed to get the contract merkle root information from the db %s", err.Error())
		}
		if !hashSliceComparator(fetchedRoot, rts) {
			t.Fatalf("the fetched root does not match the root inserted. Expected %v, got %v",
				rts, fetchedRoot)
		}
	}

}

func TestStorageContractSet_InsertContractMultiRoutines(t *testing.T) {
	var wg sync.WaitGroup

	scs, err := New(persistDir)

	if err != nil {
		t.Fatalf("failed to initialize storage contract set: %s", err.Error())
	}

	// making sure db and memory is empty
	clearAll(scs)

	defer scs.Close()
	defer scs.db.EmptyDB()

	var chList []ContractHeader
	var rootsList = make(map[storage.ContractID][]common.Hash)

	for i := 0; i < 200; i++ {
		ch := contractHeaderGenerator()
		rts := rootsGenerator(158)
		chList = append(chList, ch)
		rootsList[ch.ID] = rts

		go contractInsertTestWrapper(scs, &wg, ch, rts, t)
	}

	// wait until all contracts are inserted
	wg.Wait()

	// check if correct contracts are inserted (in memory check)
	if len(scs.contracts) != 200 {
		t.Fatalf("the expected number of contracts is 200, got %v", len(scs.contracts))
	}

	if len(scs.hostToContractID) != 200 {
		t.Fatalf("the expected number of host contract mapping is 200, got %v",
			len(scs.hostToContractID))
	}

	// check if the contract is inside the contracts list
	for _, ch := range chList {
		memContract, exists := scs.contracts[ch.ID]
		if !exists {
			t.Fatalf("failed to insert the contract with id %v, contract cannot be found in the contracts",
				ch.ID)
		}

		rt, exists := rootsList[ch.ID]
		if !exists {
			t.Fatalf("the root with id %v, supppose to be stored in the rootsList", ch.ID)
		}

		if memContract.merkleRoots.numMerkleRoots != 158 {
			t.Fatalf("the numMerkleRoots is supposed to be 158, instead got %v",
				memContract.merkleRoots.numMerkleRoots)
		}

		// check db contract root
		fetchedRoot, err := scs.db.FetchMerkleRoots(ch.ID)
		if err != nil {
			t.Fatalf("failed to get the contract merkle root information from the db %s", err.Error())
		}
		if !hashSliceComparator(fetchedRoot, rt) {
			t.Fatalf("the fetched root does not match the root inserted. Expected %v, got %v",
				rt, fetchedRoot)
		}
	}

}

func TestStorageContractSet_Acquire(t *testing.T) {
	scs, err := New(persistDir)

	if err != nil {
		t.Fatalf("failed to initialize storage contract set: %s", err.Error())
	}

	// making sure db and memory is empty
	clearAll(scs)

	defer scs.Close()
	defer scs.db.EmptyDB()

	// insert a contract
	ch := contractHeaderGenerator()
	rts := rootsGenerator(158)
	if _, err := scs.InsertContract(ch, rts); err != nil {
		t.Errorf("failed to insert the contract with ID %v: %s", ch.ID, err.Error())
		return
	}

	// acquire the contract
	c, exists := scs.Acquire(ch.ID)
	if !exists {
		t.Fatalf("failed to acquire the contract")
	}

	if c.header.EnodeID != ch.EnodeID {
		t.Fatalf("the acquired contract does not match with the contract inserted, expected enode id %v, got %v",
			ch.EnodeID, c.header.EnodeID)
	}

	// return
	if err := scs.Return(c); err != nil {
		t.Fatalf("failed to return the contract: %s", err.Error())
	}

	// delete
	c, exists = scs.Acquire(ch.ID)
	if !exists {
		t.Fatalf("failed to acquire the contract")
	}

	if err := scs.Delete(c); err != nil {
		t.Fatalf("failed to delete the contract")
	}

	if _, exists := scs.contracts[ch.ID]; exists {
		t.Fatalf("delete failed, the contract should not be inside the contract list")
	}

	if _, exists := scs.hostToContractID[ch.EnodeID]; exists {
		t.Fatalf("delete failed, the hostContract mapping should not exist")
	}

	if _, err := scs.db.FetchContractHeader(ch.ID); err == nil {
		t.Fatalf("delete failed, the contract header should not be saved in the db")
	}

	if _, err := scs.db.FetchMerkleRoots(ch.ID); err == nil {
		t.Fatalf("delete failed, the contract merkle root should not be saved in the db")
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

func contractInsertTestWrapper(scs *StorageContractSet, wg *sync.WaitGroup, ch ContractHeader, roots []common.Hash, t *testing.T) {
	wg.Add(1)
	defer wg.Done()
	if _, err := scs.InsertContract(ch, roots); err != nil {
		t.Errorf("failed to insert the contract with ID %v: %s", ch.ID, err.Error())
		return
	}

	return
}

func clearAll(scs *StorageContractSet) {
	scs.db.EmptyDB()
	scs.contracts = make(map[storage.ContractID]*Contract)
	scs.hostToContractID = make(map[enode.ID]storage.ContractID)
}

func fillDB(persistDir string, contractCount, rootCount int) (chs []ContractHeader, roots [][]common.Hash, err error) {
	db, err := OpenDB(filepath.Join(persistDir, persistDBName))
	if err != nil {
		return
	}
	defer db.Close()

	// insert contract header and contract root information into the db and memory
	for i := 0; i < contractCount; i++ {
		// generate contract header
		ch := contractHeaderGenerator()

		// generate contract root
		rts := rootsGenerator(rootCount)

		// store the information into the database
		if err = db.StoreHeaderAndRoots(ch, rts); err != nil {
			return
		}

		// append
		chs = append(chs, ch)
		roots = append(roots, rts)
	}

	return
}
