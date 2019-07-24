// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
)

// merkleRoots contained a bunch of uploaded data merkle roots
type merkleRoots struct {
	cachedSubTrees []*cachedSubTree
	uncachedRoots  []common.Hash
	numMerkleRoots int
	db             *DB
	id             storage.ContractID
}

type cachedSubTree struct {
	height int
	sum    common.Hash
}

func newMerkleRoots(db *DB, id storage.ContractID) (mk *merkleRoots) {
	return &merkleRoots{
		db: db,
		id: id,
	}
}

func newCachedSubTree(roots []common.Hash) (ct *cachedSubTree, err error) {
	// input validation
	if len(roots) != merkleRootsPerCache {
		log.Error("failed to create the cachedSubTree using the root provided")
		return nil, fmt.Errorf("failed to create the cachedSubTree using the root provided")
	}

	// create the cachedSubTree, where the height of the sub tree
	// will be the height of the cached tree + the height of the
	// merkle tree constructed by the data sector, which are both
	// constant
	return &cachedSubTree{
		height: int(merkleRootsCacheHeight + sectorHeight),
		sum:    merkle.Sha256CachedTreeRoot(roots, sectorHeight),
	}, nil
}

// loadMerkleRoots will load all merkle roots saved in the dab, which
// will then be saved into the memory
func loadMerkleRoots(db *DB, id storage.ContractID, roots []common.Hash) (mr *merkleRoots, err error) {

	// initialize merkle roots
	mr = &merkleRoots{
		db: db,
		id: id,
	}

	err = mr.appendRootMemory(roots...)
	mr.numMerkleRoots = len(roots)

	return
}

// push will store the root passed in into database first, then it will
// be saved into the memory
func (mr *merkleRoots) push(root common.Hash) (err error) {
	// validation
	if len(mr.uncachedRoots) == merkleRootsPerCache {
		log.Crit("the number of uncachedRoots is too big, they should be cached")
	}

	// store the root into the database
	if err = mr.db.StoreSingleRoot(mr.id, root); err != nil {
		return
	}

	// add the root
	if err := mr.appendRootMemory(root); err != nil {
		return err
	}

	mr.numMerkleRoots++

	return
}

// appendRootMemory will store the root in the uncached roots field
// if the number of uncached roots reached a limit, then those
// roots will be build up to a cachedSubTree
func (mr *merkleRoots) appendRootMemory(roots ...common.Hash) error {
	for _, root := range roots {
		mr.uncachedRoots = append(mr.uncachedRoots, root)
		if len(mr.uncachedRoots) == merkleRootsPerCache {
			cachedTree, err := newCachedSubTree(mr.uncachedRoots)
			if err != nil {
				return err
			}

			mr.cachedSubTrees = append(mr.cachedSubTrees, cachedTree)
			mr.uncachedRoots = mr.uncachedRoots[:0]
		}
	}

	return nil
}

// newMerkleRootPreview will display the new merkle root when a newRoot is passed in.
// Note: this is only a preview, root will not be saved into the memory nor db
func (mr *merkleRoots) newMerkleRootPreview(newRoot common.Hash) (mroot common.Hash, err error) {
	// create a new cached merkle tree
	ct := merkle.NewSha256CachedTree(sectorHeight)

	// append all cachedSubTrees first
	for _, sub := range mr.cachedSubTrees {
		if err = ct.PushSubTree(sub.height, sub.sum); err != nil {
			return
		}
	}

	// append uncached root
	for _, root := range mr.uncachedRoots {
		ct.Push(root)
	}

	// push the newRoot and calculate the merkle root
	ct.Push(newRoot)
	mroot = ct.Root()
	return
}

// roots will return all roots saved in the database which belongs to the contract id
func (mr *merkleRoots) roots() (roots []common.Hash, err error) {
	if roots, err = mr.db.FetchMerkleRoots(mr.id); err != nil {
		return
	}

	if len(roots) != mr.numMerkleRoots {
		log.Crit("Error: the length of retrieved merkle roots does not match with the number of merkle roots stored in the memory")
	}

	return
}

// len returns the current number of merkle roots inserted
func (mr *merkleRoots) len() int {
	return mr.numMerkleRoots
}
