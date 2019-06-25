// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package types

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
)

type StorageContractRLPHash interface {
	RLPHash() common.Hash
}

type HostAnnouncement struct {
	// host enode url
	NetAddress string
	Signature  []byte
}

type UnlockConditions struct {
	PaymentAddresses   []common.Address `json:"paymentaddress"`
	SignaturesRequired uint64           `json:"signaturesrequired"`
}

type DxcoinCharge struct {
	Address common.Address
	Value   *big.Int
}

type DxcoinCollateral struct {
	DxcoinCharge
}

type StorageContract struct {
	// file part
	FileSize       uint64      `json:"filesize"`
	FileMerkleRoot common.Hash `json:"filemerkleroot"`
	WindowStart    uint64      `json:"windowstart"`
	WindowEnd      uint64      `json:"windowend"`

	// money part
	// original collateral
	ClientCollateral DxcoinCollateral `json:"client_collateral"`
	HostCollateral   DxcoinCollateral `json:"host_collateral"`

	// temporary book while file upload and download
	ValidProofOutputs  []DxcoinCharge `json:"validproofoutputs"`
	MissedProofOutputs []DxcoinCharge `json:"missedproofoutputs"`

	// lock the client and host for this storage contract
	UnlockHash     common.Hash `json:"unlockhash"`
	RevisionNumber uint64      `json:"revisionnumber"`
	Signatures     [][]byte
}

type StorageContractRevision struct {
	ParentID              common.Hash      `json:"parentid"`
	UnlockConditions      UnlockConditions `json:"unlockconditions"`
	NewRevisionNumber     uint64           `json:"newrevisionnumber"`
	NewFileSize           uint64           `json:"newfilesize"`
	NewFileMerkleRoot     common.Hash      `json:"newfilemerkleroot"`
	NewWindowStart        uint64           `json:"newwindowstart"`
	NewWindowEnd          uint64           `json:"newwindowend"`
	NewValidProofOutputs  []DxcoinCharge   `json:"newvalidproofoutputs"`
	NewMissedProofOutputs []DxcoinCharge   `json:"newmissedproofoutputs"`
	NewUnlockHash         common.Hash      `json:"newunlockhash"`
	Signatures            [][]byte
}

type StorageProof struct {
	ParentID  common.Hash   `json:"parentid"`
	Segment   [64]byte      `json:"segment"`
	HashSet   []common.Hash `json:"hashset"`
	Signature []byte
}

// RLPHash calculate the hash of HostAnnouncement
func (ha HostAnnouncement) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		ha.NetAddress,
	})
}

// RLPHash calculate the hash of StorageContract
func (sc StorageContract) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		sc.FileSize,
		sc.FileMerkleRoot,
		sc.WindowStart,
		sc.WindowEnd,
		sc.ClientCollateral,
		sc.HostCollateral,
		sc.ValidProofOutputs,
		sc.MissedProofOutputs,
		sc.RevisionNumber,
	})
}

// ID calculate the ID of StorageContract
func (sc StorageContract) ID() common.Hash {
	return common.Hash(sc.RLPHash())
}

// UnlockHash calculate the hash of UnlockCondition
func (uc UnlockConditions) UnlockHash() common.Hash {
	return rlpHash([]interface{}{
		uc.PaymentAddresses,
		uc.SignaturesRequired,
	})
}

// RLPHash calculate the hash of StorageContractRevision
func (scr StorageContractRevision) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		scr.ParentID,
		scr.UnlockConditions,
		scr.NewRevisionNumber,
		scr.NewFileSize,
		scr.NewFileMerkleRoot,
		scr.NewWindowStart,
		scr.NewWindowEnd,
		scr.NewValidProofOutputs,
		scr.NewMissedProofOutputs,
	})
}

// RLPHash calculate the hash of StorageProof
func (sp StorageProof) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		sp.ParentID,
		sp.Segment,
		sp.HashSet,
	})
}
