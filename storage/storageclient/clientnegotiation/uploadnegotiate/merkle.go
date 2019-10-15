// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiate

import (
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

// CalculateProofRanges will calculate the proof ranges which is used to verify a
// pre-modification Merkle diff proof for the specified actions.
func calculateProofRanges(actions []storage.UploadAction, oldNumSectors uint64) []merkle.SubTreeLimit {
	newNumSectors := oldNumSectors
	sectorsChanged := make(map[uint64]struct{})
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			sectorsChanged[newNumSectors] = struct{}{}
			newNumSectors++
		}
	}

	oldRanges := make([]merkle.SubTreeLimit, 0, len(sectorsChanged))
	for sectorNum := range sectorsChanged {
		if sectorNum < oldNumSectors {
			oldRanges = append(oldRanges, merkle.SubTreeLimit{
				Left:  sectorNum,
				Right: sectorNum + 1,
			})
		}
	}
	sort.Slice(oldRanges, func(i, j int) bool {
		return oldRanges[i].Left < oldRanges[j].Left
	})

	return oldRanges
}

// ModifyProofRanges will modify the proof ranges produced by calculateProofRanges
// to verify a post-modification Merkle diff proof for the specified actions.
func modifyProofRanges(proofRanges []merkle.SubTreeLimit, actions []storage.UploadAction, numSectors uint64) []merkle.SubTreeLimit {
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			proofRanges = append(proofRanges, merkle.SubTreeLimit{
				Left:  numSectors,
				Right: numSectors + 1,
			})
			numSectors++
		}
	}
	return proofRanges
}

// ModifyLeaves will modify the leaf hashes of a Merkle diff proof to verify a
// post-modification Merkle diff proof for the specified actions.
func modifyLeaves(leafHashes []common.Hash, actions []storage.UploadAction, numSectors uint64) []common.Hash {
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			leafHashes = append(leafHashes, merkle.Sha256MerkleTreeRoot(action.Data))
		}
	}
	return leafHashes
}
