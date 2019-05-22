package storage

import (
	"crypto/ecdsa"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
	"math/big"
)

const (
	UploadActionAppend = "Append"
	UploadActionTrim   = "Trim"
	UploadActionSwap   = "Swap"
	UploadActionUpdate = "Update"
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
