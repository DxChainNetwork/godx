package vm

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

func TestSplitStorageContractID(t *testing.T) {
	wantedHeight := uint64(123)
	wantedStorageContractID := common.HexToHash("0x51da85b8a745b0e2cf3bcd4cae108ad42f0dac49124419736e1bac49c2d44cd7")

	keyBytes, err := makeKey(PrefixExpireStorageContract+"123-", wantedStorageContractID)
	if err != nil {
		t.Errorf("failed to make key: %v", err)
	}

	splitHeight, splitSCID := SplitStorageContractID(keyBytes)

	if wantedHeight != splitHeight {
		t.Errorf("wanted %d , getted %d", wantedHeight, splitHeight)
	}

	if wantedStorageContractID != splitSCID {
		t.Errorf("wanted %s , getted %s", common.Hash(wantedStorageContractID).Hex(), common.Hash(splitSCID).Hex())
	}
}

func TestStorageContractDB(t *testing.T) {
	db := ethdb.NewMemDatabase()

	sc := types.StorageContract{
		FileSize:       2048,
		FileMerkleRoot: common.HexToHash("0x51da85b8a745b0e2cf3bcd4cae108ad42f0dac49124419736e1bac49c2d44cd7"),
		WindowStart:    uint64(234),
		WindowEnd:      uint64(345),
		RenterCollateral: types.DxcoinCollateral{
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
		UnlockHash:     common.HexToHash("0x000000000000000000000000000000000000000A"),
		RevisionNumber: 111,
	}

	scID := sc.ID()

	// store
	err := StoreStorageContract(db, scID, sc)
	if err != nil {
		t.Errorf("failed to store storage contract: %v", err)
	}

	// query
	gettedSC, err := GetStorageContract(db, scID)
	if err != nil {
		t.Errorf("failed to query storage contract: %v", err)
	}

	if gettedSC.RLPHash() != sc.RLPHash() {
		t.Errorf("wanted %s , getted %s", sc.RLPHash().Hex(), gettedSC.RLPHash().Hex())
	}

	// delete
	err = DeleteStorageContract(db, scID)
	if err != nil {
		t.Errorf("failed to delete storage contract: %v", err)
	}

}
