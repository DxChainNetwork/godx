// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
)

type merkleRoots struct {
	cachedSubTrees []*cachedSubTree
	uncachedRoots  []common.Hash
	numMerkleRoots int
	db             *DB
}

type cachedSubTree struct {
	height int
	sum    common.Hash
}

func newMerkleRoots(db *DB) (mk *merkleRoots) {
	return &merkleRoots{
		db: db,
	}
}

func (mr *merkleRoots) push(id storage.ContractID, root common.Hash) (err error) {
	// validation
	if len(mr.uncachedRoots) == merkleRootsPerCache {
		log.Crit("the number of uncachedRoots is too big, they should be cached")
	}

	// store the root into the database
	if err = mr.db.StoreSingleRoot(id, root); err != nil {
		return
	}

	// TODO (mzhang): add the root to uncached roots

	mr.numMerkleRoots++

	return
}

// appendRootMemory will store the root in the uncached roots field
// if the number of uncached roots reached a limit, then those
// roots will be build up to a cachedSubTree
func (mr *merkleRoots) appendRootMemory(roots ...common.Hash) {
	for _, root := range roots {
		mr.uncachedRoots = append(mr.uncachedRoots, root)
		if len(mr.uncachedRoots) == merkleRootsPerCache {
			mr.cachedSubTrees = append(mr.cachedSubTrees, newCachedSubTree(mr.uncachedRoots))
		}
	}
}

func newCachedSubTree(roots []common.Hash) (ct *cachedSubTree) {
	// input validation
	if len(roots) != merkleRootsPerCache {
		log.Crit("failed to create the cachedSubTree using the root provided")
	}

	// create the cachedSubTree
	return &cachedSubTree{
		height: int(merkleRootsCacheHeight + sectorHeight),
		sum:    cachedMerkleRoot(roots),
	}
}

// TODO (mzhang): WIP
func cachedMerkleRoot(roots []common.Hash) (root common.Hash) {
	return
}
