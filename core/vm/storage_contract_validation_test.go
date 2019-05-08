package vm

import (
	"math/big"
	"testing"

	"github.com/magiconair/properties/assert"
	"golang.org/x/crypto/sha3"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
)

func TestCheckMultiSignatures(t *testing.T) {
	prvKeyHost, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage host: %v", err)
	}

	prvKeyClient, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs for storage client: %v", err)
	}

	// test host announce signature(only one signature)
	ha := types.HostAnnouncement{
		NetAddress: "127.0.0.1:8888",
	}

	sigHa, err := SignStorageContract(ha, prvKeyHost)
	if err != nil {
		t.Errorf("failed to sign host announce: %v", err)
	}

	currentHeight := types.BlockHeight(1001)

	err = CheckMultiSignatures(ha, currentHeight, []types.Signature{sigHa})
	if err != nil {
		t.Errorf("failed to check host announce signature: %v", err)
	}

	// test storage contract signature(two signatures)
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
		UnlockHash:     types.UnlockHash(common.HexToHash("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")),
		RevisionNumber: 111,
	}

	sigsScByHost, err := SignStorageContract(ha, prvKeyHost)
	if err != nil {
		t.Errorf("host failed to sign host announce: %v", err)
	}

	sigsScByClient, err := SignStorageContract(ha, prvKeyClient)
	if err != nil {
		t.Errorf("client failed to sign host announce: %v", err)
	}

	sc.Signatures = make([]types.Signature, 2)
	sc.Signatures[0] = sigsScByClient
	sc.Signatures[1] = sigsScByHost

	err = CheckMultiSignatures(sc, currentHeight, sc.Signatures)
	if err != nil {
		t.Errorf("failed to check storage contract signature: %v", err)
	}
}

var (
	leaveContent = []string{"jack", "lucy", "green", "apple"}
)

func mockMerkleProof(leaveContent []string) (common.Hash, []common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	// hash of leaves
	leaveHashes := make([][]byte, len(leaveContent))
	for i, lcontent := range leaveContent {
		leaveHashes[i] = HashSum(hasher, []byte(lcontent))
	}

	// caculate merkle root
	p1 := HashSum(hasher, leaveHashes[0], leaveHashes[1])
	p2 := HashSum(hasher, leaveHashes[2], leaveHashes[3])
	p0 := HashSum(hasher, p1, p2)
	hashSet := []common.Hash{common.BytesToHash(leaveHashes[1]), common.BytesToHash(p2)}
	return common.BytesToHash(p0), hashSet
}

func TestVerifySegment(t *testing.T) {
	root, hashSet := mockMerkleProof(leaveContent)

	// check "jack" merkle proof
	assert.Equal(t, VerifySegment([]byte("jack"), hashSet, 4, 0, root), true, "incorrect verification merkle proof")
	assert.Equal(t, VerifySegment([]byte("lucy"), hashSet, 4, 0, root), false, "incorrect verification merkle proof")
}
