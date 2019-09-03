// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/rlp"
)

var header = &types.Header{
	ParentHash: common.HexToHash("abcdef"),
	UncleHash:  types.CalcUncleHash(nil),
	Coinbase:   common.HexToAddress("01238abcdd"),
	Root:       crypto.Keccak256Hash([]byte("1")),
	TxHash:     crypto.Keccak256Hash([]byte("11")),
	Bloom:      types.BytesToBloom(nil),
	Difficulty: big.NewInt(10000000),
	Number:     big.NewInt(50),
	GasLimit:   uint64(5000),
	GasUsed:    uint64(300),
	Time:       big.NewInt(1550103878),
	Extra:      []byte{},
	MixDigest:  crypto.Keccak256Hash(nil),
	Nonce:      types.EncodeNonce(uint64(1)),
}

var sc = types.StorageContract{
	FileSize:       2048,
	FileMerkleRoot: common.HexToHash("0x51da85b8a745b0e2cf3bcd4cae108ad42f0dac49124419736e1bac49c2d44cd7"),
	WindowStart:    uint64(234),
	WindowEnd:      uint64(345),
	ClientCollateral: types.DxcoinCollateral{
		DxcoinCharge: types.DxcoinCharge{
			Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"),
			Value:   new(big.Int).SetInt64(10000),
		},
	},
	HostCollateral: types.DxcoinCollateral{
		DxcoinCharge: types.DxcoinCharge{
			Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"),
			Value:   new(big.Int).SetInt64(10000),
		},
	},
	ValidProofOutputs: []types.DxcoinCharge{
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"),
			Value: new(big.Int).SetInt64(10000)},
	},
	MissedProofOutputs: []types.DxcoinCharge{
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"),
			Value: new(big.Int).SetInt64(10000)},
	},
	UnlockHash:     common.HexToHash("0x51da85b8a745b0e2cf3bcd4cae108ad42f0dac49124419736e1bac49c2d44cd8"),
	RevisionNumber: 111,
}

var scr = types.StorageContractRevision{
	ParentID: sc.ID(),
	UnlockConditions: types.UnlockConditions{
		PaymentAddresses: []common.Address{
			sc.ClientCollateral.Address,
			sc.HostCollateral.Address,
		},
		SignaturesRequired: 2,
	},
	NewRevisionNumber: 2,
	NewFileSize:       1000,
	NewFileMerkleRoot: common.HexToHash("0x20198404b29fdc225c1ad7df48da3e16c08f8c9fb50c1768ce08baeba57b3bd7"),
	NewWindowStart:    sc.WindowStart,
	NewWindowEnd:      sc.WindowEnd,
	NewValidProofOutputs: []types.DxcoinCharge{
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"), Value: new(big.Int).SetInt64(1)},
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118E"), Value: new(big.Int).SetInt64(1)},
	},
	NewMissedProofOutputs: []types.DxcoinCharge{
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118D"), Value: new(big.Int).SetInt64(1)},
		{Address: common.HexToAddress("0xcf1FA0d741F155Bd2cF69A5a791C81BB8222118E"), Value: new(big.Int).SetInt64(1)},
	},
	NewUnlockHash: sc.UnlockHash,
}

var spf = types.StorageProof{
	ParentID: sc.RLPHash(),
	Segment:  [64]byte{},
	HashSet: []common.Hash{
		common.HexToHash("0000000001"),
		common.HexToHash("0000000002"),
	},
	Signature: []byte("0x14564645456"),
}

var ha = types.HostAnnouncement{
	NetAddress: "enode://0ec8f957266eb79c56fc422c28643119a0b7b9771f0cd1a3dc91dc1b865b29e25e3856703bd8fe040556c443cea2ff13fd5bf432adfa3445a3366e0eb9ae063d@127.0.0.1:30303",
	Signature:  []byte("0x78469416"),
}

func TestBlockToStorageContract(t *testing.T) {
	tests := []struct {
		block        *types.Block
		expectOutput string
	}{
		{
			block: types.NewBlock(
				header,
				types.Transactions{
					types.NewTransaction(
						0,
						common.BytesToAddress([]byte{10}),
						new(big.Int).SetInt64(1),
						0,
						new(big.Int).SetInt64(1),
						[]byte("contractCreate")),
				},
				nil,
				nil),
			expectOutput: vm.ContractCreateTransaction,
		},
		{
			block: types.NewBlock(
				header,
				types.Transactions{
					types.NewTransaction(
						0,
						common.BytesToAddress([]byte{11}),
						new(big.Int).SetInt64(1),
						0,
						new(big.Int).SetInt64(1),
						[]byte("CommitRevision")),
				},
				nil,
				nil),
			expectOutput: vm.CommitRevisionTransaction,
		},
		{
			block: types.NewBlock(
				header,
				types.Transactions{
					types.NewTransaction(
						0,
						common.BytesToAddress([]byte{12}),
						new(big.Int).SetInt64(1),
						0,
						new(big.Int).SetInt64(1),
						[]byte("StorageProof")),
				},
				nil,
				nil),
			expectOutput: vm.StorageProofTransaction,
		},
		{
			block: types.NewBlock(
				header,
				types.Transactions{
					types.NewTransaction(
						0,
						common.BytesToAddress([]byte{9}),
						new(big.Int).SetInt64(1),
						0,
						new(big.Int).SetInt64(1),
						[]byte("HostAnnounce")),
				},
				nil,
				nil),
			expectOutput: vm.HostAnnounceTransaction,
		},
	}

	for _, test := range tests {
		result, err := blockToStorageContract(test.block)
		if err != nil {
			t.Error(err)
			return
		}
		if result[test.block.Transactions()[0].Hash().String()] != test.expectOutput {
			t.Error("the returned result does not match the expectOutput value")
		}
	}

}

func TestTransactionToStorageContract(t *testing.T) {
	scRlp, err := rlp.EncodeToBytes(sc)
	if err != nil {
		t.Error("StorageContract rlp err:", err)
	}
	scrRlp, err := rlp.EncodeToBytes(scr)
	if err != nil {
		t.Error("StorageContractRevision rlp err:", err)
	}
	spfRlp, err := rlp.EncodeToBytes(spf)
	if err != nil {
		t.Error("StorageProof rlp err:", err)
	}
	haRlp, err := rlp.EncodeToBytes(ha)
	if err != nil {
		t.Error("HostAnnouncement rlp err:", err)
	}
	tests := []struct {
		tx              *types.Transaction
		expectedOutputs []string
	}{
		{
			tx: types.NewTransaction(
				0,
				common.BytesToAddress([]byte{10}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				scRlp),
			expectedOutputs: []string{
				vm.ContractCreateTransaction,
				"StorageContract",
			},
		},
		{
			tx: types.NewTransaction(
				0,
				common.BytesToAddress([]byte{11}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				scrRlp),
			expectedOutputs: []string{
				vm.CommitRevisionTransaction,
				"StorageContractRevision",
			},
		},
		{
			tx: types.NewTransaction(
				0,
				common.BytesToAddress([]byte{12}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				spfRlp),
			expectedOutputs: []string{
				vm.StorageProofTransaction,
				"StorageContractStorageProof",
			},
		},
		{
			tx: types.NewTransaction(
				0,
				common.BytesToAddress([]byte{9}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				haRlp),
			expectedOutputs: []string{
				vm.HostAnnounceTransaction,
				"HostAnnouncement",
			},
		},
	}

	for _, test := range tests {
		result, err := transactionToStorageContract(test.tx)
		if err != nil {
			t.Error(err)
			return
		}
		if result[test.tx.Hash().String()] != test.expectedOutputs[0] {
			t.Error("the returned result does not match the expectOutput[0] value")
			return
		}
		if _, ok := result[test.expectedOutputs[1]]; !ok {
			t.Error("the returned result does not match the expectOutput[1] value")
		}
	}
}
