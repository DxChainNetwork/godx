package storage

import (
	"github.com/DxChainNetwork/godx/common"
	"math/big"
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
)
