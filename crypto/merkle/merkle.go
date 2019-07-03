// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/merkletree"
)

// LeafSize is the data size stored in one merkle leaf.
// the original data will be divided into pieces based on the
// merkleRootSize, and then be pushed into the merkle tree
const (
	SectorSize = uint64(1 << 22)
	LeafSize   = 64
)

// Tree serves as a wrapper of the merkle tree, provide a convenient way
// to access methods associated with the merkle tree
type Tree struct {
	merkletree.Tree
}

// NewTree will initialize a Tree object with sha256 as
// the hashing method
func NewTree() (mk *Tree) {
	mk = &Tree{
		Tree: *merkletree.New(sha256.New()),
	}
	return
}

// Root will calculate and return the merkle root of the merkle tree
func (mt *Tree) Root() (h common.Hash) {
	copy(h[:], mt.Tree.Root())
	return
}

// CachedTree is similar to Tree, one obvious difference between them
// is that cached merkle tree will not hash the data before inserting them into
// the tree
type CachedTree struct {
	merkletree.CachedTree
}

// NewCachedTree will create a CachedTree object with
// sha256 as hashing method.
func NewCachedTree(height uint64) (ct *CachedTree) {
	ct = &CachedTree{
		CachedTree: *merkletree.NewCachedTree(sha256.New(), height),
	}
	return
}

// Push will push the data into the merkle tree
func (ct *CachedTree) Push(h common.Hash) {
	ct.CachedTree.Push(h[:])
}

// PushSubTree will insert a sub merkle tree and trying to combine it with
// other data
func (ct *CachedTree) PushSubTree(height int, h common.Hash) (err error) {
	return ct.CachedTree.PushSubTree(height, h[:])
}

// Root will return the merkle root of the CachedTree
func (ct *CachedTree) Root() (h common.Hash) {
	copy(h[:], ct.CachedTree.Root())
	return
}

// Prove will be used to generate a storage proof hash set, which is used in storage
// responsibility to submit the storageProof. The data field gives user more freedom to
// choose which data they want to do merkle proof with
func (ct *CachedTree) Prove(proofData []byte, cachedHashProofSet []common.Hash) (hashProofSet []common.Hash) {
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

// Root will calculates the root of a data
func Root(b []byte) (h common.Hash) {
	mt := NewTree()
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		mt.Push(buf.Next(LeafSize))
	}
	return mt.Root()
}

// CachedTreeRoot will return the root of the cached tree
func CachedTreeRoot(roots []common.Hash, height uint64) (root common.Hash) {
	cmt := NewCachedTree(height)
	for _, r := range roots {
		cmt.Push(r)
	}

	return cmt.Root()
}

// Proof will return the hash proof set of the proof based on the data provided.
// proofData represents the data that needs to be hashed and combined with the data hashes
// in the proof set to check the integrity of the data
func Proof(data []byte, proofIndex uint64) (proofData []byte, hashProofSet []common.Hash, leavesCount uint64, err error) {
	// create a tree, and set the proofIndex
	t := NewTree()
	if err = t.SetIndex(proofIndex); err != nil {
		return
	}

	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		t.Push(buf.Next(LeafSize))
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

// VerifyDataPiece will verify if the data piece exists in the merkle tree
func VerifyDataPiece(dataPiece []byte, hashProofSet []common.Hash, numLeaves, proofIndex uint64, merkleRoot common.Hash) (verified bool) {
	// combine data piece with hash proof set
	proofSet := make([][]byte, len(hashProofSet)+1)
	proofSet[0] = dataPiece
	for i, proof := range hashProofSet {
		proofSet[i+1] = proof.Bytes()
	}

	// verify the data piece
	return merkletree.VerifyProof(sha256.New(), merkleRoot[:], proofSet, proofIndex, numLeaves)
}

// RangeProof will create the hashProofSet for the range of data provided
// NOTE: proofStart and proofEnd are measured in terms of
// merkle leaves (64), meaning data[proofStart * 64:proofEnd * 64]
func RangeProof(data []byte, proofStart, proofEnd int) (hashPoofSet []common.Hash, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("making the merkle range proof: %s", err.Error())
		return
	}

	// get the proof set
	proofSet, err := merkletree.BuildRangeProof(proofStart, proofEnd, merkletree.NewReaderSubtreeHasher(bytes.NewReader(data), LeafSize, sha256.New()))
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
		sha256.New(), LeafSize), sha256.New(), proofStart, proofEnd, bytesProofSet, merkleRoot[:])

	return
}

// SectorRangeProof is similar to RangeProof. The difference is that the latter one is
// used to create proof set for range within the  data pieces, which is divided from the data sector.
// The former one is used to create proof set for range within data sectors in a collection of data sectors
// stored in the contract. roots represents the collection of data sectors
func SectorRangeProof(roots []common.Hash, proofStart, proofEnd int) (hashProofSet []common.Hash, err error) {
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

// DiffProof is similar to SectorRangeProof, the only difference is that this function
// can provide multiple ranges
func DiffProof(roots []common.Hash, rangeSet []merkletree.LeafRange, leavesCount uint64) (hashProofSet []common.Hash, err error) {
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

	var m []byte
	if reflect.DeepEqual(merkleRoot, common.Hash{}) {
		m = nil
	} else {
		m = merkleRoot[:]
	}
	verified, err = merkletree.VerifyDiffProof(hasher, leavesCount, sha256.New(), rangeSet, byteProofSet, m)

	return
}

// LeavesCount will count how many leaves a merkle tree has
func LeavesCount(dataSize uint64) (count uint64) {
	if dataSize == 0 {
		return 0
	}

	count = dataSize / LeafSize
	if count == 0 || dataSize%LeafSize != 0 {
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
