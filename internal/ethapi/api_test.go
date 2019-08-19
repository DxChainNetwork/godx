package ethapi

import (
	"math/big"
	"net"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
)

func TestBlockToStorageContract(t *testing.T) {
	txs := make([]*types.Transaction, 0)
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{10}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), []byte("contractCreate")))
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{11}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), []byte("CommitRevision")))
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{12}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), []byte("StorageProof")))
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{9}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), []byte("HostAnnounce")))
	hearder := &types.Header{
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
	block := types.NewBlock(hearder, txs, nil, nil)
	_, err := blockToStorageContract(block)
	if err != nil {
		t.Error(err)
	}
}

func TestTransactionToStorageContract(t *testing.T) {
	txs := make([]*types.Transaction, 0)

	prvKeyHost, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage host: %v", err)
	}

	prvKeyClient, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage client: %v", err)
	}
	uc := types.UnlockConditions{
		PaymentAddresses:   []common.Address{crypto.PubkeyToAddress(prvKeyClient.PublicKey), crypto.PubkeyToAddress(prvKeyHost.PublicKey)},
		SignaturesRequired: 2,
	}
	sc := types.StorageContract{
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
		UnlockHash:     uc.UnlockHash(),
		RevisionNumber: 111,
	}
	scRlp, err := rlp.EncodeToBytes(sc)
	if err != nil {
		t.Error("StorageContract rlp err:", err)
	}
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{10}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), scRlp))

	scr := types.StorageContractRevision{
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
	scrRlp, err := rlp.EncodeToBytes(scr)
	if err != nil {
		t.Error("StorageContractRevision rlp err:", err)
	}
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{11}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), scrRlp))

	spf := types.StorageProof{
		ParentID: sc.RLPHash(),
		Segment:  [64]byte{},
		HashSet: []common.Hash{
			common.HexToHash("0000000001"),
			common.HexToHash("0000000002"),
		},
		Signature: []byte("0x14564645456"),
	}
	spfRlp, err := rlp.EncodeToBytes(spf)
	if err != nil {
		t.Error("StorageProof rlp err:", err)
	}
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{12}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), spfRlp))

	hostNode := enode.NewV4(&prvKeyHost.PublicKey, net.IP{127, 0, 0, 1}, int(8888), int(8888))
	// test host announce signature(only one signature)
	ha := types.HostAnnouncement{
		NetAddress: hostNode.String(),
		Signature:  []byte("0x78469416"),
	}
	haRlp, err := rlp.EncodeToBytes(ha)
	if err != nil {
		t.Error("HostAnnouncement rlp err:", err)
	}
	txs = append(txs, types.NewTransaction(0, common.BytesToAddress([]byte{9}), new(big.Int).SetInt64(1), 0, new(big.Int).SetInt64(1), haRlp))

	for _, tx := range txs {
		_, err := transactionToStorageContract(tx)
		if err != nil {
			t.Error("transactionToStorageContract err:", err)
		}
	}

}
