// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"fmt"
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

func (mr *merkleRoots) insert(id storage.ContractID, index int, root common.Hash) (err error) {
	for index > mr.numMerkleRoots {
		if err := mr.push(id, common.Hash{}); err != nil {
			return fmt.Errorf("failed to add roots: %s", err.Error())
		}
	}

	// if same, push to the merkle tree
	if index == mr.numMerkleRoots {
		return mr.push(id, root)
	}

	// index < mr.numMerkleRoots
	roots, err := mr.db.FetchMerkleRoots(id)
	if err != nil {
		return fmt.Errorf("failed to fetch merkle roots: %s", err.Error())
	}

	// before accessing the root with the index, check the length again
	if len(roots)-1 <= index {
		return mr.insert(id, mr.numMerkleRoots, root)
	}

	// replace the root and store the value into the database
	roots[index] = root
	if err = mr.db.StoreMerkleRoots(id, roots); err != nil {
		return
	}

	// check if the root is cached, update the record in memory
	position, cached := mr.isCached(index)
	if !cached {
		mr.uncachedRoots[position] = root
		return
	}

	// otherwise, cached tree reconstruction is required
	if err = mr.reconstructCachedTree(position); err != nil {
		return fmt.Errorf("failed to reconstruct the cached tree: %s", err.Error())
	}

	return
}

func (mr *merkleRoots) reconstructCachedTree(position int) (err error) {
	return
}

// isCached check if the root with the index is already constructed
// as a sub tree. if the index is cached, meaning the root with the index
// has been contracted as a sub tree
func (mr *merkleRoots) isCached(index int) (position int, cached bool) {
	if index/merkleRootsPerCache == len(mr.cachedSubTrees) {
		// un-cached
		return index - len(mr.cachedSubTrees)*merkleRootsPerCache, false
	}

	// cached
	return index / merkleRootsPerCache, true
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

	// add the root
	mr.appendRootMemory(root)

	mr.numMerkleRoots++

	return
}

func loadMerkleRoots(db *DB, roots []common.Hash) (mr *merkleRoots) {
	// initialize merkle roots
	mr = &merkleRoots{
		db: db,
	}

	mr.appendRootMemory(roots...)

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
			mr.uncachedRoots = mr.uncachedRoots[:0]
		}
	}
}

// TODO (mzhang): WIP
func cachedMerkleRoot(roots []common.Hash) (root common.Hash) {
	return
}

// len returns the current number of merkle roots inserted
func (mr *merkleRoots) len() int {
	return mr.numMerkleRoots
}
