// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"crypto/ecdsa"
	"errors"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
)

// sign storage contract data
func SignStorageContract(originData types.StorageContractRLPHash, prv *ecdsa.PrivateKey) (types.Signature, error) {

	// rlp hash
	hashData := originData.RLPHash()

	// 65 bytes signature
	sig, err := crypto.Sign(hashData[:], prv)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

// recover pubkey from storage contract signature
func RecoverPubkeyFromSignature(originHash common.Hash, signature types.Signature) (ecdsa.PublicKey, error) {
	pub, err := crypto.SigToPub(originHash.Bytes(), signature)
	if err != nil {
		return ecdsa.PublicKey{}, err
	}

	return *pub, nil
}

// verification for storage contract
func VerifyStorageContractSignatures(pubkey, hash, signature []byte) bool {
	return crypto.VerifySignature(pubkey, hash[:], signature[:len(signature)-1])
}

// recover addr of storage client or host from signature
func RecoverAddrFromSignature(originHash common.Hash, signature types.Signature) (common.Address, error) {
	pubkeyBytes, err := crypto.Ecrecover(originHash[:], signature)
	if err != nil {
		return common.Address{}, err
	}
	if len(pubkeyBytes) == 0 || pubkeyBytes[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}

	var addr common.Address
	copy(addr[:], crypto.Keccak256(pubkeyBytes[1:])[12:])
	return addr, nil
}
