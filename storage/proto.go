// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

const (
	UploadActionAppend = "Append"
	UploadActionTrim   = "Trim"
	UploadActionSwap   = "Swap"
	UploadActionUpdate = "Update"
)

type (
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
