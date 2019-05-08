package vm

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
)

func TestSignStorageContract(t *testing.T) {
	prvKey, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs: %v", err)
	}

	sc := types.StorageContract{
		FileSize:       2048,
		FileMerkleRoot: common.HexToHash("0x51da85b8a745b0e2cf3bcd4cae108ad42f0dac49124419736e1bac49c2d44cd7"),
		WindowStart:    types.BlockHeight(234),
		WindowEnd:      types.BlockHeight(345),
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
		UnlockHash:     types.UnlockHash(common.HexToHash("0x000000000000000000000000000000000000000A")),
		RevisionNumber: 111,
	}

	scHash := sc.RLPHash()

	// sign storage contract
	sig, err := SignStorageContract(sc, prvKey)
	if err != nil {
		t.Errorf("failed to sign storage contract: %v", err)
	}

	if sig == nil {
		t.Error("no signature")
	}

	if len(sig) != 65 {
		t.Errorf("wrong signatures, wanted 65 bytes, getted %d bytes", len(sig))
	}

	// recover pubkey
	pubkey, err := RecoverPubkeyFromSignature(scHash, sig)
	if err != nil {
		t.Errorf("failed to recover pubkey from storage contract signature: %v", err)
	}

	// verify signature
	pubkeyBytes := crypto.FromECDSAPub(&pubkey)
	if !VerifyStorageContractSignatures(pubkeyBytes, scHash[:], sig) {
		t.Errorf("failed to verify storage contract signature")
	}
}
