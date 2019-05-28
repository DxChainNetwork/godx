// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"testing"
)

func TestMerkleRoot_Push(t *testing.T) {
	// initialize storage contract id and new merkle root object
	id := storageContractIDGenerator()
	mk, err := newTestMerkleRoots(id)

	if err != nil {
		t.Fatalf("failed to create and initialize ")
	}

	defer mk.db.Close()

	for i := 1; i < 400; i++ {
		rootsOrign := rootsGenerator(i)

		// push the rootsOrign
		for _, r := range rootsOrign {
			if err := mk.push(r); err != nil {
				t.Fatalf("failed to push the root: %v", r)
			}
		}

		// check db first
		fetchedAll, err := mk.db.FetchAllRoots()
		if err != nil {
			t.Fatalf("failed to fetch all rootsOrign: %s", err.Error())
		}

		if len(fetchedAll) != 1 {
			t.Fatalf("db should only stored one list of merkle rootsOrign, instead of %v", len(fetchedAll))
		}

		fetchedWithID, err := mk.roots()
		if err != nil {
			t.Fatalf("failed to fetch the rootsOrign with stoarge ID: %s", err.Error())
		}

		if len(fetchedWithID) != len(rootsOrign) {
			t.Fatalf("db should only stored 10 rootsOrign with the ID %v, instead got %v", id, len(fetchedWithID))
		}

		for i, r := range fetchedWithID {
			if rootsOrign[i] != r {
				t.Errorf("the root stored in the db does not match with the rootsOrign fed. Expected %v, got %v",
					rootsOrign[i], r)
			}
			if rootsOrign[i] != fetchedAll[0][i] {
				t.Errorf("the root stored in the db, fetched with all, does not match with the rootsOrign fed. Expected %v, got %v",
					rootsOrign[i], fetchedAll[0][i])
			}
		}

		// check numMerkleRoots
		if mk.numMerkleRoots != len(rootsOrign) {
			t.Fatalf("numMerkleRoots does not match with the num of rootsOrign fed. Expected %v, got %v",
				len(rootsOrign), mk.numMerkleRoots)
		}

		// check memory uncachedRoots
		if i%128 == 0 {
			if len(mk.uncachedRoots) != 0 {
				t.Fatalf("the number of uncached rootsOrign is expected to be 0, instead got %v", len(mk.uncachedRoots))
			}

		} else {
			if len(mk.uncachedRoots) != (i - 128*(i/128)) {
				t.Fatalf("the number of uncached rootsOrign is expected to be %v, instead got %v",
					i, len(mk.uncachedRoots))
			}
		}

		//  check cachedSubTrees
		if len(mk.cachedSubTrees) != i/128 {
			t.Fatalf("the number of cached subtree is expected to be %v, instead got %v",
				i/128, len(mk.cachedSubTrees))
		}

		// clear the data saved in the db and memory, prepare for the next iteration
		mk.numMerkleRoots = 0
		mk.uncachedRoots = mk.uncachedRoots[:0]
		mk.cachedSubTrees = mk.cachedSubTrees[:0]
		mk.db.EmptyDB()
	}
}

func TestMerkleRoot_newMerkleRootPreview(t *testing.T) {
	// initialize storage contract id and new merkle root object
	id := storageContractIDGenerator()
	mk, err := newTestMerkleRoots(id)

	if err != nil {
		t.Fatalf("failed to create and initialize ")
	}

	defer mk.db.Close()

	for i := 1; i < 400; i++ {
		originalRoots := rootsGenerator(i)
		newRoot := randomHashGenerator()

		// push the roots
		for _, r := range originalRoots {
			if err := mk.push(r); err != nil {
				t.Fatalf("failed to push the root: %v", r)
			}
		}

		mroot, err := mk.newMerkleRootPreview(newRoot)
		if err != nil {
			t.Fatalf("failed to preview the new merkle root: %s", err.Error())
		}

		// calculate the expected value
		ct := crypto.NewCachedMerkleTree(sectorHeight)
		for _, r := range originalRoots {
			ct.Push(r)
		}
		ct.Push(newRoot)

		if mroot != ct.Root() {
			t.Fatalf("the calculated merkle roots are not matched when the number of roots is %v. Expected %v, got %v",
				i, ct.Root(), mroot)
		}

		// clear the data saved in the db and memory, prepare for the next iteration
		mk.numMerkleRoots = 0
		mk.uncachedRoots = mk.uncachedRoots[:0]
		mk.cachedSubTrees = mk.cachedSubTrees[:0]
		mk.db.EmptyDB()

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

func rootsGenerator(rootCount int) (roots []common.Hash) {
	for i := 0; i < rootCount; i++ {
		roots = append(roots, randomHashGenerator())
	}
	return
}

func newTestMerkleRoots(id storage.ContractID) (mk *merkleRoots, err error) {

	// initialize new db
	db, err := OpenDB("./testdata/contractsetdb")
	if err != nil {
		return
	}

	// initialize merkle roots object
	mk = newMerkleRoots(db, id)

	return
}
