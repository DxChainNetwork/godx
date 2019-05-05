// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

// TODO: 文件合约签名
func SignStorageContract() ([]types.Signature, error) {
	return nil, nil
}

// TODO: 从文件合约回复公钥
func RecoverPubkeyFromSignature(signature types.Signature) (ecdsa.PublicKey, error) {
	return ecdsa.PublicKey{}, nil
}

// TODO: 文件合约验签，true表示验签通过
func VerifyStorageContractSignatures(pubkey, hash, signature []byte) bool {
	return true
}

// TODO: 从文件合约回复公钥
func RecoverAddrFromSignature(signature []types.Signature) (common.Address, common.Address, error) {
	return common.Address{}, common.Address{}, nil
}
