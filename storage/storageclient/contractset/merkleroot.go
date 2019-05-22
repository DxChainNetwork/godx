// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
)

type merkleRoots struct {
	cachedSubTrees []cachedSubTree
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

// TODO (mzhang): WIP
func (mr *merkleRoots) push(root common.Hash) (err error) {
	if len(mr.uncachedRoots) == merkleRootsPerCache {
		log.Crit("the number of uncachedRoots is too big, they should be cached")
	}

	//if err = mr.db.
	return
}
