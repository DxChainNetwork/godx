package storagehost

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/rpc"
)

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

type mockHostBackend struct{}

func mockBlockHeader(number uint64) *types.Header {
	return &types.Header{
		ParentHash: common.HexToHash("abcdef"),
		UncleHash:  types.CalcUncleHash(nil),
		Coinbase:   common.HexToAddress("01238abcdd"),
		Root:       crypto.Keccak256Hash([]byte("1")),
		TxHash:     crypto.Keccak256Hash([]byte("11")),
		Bloom:      types.BytesToBloom(nil),
		Difficulty: big.NewInt(10000000),
		Number:     new(big.Int).SetUint64(number),
		GasLimit:   uint64(5000),
		GasUsed:    uint64(300),
		Time:       big.NewInt(1550103878),
		Extra:      []byte{},
		MixDigest:  crypto.Keccak256Hash(nil),
		Nonce:      types.EncodeNonce(uint64(1)),
	}
}

func (m *mockHostBackend) GetBlockByHash(blockHash common.Hash) (*types.Block, error) {
	switch blockHash {
	case common.Hash{1}:
		return types.NewBlock(
			mockBlockHeader(1),
			types.Transactions{
				types.NewContractCreation(
					0,
					new(big.Int).SetInt64(1),
					0,
					new(big.Int).SetInt64(1),
					nil,
				),
			},
			nil,
			nil), nil
	case common.Hash{2}:
		scRlp, err := rlp.EncodeToBytes(sc)
		if err != nil {
			return nil, err
		}
		return types.NewBlock(
			mockBlockHeader(2),
			types.Transactions{
				types.NewTransaction(
					0,
					common.BytesToAddress([]byte{10}),
					new(big.Int).SetInt64(1),
					0,
					new(big.Int).SetInt64(1),
					scRlp),
			},
			nil,
			nil), nil
	case common.Hash{3}:
		scrRlp, err := rlp.EncodeToBytes(scr)
		if err != nil {
			return nil, err
		}
		return types.NewBlock(
			mockBlockHeader(3),
			types.Transactions{
				types.NewTransaction(
					0,
					common.BytesToAddress([]byte{11}),
					new(big.Int).SetInt64(1),
					0,
					new(big.Int).SetInt64(1),
					scrRlp),
			},
			nil,
			nil), nil
	case common.Hash{4}:
		spfRlp, err := rlp.EncodeToBytes(spf)
		if err != nil {
			return nil, err
		}
		return types.NewBlock(
			mockBlockHeader(4),
			types.Transactions{
				types.NewTransaction(
					0,
					common.BytesToAddress([]byte{12}),
					new(big.Int).SetInt64(1),
					0,
					new(big.Int).SetInt64(1),
					spfRlp),
			},
			nil,
			nil), nil
	}
	return nil, nil
}

func (m *mockHostBackend) GetBlockByNumber(number uint64) (*types.Block, error) { return nil, nil }
func (m *mockHostBackend) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}
func (m *mockHostBackend) GetBlockChain() *core.BlockChain               { return nil }
func (m *mockHostBackend) AccountManager() *accounts.Manager             { return nil }
func (m *mockHostBackend) SetStatic(node *enode.Node)                    {}
func (m *mockHostBackend) CheckAndUpdateConnection(peerNode *enode.Node) {}
func (m *mockHostBackend) APIs() []rpc.API                               { return nil }

func TestGetAllStorageContractIDsWithBlockHash(t *testing.T) {
	host := &StorageHost{}
	host.ethBackend = &mockHostBackend{}

	tests := []struct {
		host                 *StorageHost
		blockHash            common.Hash
		expectError          error
		expectNumber         uint64
		expectHash           common.Hash
		expectRevisionNumber uint64
	}{
		{
			host:         host,
			blockHash:    common.Hash{1},
			expectError:  nil,
			expectNumber: 1,
			expectHash:   common.Hash{1},
		},
		{
			host:         host,
			blockHash:    common.Hash{2},
			expectError:  nil,
			expectNumber: 2,
			expectHash:   sc.RLPHash(),
		},
		{
			host:                 host,
			blockHash:            common.Hash{3},
			expectError:          nil,
			expectNumber:         3,
			expectHash:           scr.ParentID,
			expectRevisionNumber: scr.NewRevisionNumber,
		},
		{
			host:         host,
			blockHash:    common.Hash{4},
			expectError:  nil,
			expectNumber: 4,
			expectHash:   spf.ParentID,
		},
	}

	for _, test := range tests {
		cc, cr, sp, number, err := test.host.getAllStorageContractIDsWithBlockHash(test.blockHash)
		if err != test.expectError {
			t.Fatal("the function getAllStorageContractIDsWithBlockHash error:", err)
		}
		if number != test.expectNumber {
			t.Error("the block number error:", number)
		}

		if len(cc) != 0 && cc[0] != test.expectHash {
			t.Error("the storage contract error:", cc)
		}
		if len(cr) != 0 && cr[test.expectHash] != test.expectRevisionNumber {
			t.Error("the storage contract revision error:", cr)
		}
		if len(sp) != 0 && sp[0] != test.expectHash {
			t.Error("the storage proof error:", sp)
		}
	}

}
