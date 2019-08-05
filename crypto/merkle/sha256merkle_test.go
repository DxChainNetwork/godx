// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package merkle

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

func TestLeavesCount(t *testing.T) {
	tables := []struct {
		dataSize uint64
		count    uint64
	}{
		{64 * 2, 2},
		{64 * 1, 1},
		{64 * 0, 0},
		{63, 1},
		{65, 2},
	}

	for _, table := range tables {
		result := LeavesCount(table.dataSize)
		if result != table.count {
			t.Errorf("wrong leaves count obtained. Expected: %v, got: %v",
				table.count, result)
		}
	}
}

func TestRangeVerification(t *testing.T) {
	tables := []struct {
		start int
		end   int
		err   bool
	}{
		{1, 2, false},
		{2, 1, true},
		{1, 1, true},
		{-1, 1, true},
	}

	for _, table := range tables {
		validErr := rangeVerification(table.start, table.end)
		if validErr == nil && table.err {
			t.Errorf("error is expected, howoever, no error is detected")
		}

		if validErr != nil && !table.err {
			t.Errorf("error is detected but not expected: %s", validErr.Error())
		}
	}
}

func TestMerkleRootProofVerification(t *testing.T) {
	var proofIndex uint64
	data := []byte("Good Morning")

	// create merkle root
	mr := Sha256MerkleTreeRoot(data)

	// create merkle proof
	proofDataPiece, proofSet, numLeaves, err := Sha256MerkleTreeProof(data, proofIndex)
	if err != nil {
		t.Fatalf("failed to get the merkle proof: %s", err.Error())
	}

	// verification
	verified := Sha256VerifyDataPiece(proofDataPiece, proofSet, numLeaves, proofIndex, mr)
	if !verified {
		t.Errorf("expected merkle verification to be succeed, instead, got failed")
	}

	// randomly generate a merkle root, and do a test
	randmr := randomMerkleRoot()
	verified = Sha256VerifyDataPiece(proofDataPiece, proofSet, numLeaves, proofIndex, randmr)
	if verified {
		t.Error("expected merkle verification to be failed, instead, got succeed")
	}

	// change the proofDataPiece
	proofDataPiece[0] = 'd'
	verified = Sha256VerifyDataPiece(proofDataPiece, proofSet, numLeaves, proofIndex, mr)
	if verified {
		t.Error("expected merkle verification to be failed, instead, got succeed")
	}
}

func TestMerkleRangeProofVerification(t *testing.T) {
	for piece := 0; piece < 50; piece++ {
		data := randomDataGenerator(uint64(piece * LeafSize))
		mr := Sha256MerkleTreeRoot(data)
		for startProof := 0; startProof < piece; startProof++ {
			for endProof := startProof + 1; endProof < piece-1; endProof++ {
				proofSet, err := Sha256RangeProof(data, startProof, endProof)
				if err != nil {
					t.Fatalf("failed to get merkle range proof set")
				}

				verified, err := Sha256VerifyRangeProof(data[startProof*LeafSize:endProof*LeafSize], proofSet, startProof, endProof, mr)
				if err != nil {
					t.Errorf("failed to obtain the range proof: %s", err.Error())
				}

				if !verified {
					t.Errorf("expected verified, but not")
				}
			}
		}
	}
}

func TestMerkleSectorRangeProofVerification(t *testing.T) {
	for piece := 0; piece < 50; piece++ {
		roots := randomHashSliceGenerator(piece)

		mr := Sha256CachedTreeRoot(roots, sectorHeight)

		for startProof := 0; startProof < piece; startProof++ {
			for endProof := startProof + 1; endProof < piece-1; endProof++ {
				proofSet, err := Sha256SectorRangeProof(roots, startProof, endProof)
				if err != nil {
					t.Fatalf("failed to get the merkle sector range proof set")
				}

				verified, err := Sha256VerifySectorRangeProof(roots[startProof:endProof], proofSet, startProof, endProof, mr)
				if err != nil {
					t.Fatalf("failed to verify the sector range proof: %s", err.Error())
				}
				if !verified {
					t.Errorf("expected verified, but not")
				}
			}
		}
	}
}

func TestMerkleDiffProofVerification(t *testing.T) {
	roots := randomHashSliceGenerator(50)
	mr := Sha256CachedTreeRoot(roots, sectorHeight)
	rangeSet := []SubTreeLimit{
		{Left: 1, Right: 2},
		{Left: 10, Right: 20},
		{Left: 30, Right: 50},
	}

	// create proofSet
	proofSet, err := Sha256DiffProof(roots, rangeSet, uint64(50))
	if err != nil {
		t.Fatalf("failed to create merkleDiffProof: %s", err.Error())
	}

	// construct rootVerify
	var rootVerify []common.Hash
	sections := [][]common.Hash{
		roots[1:2],
		roots[10:20],
		roots[30:50],
	}

	for _, sec := range sections {
		rootVerify = append(rootVerify, sec...)
	}

	// verification
	err = Sha256VerifyDiffProof(rangeSet, uint64(50), proofSet, rootVerify, mr)
	if err != nil {
		t.Fatalf("failed to verify the diff proof: %s", err.Error())
	}


}

/*
 _____  _____  _______      __  _______ ______      ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|    |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__       | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____     | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|    |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

const (
	sectorSize = uint64(1 << 22)
)

var sectorHeight = func() uint64 {
	height := uint64(0)
	for 1<<height < (sectorSize / LeafSize) {
		height++
	}
	return height
}()

func merkleLeaves(data []byte) (leaves [][]byte) {
	// length of the data pieces should be equivalent to the number of leaves of the merkle tree
	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		leaves = append(leaves, buf.Next(LeafSize))
	}

	return
}

func randomHashSliceGenerator(length int) (hs []common.Hash) {
	for i := 0; i < length; i++ {
		hs = append(hs, randomHash())
	}
	return
}

func randomHash() (h common.Hash) {
	rand.Seed(time.Now().UnixNano())
	rand.Read(h[:])
	return
}

func randomMerkleRoot() (mr common.Hash) {
	rand.Seed(time.Now().UnixNano())
	rand.Read(mr[:])
	return
}

func randomIndex(max int) (index int) {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max)
}

func randomDataGenerator(length uint64) (data []byte) {
	data = make([]byte, length)
	rand.Seed(time.Now().UnixNano())
	rand.Read(data)
	return
}
