// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"crypto/ecdsa"
	"io"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	UploadActionAppend = "Append"
	UploadActionTrim   = "Trim"
	UploadActionSwap   = "Swap"
	UploadActionUpdate = "Update"
)

const (
	// the amount of time that the client and host have to negotiate a download request batch.
	// the time is set high enough that two nodes can complete the negotiation.
	DownloadTime = 600 * time.Second

	// the minimum amount of time that the client and host have to negotiate a storage contract revision.
	// the time is set high enough that a full 4MB can be piped through a connection.
	ContractRevisionTime = 600 * time.Second

	// the amount of time that the client and host have to negotiate a new storage contract.
	// the time is set high enough that a node can make the multiple required round trips to complete the negotiation.
	FormContractTime = 360 * time.Second
)

type (
	// Structure about 'Storage Create' protocol
	// ContractCreateRequest contains storage contract info and client pk
	ContractCreateRequest struct {
		StorageContract types.StorageContract
		ClientPK        ecdsa.PublicKey
	}

	ContractCreateSignature struct {
		ContractSign []byte
		RevisionSign []byte
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
