// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/merkletree"
	"io"
	"math/big"
	"sort"
)

const (
	UploadActionAppend = "Append"
	UploadActionTrim   = "Trim"
	UploadActionSwap   = "Swap"
	UploadActionUpdate = "Update"
)

const (

	// the amount of time that host provide host settings
	HostSettingTime = 60 * time.Second

	// the amount of time that the client and host have to negotiate a download request batch.
	DownloadTime = 600 * time.Second

	// the minimum amount of time that the client and host have to negotiate a storage contract revision.
	ContractRevisionTime = 600 * time.Second

	// the amount of time that the client and host have to negotiate a new storage contract.
	FormContractTime = 360 * time.Second
)

type (
	// Structure about 'Storage Create' protocol
	// ContractCreateRequest contains storage contract info and client pk
	ContractCreateRequest struct {
		StorageContract types.StorageContract
		Sign            []byte
	}

	// UploadRequest contains the request parameters for RPCUpload.
	UploadRequest struct {
		StorageContractID common.Hash
		Actions           []UploadAction

		NewRevisionNumber    uint64
		NewValidProofValues  []*big.Int
		NewMissedProofValues []*big.Int
	}

	// UploadAction is a generic Write action. The meaning of each field
	// depends on the Type of the action.
	UploadAction struct {
		Type string
		A, B uint64
		Data []byte
	}

	// UploadMerkleProof contains the optional Merkle proof for response data
	// for RPCUpload.
	UploadMerkleProof struct {
		OldSubtreeHashes []common.Hash
		OldLeafHashes    []common.Hash
		NewMerkleRoot    common.Hash
	}

	// DownloadRequest contains the request parameters for RPCDownload.
	DownloadRequest struct {
		StorageContractID common.Hash
		Sections          []DownloadRequestSection
		MerkleProof       bool

		NewRevisionNumber    uint64
		NewValidProofValues  []*big.Int
		NewMissedProofValues []*big.Int
		Signature            []byte
	}

	// DownloadRequestSection is a section requested in DownloadRequest.
	DownloadRequestSection struct {
		MerkleRoot [32]byte
		Offset     uint32
		Length     uint32
	}

	// DownloadResponse contains the response data for RPCDownload.
	DownloadResponse struct {
		Signature   []byte
		Data        []byte
		MerkleProof []common.Hash
	}
)

func (ccr *ContractCreateRequest) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	if err := s.Decode(&ccr); err != nil {
		return err
	}
	log.Debug("rlp decode form contract request", "encode_size", rlp.ListSize(size))
	return nil
}

func (ccr *ContractCreateRequest) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, ccr)
}

// newRevision creates a copy of current with its revision number incremented,
// and with cost transferred from the renter to the host.
func newRevision(current types.StorageContractRevision, cost *big.Int) types.StorageContractRevision {
	rev := current

	// need to manually copy slice memory
	rev.NewValidProofOutputs = make([]types.DxcoinCharge, 2)
	rev.NewMissedProofOutputs = make([]types.DxcoinCharge, 3)
	copy(rev.NewValidProofOutputs, current.NewValidProofOutputs)
	copy(rev.NewMissedProofOutputs, current.NewMissedProofOutputs)

	// move valid payout from renter to host
	rev.NewValidProofOutputs[0].Value = current.NewValidProofOutputs[0].Value.Sub(current.NewValidProofOutputs[0].Value, cost)
	rev.NewValidProofOutputs[1].Value = current.NewValidProofOutputs[1].Value.Add(current.NewValidProofOutputs[1].Value, cost)

	// move missed payout from renter to void
	rev.NewMissedProofOutputs[0].Value = current.NewMissedProofOutputs[0].Value.Sub(current.NewMissedProofOutputs[0].Value, cost)
	rev.NewMissedProofOutputs[2].Value = current.NewMissedProofOutputs[2].Value.Add(current.NewMissedProofOutputs[2].Value, cost)

	// increment revision number
	rev.NewRevisionNumber++

	return rev
}

// newDownloadRevision revises the current revision to cover the cost of
// downloading data.
func NewDownloadRevision(current types.StorageContractRevision, downloadCost *big.Int) types.StorageContractRevision {
	return newRevision(current, downloadCost)
}

// calculateProofRanges returns the proof ranges that should be used to verify a
// pre-modification Merkle diff proof for the specified actions.
func CalculateProofRanges(actions []UploadAction, oldNumSectors uint64) []merkletree.LeafRange {
	newNumSectors := oldNumSectors
	sectorsChanged := make(map[uint64]struct{})
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			sectorsChanged[newNumSectors] = struct{}{}
			newNumSectors++
		}
	}

	oldRanges := make([]merkletree.LeafRange, 0, len(sectorsChanged))
	for index := range sectorsChanged {
		if index < oldNumSectors {
			oldRanges = append(oldRanges, merkletree.LeafRange{
				Start: index,
				End:   index + 1,
			})
		}
	}
	sort.Slice(oldRanges, func(i, j int) bool {
		return oldRanges[i].Start < oldRanges[j].Start
	})

	return oldRanges
}

// modifyProofRanges modifies the proof ranges produced by calculateProofRanges
// to verify a post-modification Merkle diff proof for the specified actions.
func ModifyProofRanges(proofRanges []merkletree.LeafRange, actions []UploadAction, numSectors uint64) []merkletree.LeafRange {
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			proofRanges = append(proofRanges, merkletree.LeafRange{
				Start: numSectors,
				End:   numSectors + 1,
			})
			numSectors++
		}
	}
	return proofRanges
}

// modifyLeaves modifies the leaf hashes of a Merkle diff proof to verify a
// post-modification Merkle diff proof for the specified actions.
func ModifyLeaves(leafHashes []common.Hash, actions []UploadAction, numSectors uint64) []common.Hash {
	// determine which sector index corresponds to each leaf hash
	var indices []uint64
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			indices = append(indices, numSectors)
			numSectors++
		}
	}
	sort.Slice(indices, func(i, j int) bool {
		return indices[i] < indices[j]
	})
	indexMap := make(map[uint64]int, len(leafHashes))
	for i, index := range indices {
		if i > 0 && index == indices[i-1] {
			continue // remove duplicates
		}
		indexMap[index] = i
	}

	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			leafHashes = append(leafHashes, merkle.Root(action.Data))
		}
	}
	return leafHashes
}
