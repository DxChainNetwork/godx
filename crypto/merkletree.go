// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package crypto

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/merkletree"
)

// MerkleLeafSize is the data size stored in one merkle leaf.
// the original data will be divided into pieces based on the
// merkleRootSize, and then be pushed into the merkle tree
const (
	MerkleLeafSize = 64
)

// MerkleTree serves as a wrapper of the merkle tree, provide a convenient way
// to access methods associated with the merkle tree
type MerkleTree struct {
	merkletree.Tree
}

// NewMerkleTree will initialize a MerkleTree object with sha256 as
// the hashing method
func NewMerkleTree() (mk *MerkleTree) {
	mk = &MerkleTree{
		Tree: *merkletree.New(sha256.New()),
	}
	return
}

// Root will calculate and return the merkle root of the merkle tree
func (mt *MerkleTree) Root() (h common.Hash) {
	copy(h[:], mt.Tree.Root())
	return
}

// CachedMerkleTree is similar to MerkleTree, one obvious difference between them
// is that cached merkle tree will not hash the data before inserting them into
// the tree
type CachedMerkleTree struct {
	merkletree.CachedTree
}

// NewCachedMerkleTree will create a CachedMerkleTree object with
// sha256 as hashing method.
func NewCachedMerkleTree(height uint64) (ct *CachedMerkleTree) {
	ct = &CachedMerkleTree{
		CachedTree: *merkletree.NewCachedTree(sha256.New(), height),
	}
	return
}

// Push will push the data into the merkle tree
func (ct *CachedMerkleTree) Push(h common.Hash) {
	ct.CachedTree.Push(h[:])
}

// PushSubTree will insert a sub merkle tree and trying to combine it with
// other data
func (ct *CachedMerkleTree) PushSubTree(height int, h common.Hash) (err error) {
	return ct.CachedTree.PushSubTree(height, h[:])
}

// Root will return the merkle root of the CachedMerkleTree
func (ct *CachedMerkleTree) Root() (h common.Hash) {
	copy(h[:], ct.CachedTree.Root())
	return
}

// Prove will be used to generate a storage proof hash set, which is used in storage
// obligation to submit the storageProof. The data field gives user more freedom to
// choose which data they want to do merkle proof with
func (ct *CachedMerkleTree) Prove(proofData []byte, cachedHashProofSet []common.Hash) (hashProofSet []common.Hash) {
	// combine base and cachedHashSet, which will be used to generate proof set
	cachedProofSet := make([][]byte, len(cachedHashProofSet)+1)
	cachedProofSet[0] = proofData
	for i, cachedHash := range cachedHashProofSet {
		cachedProofSet[i+1] = cachedHash.Bytes()
	}

	// get proofSet
	_, proofSet, _, _ := ct.CachedTree.Prove(cachedProofSet)

	// convert the proof set to hashSet
	for _, proof := range proofSet[1:] {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// MerkleRoot will calculates the merkle root of a data
func MerkleRoot(b []byte) (h common.Hash) {
	mt := NewMerkleTree()
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		mt.Push(buf.Next(MerkleLeafSize))
	}
	return mt.Root()
}

// MerkleProof will return the hash proof set of the merkle proof based on the data provided.
// proofData represents the data that needs to be hashed and combined with the data hashes
// in the proof set to check the integrity of the data
func MerkleProof(data []byte, proofIndex uint64) (proofData []byte, hashProofSet []common.Hash, leavesCount uint64, err error) {
	// create a merkle tree, and set the proofIndex
	t := NewMerkleTree()
	if err = t.SetIndex(proofIndex); err != nil {
		return
	}

	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		t.Push(buf.Next(MerkleLeafSize))
	}

	// get the proof set
	_, proofSet, index, leavesCount := t.Prove()

	// verification, if proofSet is empty, return error
	if proofSet == nil {
		err = fmt.Errorf("empty proofSet, please double check the proof index: %v. The number of merkle leaves is : %v. The index must be smaller or equal to the number of the merkle leaves",
			index, leavesCount)
		return
	}

	// get the proof data and hashed proof set
	proofData = proofSet[0]
	for _, proof := range proofSet[1:] {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// VerifyMerkleDataPiece will verify if the data piece exists in the merkle tree
func VerifyMerkleDataPiece(dataPiece []byte, hashProofSet []common.Hash, numLeaves, proofIndex uint64, merkleRoot common.Hash) (verified bool) {
	// combine data piece with hash proof set
	proofSet := make([][]byte, len(hashProofSet)+1)
	proofSet[0] = dataPiece
	for i, proof := range hashProofSet {
		proofSet[i+1] = proof.Bytes()
	}

	// verify the data piece
	return merkletree.VerifyProof(sha256.New(), merkleRoot[:], proofSet, proofIndex, numLeaves)
}

// MerkleRangeProof will create the hashProofSet for the range of data provided
// NOTE: proofStart and proofEnd are measured in terms of
// merkle leaves (64), meaning data[proofStart * 64:proofEnd * 64]
func MerkleRangeProof(data []byte, proofStart, proofEnd int) (hashPoofSet []common.Hash, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("making the merkle range proof: %s", err.Error())
		return
	}

	// get the proof set
	proofSet, err := merkletree.BuildRangeProof(proofStart, proofEnd, merkletree.NewReaderSubtreeHasher(bytes.NewReader(data), MerkleLeafSize, sha256.New()))
	if err != nil {
		return
	}

	// convert the hash slice
	for _, proof := range proofSet {
		hashPoofSet = append(hashPoofSet, common.BytesToHash(proof))
	}

	return
}

// VerifyRangeProof will verify if the data within the range provided belongs to the merkle tree
// dataWithinRange = data[start:end]
func VerifyRangeProof(dataWithinRange []byte, hashProofSet []common.Hash, proofStart, proofEnd int, merkleRoot common.Hash) (verified bool, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("verifying the range proof: %s", err)
		return
	}

	// convert the hash slice to slice of byte slice
	bytesProofSet := hashSliceToByteSlices(hashProofSet)

	// verification
	verified, err = merkletree.VerifyRangeProof(merkletree.NewReaderLeafHasher(bytes.NewReader(dataWithinRange),
		sha256.New(), MerkleLeafSize), sha256.New(), proofStart, proofEnd, bytesProofSet, merkleRoot[:])

	return
}

// MerkleSectorRangeProof is similar to MerkleRangeProof. The difference is that the latter one is
// used to create proof set for range within the  data pieces, which is divided from the data sector.
// The former one is used to create proof set for range within data sectors in a collection of data sectors
// stored in the contract. roots represents the collection of data sectors
func MerkleSectorRangeProof(roots []common.Hash, proofStart, proofEnd int) (hashProofSet []common.Hash, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("merkle sector range proof: %s", err)
		return
	}

	// conversion
	byteRoots := hashSliceToByteSlices(roots)

	// make proofSet
	sh := merkletree.NewCachedSubtreeHasher(byteRoots, sha256.New())
	proofSet, err := merkletree.BuildRangeProof(proofStart, proofEnd, sh)
	if err != nil {
		return
	}

	// convert proofSet to hashProofSet
	for _, proof := range proofSet {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// VerifySectorRangeProof is similar to VerifyRangeProof. The difference is that the latter one is
// used verify data pieces, which is divided from the data sector. The former one is used to verify
// data sectors in a collection of data sectors stored in the contract. roots represents the collection
// of data sectors. NOTE: Unlike the range proof, the data is not divided into pieces, therefore,
// the roots need to be provided will be roots[proofStart:proofEnd] == rootsVerify
func VerifySectorRangeProof(rootsVerify []common.Hash, hashProofSet []common.Hash, proofStart, proofEnd int, merkleRoot common.Hash) (verified bool, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("verify sector range proof: %s", err)
		return
	}

	// conversion
	byteRoots := hashSliceToByteSlices(rootsVerify)
	lh := merkletree.NewCachedLeafHasher(byteRoots)
	byteProofSet := hashSliceToByteSlices(hashProofSet)

	verified, err = merkletree.VerifyRangeProof(lh, sha256.New(), proofStart, proofEnd, byteProofSet, merkleRoot[:])

	return
}

// MerkleDiffProof is similar to MerkleSectorRangeProof, the only difference is that this function
// can provide multiple ranges
func MerkleDiffProof(roots []common.Hash, rangeSet []merkletree.LeafRange, leavesCount uint64) (hashProofSet []common.Hash, err error) {
	// range set validation
	if err = rangeSetVerification(rangeSet); err != nil {
		return
	}

	byteSectorRoots := hashSliceToByteSlices(roots)
	hasher := merkletree.NewCachedSubtreeHasher(byteSectorRoots, sha256.New())
	proofSet, err := merkletree.BuildDiffProof(rangeSet, hasher, leavesCount)

	// conversion
	for _, proof := range proofSet {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// VerifyDiffProof is similar to VerifySectorRangeProof, the only difference is that this function
// can provide multiple ranges
func VerifyDiffProof(rangeSet []merkletree.LeafRange, leavesCount uint64, hashProofSet, rootsVerify []common.Hash, merkleRoot common.Hash) (verified bool, err error) {
	// rangeSet verification
	if err = rangeSetVerification(rangeSet); err != nil {
		return
	}

	byteProofSet := hashSliceToByteSlices(hashProofSet)
	byteRootsVerify := hashSliceToByteSlices(rootsVerify)

	hasher := merkletree.NewCachedLeafHasher(byteRootsVerify)
	verified, err = merkletree.VerifyDiffProof(hasher, leavesCount, sha256.New(), rangeSet, byteProofSet, merkleRoot[:])

	return
}

// LeavesCount will count how many leaves a merkle tree has
func LeavesCount(dataSize uint64) (count uint64) {
	if dataSize == 0 {
		return 0
	}

	count = dataSize / MerkleLeafSize
	if count == 0 || dataSize%MerkleLeafSize != 0 {
		count++
	}
	return
}

// rangeVerification validates the start and end position provided
func rangeVerification(start, end int) (err error) {
	if start < 0 || start > end || start == end {
		err = errors.New("illegal range")
		return
	}

	return
}

// hashSliceToByteSlices convert a hash slice to a slice of byte slice
func hashSliceToByteSlices(hs []common.Hash) (bss [][]byte) {
	for _, h := range hs {
		bss = append(bss, h.Bytes())
	}
	return
}

// rangeSetVerification validates a set of ranges provided, each range
// contains a start position and end position
func rangeSetVerification(rangeSet []merkletree.LeafRange) (err error) {
	for i, r := range rangeSet {
		if r.Start < 0 || r.Start >= r.End {
			return errors.New("range set validation failed")
		}
		if i > 0 && rangeSet[i-1].End > r.Start {
			return errors.New("range set validation failed")
		}
	}
	return
}
