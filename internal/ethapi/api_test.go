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
		expectLength int
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
						[]byte("contractCreate1")),
					types.NewTransaction(
						1,
						common.BytesToAddress([]byte{10}),
						new(big.Int).SetInt64(2),
						0,
						new(big.Int).SetInt64(2),
						[]byte("contractCreate2")),
					types.NewTransaction(
						2,
						common.BytesToAddress([]byte{10}),
						new(big.Int).SetInt64(3),
						0,
						new(big.Int).SetInt64(3),
						[]byte("contractCreate3")),
				},
				nil,
				nil),
			expectOutput: vm.ContractCreateTransaction,
			expectLength: 3,
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
						[]byte("CommitRevision1")),
					types.NewTransaction(
						1,
						common.BytesToAddress([]byte{11}),
						new(big.Int).SetInt64(2),
						0,
						new(big.Int).SetInt64(2),
						[]byte("CommitRevision2")),
				},
				nil,
				nil),
			expectOutput: vm.CommitRevisionTransaction,
			expectLength: 2,
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
						[]byte("StorageProof1")),
					types.NewTransaction(
						1,
						common.BytesToAddress([]byte{12}),
						new(big.Int).SetInt64(2),
						0,
						new(big.Int).SetInt64(2),
						[]byte("StorageProof2")),
					types.NewTransaction(
						2,
						common.BytesToAddress([]byte{12}),
						new(big.Int).SetInt64(3),
						0,
						new(big.Int).SetInt64(3),
						[]byte("StorageProof3")),
					types.NewTransaction(
						3,
						common.BytesToAddress([]byte{12}),
						new(big.Int).SetInt64(4),
						0,
						new(big.Int).SetInt64(4),
						[]byte("StorageProof4")),
				},
				nil,
				nil),
			expectOutput: vm.StorageProofTransaction,
			expectLength: 4,
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
						[]byte("HostAnnounce1")),
					types.NewTransaction(
						1,
						common.BytesToAddress([]byte{9}),
						new(big.Int).SetInt64(2),
						0,
						new(big.Int).SetInt64(2),
						[]byte("HostAnnounce2")),
					types.NewTransaction(
						2,
						common.BytesToAddress([]byte{9}),
						new(big.Int).SetInt64(3),
						0,
						new(big.Int).SetInt64(3),
						[]byte("HostAnnounce3")),
					types.NewTransaction(
						3,
						common.BytesToAddress([]byte{9}),
						new(big.Int).SetInt64(4),
						0,
						new(big.Int).SetInt64(4),
						[]byte("HostAnnounce4")),
					types.NewTransaction(
						4,
						common.BytesToAddress([]byte{9}),
						new(big.Int).SetInt64(5),
						0,
						new(big.Int).SetInt64(5),
						[]byte("HostAnnounce5")),
				},
				nil,
				nil),
			expectOutput: vm.HostAnnounceTransaction,
			expectLength: 5,
		},
		{
			block: types.NewBlock(
				header,
				types.Transactions{
					types.NewContractCreation(
						0,
						new(big.Int).SetInt64(1),
						0,
						new(big.Int).SetInt64(1),
						nil,
					),
					types.NewContractCreation(
						1,
						new(big.Int).SetInt64(2),
						0,
						new(big.Int).SetInt64(2),
						nil,
					),
					types.NewContractCreation(
						2,
						new(big.Int).SetInt64(3),
						0,
						new(big.Int).SetInt64(3),
						nil,
					),
				},
				nil,
				nil),
			expectLength: 0,
		},
	}

	for _, test := range tests {
		result, err := blockToStorageContract(test.block)
		if err != nil {
			t.Error(err)
			return
		}

		if len(result) != test.expectLength {
			t.Error("the expectLength error:", len(result))
		}

		if len(result) != 0 {
			for _, value := range result {
				if value != test.expectOutput {
					t.Error("the expectOutput error:", value)
				}
			}
		}

	}

}

func TestTransactionToStorageContractErr(t *testing.T) {
	tests := []struct {
		tx             *types.Transaction
		expectedOutput string
	}{
		{
			tx: types.NewTransaction(
				0,
				common.Address{},
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "not a storage contract related transaction",
		},
		{
			tx: types.NewContractCreation(
				1,
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "this is a deployment contract transaction",
		},
		{
			tx: types.NewTransaction(
				2,
				common.BytesToAddress([]byte{9}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "the data field in the transaction is decoded abnormally",
		},
		{
			tx: types.NewTransaction(
				3,
				common.BytesToAddress([]byte{10}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "the data field in the transaction is decoded abnormally",
		},
		{
			tx: types.NewTransaction(
				4,
				common.BytesToAddress([]byte{11}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "the data field in the transaction is decoded abnormally",
		},
		{
			tx: types.NewTransaction(
				5,
				common.BytesToAddress([]byte{12}),
				new(big.Int).SetInt64(1),
				0,
				new(big.Int).SetInt64(1),
				nil,
			),
			expectedOutput: "the data field in the transaction is decoded abnormally",
		},
	}

	for _, test := range tests {
		_, err := transactionToStorageContract(test.tx)
		if err.Error() != test.expectedOutput {
			t.Error(err)
			return
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
				1,
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
				2,
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
				3,
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
