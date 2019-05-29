// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

const (
	UploadActionAppend = "Append"
	UploadActionTrim   = "Trim"
	UploadActionSwap   = "Swap"
	UploadActionUpdate = "Update"
)

const (

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
