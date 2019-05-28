package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"path/filepath"
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
		if len(scs.contracts) != i {
			t.Fatalf("the contract length is expected to be %v, instead got %v",
				i, len(scs.contracts))
		}
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
