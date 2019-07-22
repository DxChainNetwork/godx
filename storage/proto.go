// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"math/big"
)

// Defines upload mode
const (
	UploadActionAppend = "Append"
)

type (
	// ContractCreateRequest contains storage contract info and client pk
	ContractCreateRequest struct {
		StorageContract types.StorageContract
		Sign            []byte
		Renew           bool
		OldContractID   common.Hash
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
