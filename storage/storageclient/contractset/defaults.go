// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import "github.com/DxChainNetwork/godx/crypto"

const (
	persistDBName  = "contractset.db"
	persistWalName = "contractset.wal"

	dbContractHeader = ":contractheader"
	dbMerkleRoot     = ":roots"
)

const (
	// the height of the merkle tree is 7, meaning it can store
	// 128 merkle roots
	merkleRootsCacheHeight = 7

	// number of merkle roots in a cached tree is 128
	merkleRootsPerCache = 1 << merkleRootsCacheHeight

	SectorSize    = uint64(1 << 22) // 4 MiB
	remainingFile = -1
)

// sectorHeight is the height of the merkle tree constructed
// based on the data uploaded. Data uploaded will be divided
// into data pieces based on the
var sectorHeight = func() uint64 {
	height := uint64(0)
	for 1<<height < (SectorSize / crypto.MerkleLeafSize) {
		height++
	}
	return height
}()
