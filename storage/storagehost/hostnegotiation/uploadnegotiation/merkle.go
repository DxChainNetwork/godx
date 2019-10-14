// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"fmt"
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

func constructUploadMerkleProof(session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility) (storage.UploadMerkleProof, error) {
	proofRanges := calcAndSortProofRanges(sr, *session)
	leafHashes := calcLeafHashes(proofRanges, sr)
	oldHashSet, err := calcOldHashSet(sr, proofRanges)
	if err != nil {
		err = fmt.Errorf("failed to construct upload merkle proof: %s", err.Error())
		return storage.UploadMerkleProof{}, err
	}

	// construct the merkle proof
	merkleProof := storage.UploadMerkleProof{
		OldSubtreeHashes: oldHashSet,
		OldLeafHashes:    leafHashes,
		NewMerkleRoot:    session.NewMerkleRoot,
	}

	// update uploadNegotiationData for merkle proof
	session.MerkleProof = merkleProof
	return merkleProof, nil
}

func calcAndSortProofRanges(sr storagehost.StorageResponsibility, session hostnegotiation.UploadSession) []merkle.SubTreeLimit {
	// calculate proof ranges
	oldNumSectors := uint64(len(sr.SectorRoots))
	var proofRanges []merkle.SubTreeLimit
	for i := range session.SectorsCount {
		if i < oldNumSectors {
			proofRange := merkle.SubTreeLimit{
				Left:  i,
				Right: i + 1,
			}

			proofRanges = append(proofRanges, proofRange)
		}
	}

	// sort proof ranges
	sort.Slice(proofRanges, func(i, j int) bool {
		return proofRanges[i].Left < proofRanges[j].Left
	})

	return proofRanges
}

func calcLeafHashes(proofRanges []merkle.SubTreeLimit, sr storagehost.StorageResponsibility) []common.Hash {
	var leafHashes []common.Hash
	for _, proofRange := range proofRanges {
		leafHashes = append(leafHashes, sr.SectorRoots[proofRange.Left])
	}

	return leafHashes
}

func calcOldHashSet(sr storagehost.StorageResponsibility, proofRanges []merkle.SubTreeLimit) ([]common.Hash, error) {
	return merkle.Sha256DiffProof(sr.SectorRoots, proofRanges, uint64(len(sr.SectorRoots)))
}
