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
)

// LeafSize is the data size stored in one merkle leaf.
// the original data will be divided into pieces based on the
// merkleRootSize, and then be pushed into the merkle tree
const (
	SectorSize = uint64(1 << 22)
	LeafSize   = 64
)

// Sha256MerkleTree serves as a wrapper of the merkle tree, provide a convenient way
// to access methods associated with the merkle tree
type Sha256MerkleTree struct {
	Tree
}

// NewSha256MerkleTree will initialize a Sha256MerkleTree object with sha256 as
// the hashing method
func NewSha256MerkleTree() (mk *Sha256MerkleTree) {
	mk = &Sha256MerkleTree{
		Tree: *NewTree(sha256.New()),
	}
	return
}

// Root will calculate and return the merkle root of the merkle tree
func (mt *Sha256MerkleTree) Root() (h common.Hash) {
	copy(h[:], mt.Tree.Root())
	return
}

// Sha256CachedTree is similar to Sha256MerkleTree, one obvious difference between them
// is that cached merkle tree will not hash the data before inserting them into
// the tree
type Sha256CachedTree struct {
	CachedTree
}

// NewSha256CachedTree will create a Sha256CachedTree object with
// sha256 as hashing method.
func NewSha256CachedTree(height uint64) (ct *Sha256CachedTree) {
	ct = &Sha256CachedTree{
		CachedTree: *NewCachedTree(sha256.New(), height),
	}
	return
}

// Push will push the data into the merkle tree
func (ct *Sha256CachedTree) Push(h common.Hash) {
	ct.CachedTree.PushLeaf(h[:])
}

// PushSubTree will insert a sub merkle tree and trying to combine it with
// other data
func (ct *Sha256CachedTree) PushSubTree(height int, h common.Hash) (err error) {
	return ct.CachedTree.PushSubTree(height, h[:])
}

// Root will return the merkle root of the Sha256CachedTree
func (ct *Sha256CachedTree) Root() (h common.Hash) {
	copy(h[:], ct.CachedTree.Root())
	return
}

// Prove will be used to generate a storage proof hash set, which is used in storage
// responsibility to submit the storageProof. The data field gives user more freedom to
// choose which data they want to do merkle proof with
func (ct *Sha256CachedTree) Prove(proofData []byte, cachedHashProofSet []common.Hash) (hashProofSet []common.Hash) {
	// combine base and cachedHashSet, which will be used to generate proof set
	cachedProofSet := make([][]byte, len(cachedHashProofSet)+1)
	cachedProofSet[0] = proofData
	for i, cachedHash := range cachedHashProofSet {
		cachedProofSet[i+1] = cachedHash.Bytes()
	}

	// get proofSet
	_, proofSet, _, _ := ct.CachedTree.ProofList(cachedProofSet)

	// convert the proof set to hashSet
	for _, proof := range proofSet[1:] {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// Sha256MerkleTreeRoot will calculates the root of a data
func Sha256MerkleTreeRoot(b []byte) (h common.Hash) {
	mt := NewSha256MerkleTree()
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		mt.PushLeaf(buf.Next(LeafSize))
	}
	return mt.Root()
}

// Sha256CachedTreeRoot will return the root of the cached tree
func Sha256CachedTreeRoot(roots []common.Hash, height uint64) (root common.Hash) {
	cmt := NewSha256CachedTree(height)
	for _, r := range roots {
		cmt.Push(r)
	}

	return cmt.Root()
}

//Sha256CachedTreeRoot2 will return the root of the cached tree
func Sha256CachedTreeRoot2(roots []common.Hash) (root common.Hash) {
	log2SectorSize := uint64(0)
	for 1<<log2SectorSize < (SectorSize / LeafSize) {
		log2SectorSize++
	}
	cmt := NewSha256CachedTree(log2SectorSize)
	for _, r := range roots {
		cmt.Push(r)
	}

	return cmt.Root()
}

// Sha256MerkleTreeProof will return the hash proof set of the proof based on the data provided.
// proofData represents the data that needs to be hashed and combined with the data hashes
// in the proof set to check the integrity of the data
func Sha256MerkleTreeProof(data []byte, proofIndex uint64) (proofData []byte, hashProofSet []common.Hash, leavesCount uint64, err error) {
	// create a tree, and set the proofIndex
	t := NewSha256MerkleTree()
	if err = t.SetStorageProofIndex(proofIndex); err != nil {
		return
	}

	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		t.PushLeaf(buf.Next(LeafSize))
	}

	// get the proof set
	_, proofSet, index, leavesCount := t.ProofList()

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

// Sha256VerifyDataPiece will verify if the data piece exists in the merkle tree
func Sha256VerifyDataPiece(dataPiece []byte, hashProofSet []common.Hash, numLeaves, proofIndex uint64, merkleRoot common.Hash) (verified bool) {
	// combine data piece with hash proof set
	proofSet := make([][]byte, len(hashProofSet)+1)
	proofSet[0] = dataPiece
	for i, proof := range hashProofSet {
		proofSet[i+1] = proof.Bytes()
	}

	// verify the data piece
	return CheckStorageProof(sha256.New(), merkleRoot[:], proofSet, proofIndex, numLeaves)
}

// Sha256RangeProof will create the hashProofSet for the range of data provided
// NOTE: proofStart and proofEnd are measured in terms of
// merkle leaves (64), meaning data[proofStart * 64:proofEnd * 64]
func Sha256RangeProof(data []byte, proofStart, proofEnd int) (hashPoofSet []common.Hash, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("making the merkle range proof: %s", err.Error())
		return
	}

	// get the proof set
	proofSet, err := GetLimitStorageProof(proofStart, proofEnd, NewSubtreeRootReader(bytes.NewReader(data), LeafSize, sha256.New()))
	if err != nil {
		return
	}

	// convert the hash slice
	for _, proof := range proofSet {
		hashPoofSet = append(hashPoofSet, common.BytesToHash(proof))
	}

	return
}

// Sha256VerifyRangeProof will verify if the data within the range provided belongs to the merkle tree
// dataWithinRange = data[start:end]
func Sha256VerifyRangeProof(dataWithinRange []byte, hashProofSet []common.Hash, proofStart, proofEnd int, merkleRoot common.Hash) (verified bool, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("verifying the range proof: %s", err)
		return
	}

	// convert the hash slice to slice of byte slice
	bytesProofSet := hashSliceToByteSlices(hashProofSet)

	// verification
	verified, err = CheckLimitStorageProof(NewLeafRootReader(bytes.NewReader(dataWithinRange),
		sha256.New(), LeafSize), sha256.New(), proofStart, proofEnd, bytesProofSet, merkleRoot[:])

	return
}

// Sha256SectorRangeProof is similar to Sha256RangeProof. The difference is that the latter one is
// used to create proof set for range within the  data pieces, which is divided from the data sector.
// The former one is used to create proof set for range within data sectors in a collection of data sectors
// stored in the contract. roots represents the collection of data sectors
func Sha256SectorRangeProof(roots []common.Hash, proofStart, proofEnd int) (hashProofSet []common.Hash, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("merkle sector range proof: %s", err)
		return
	}

	// conversion
	byteRoots := hashSliceToByteSlices(roots)

	// make proofSet
	sh := NewCachedSubtreeRoot(byteRoots, sha256.New())
	proofSet, err := GetLimitStorageProof(proofStart, proofEnd, sh)
	if err != nil {
		return
	}

	// convert proofSet to hashProofSet
	for _, proof := range proofSet {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// Sha256VerifySectorRangeProof is similar to Sha256VerifyRangeProof. The difference is that the latter one is
// used verify data pieces, which is divided from the data sector. The former one is used to verify
// data sectors in a collection of data sectors stored in the contract. roots represents the collection
// of data sectors. NOTE: Unlike the range proof, the data is not divided into pieces, therefore,
// the roots need to be provided will be roots[proofStart:proofEnd] == rootsVerify
func Sha256VerifySectorRangeProof(rootsVerify []common.Hash, hashProofSet []common.Hash, proofStart, proofEnd int, merkleRoot common.Hash) (verified bool, err error) {
	// range validation
	if err = rangeVerification(proofStart, proofEnd); err != nil {
		err = fmt.Errorf("verify sector range proof: %s", err)
		return
	}

	// conversion
	byteRoots := hashSliceToByteSlices(rootsVerify)
	lh := NewLeafRootCached(byteRoots)
	byteProofSet := hashSliceToByteSlices(hashProofSet)

	verified, err = CheckLimitStorageProof(lh, sha256.New(), proofStart, proofEnd, byteProofSet, merkleRoot[:])

	return
}

// Sha256DiffProof is similar to Sha256SectorRangeProof, the only difference is that this function
// can provide multiple ranges
func Sha256DiffProof(roots []common.Hash, rangeSet []SubTreeLimit, leavesCount uint64) (hashProofSet []common.Hash, err error) {
	// range set validation
	if err = rangeSetVerification(rangeSet); err != nil {
		return
	}

	byteSectorRoots := hashSliceToByteSlices(roots)
	hasher := NewCachedSubtreeRoot(byteSectorRoots, sha256.New())
	proofSet, err := GetDiffStorageProof(rangeSet, hasher, leavesCount)

	// conversion
	for _, proof := range proofSet {
		hashProofSet = append(hashProofSet, common.BytesToHash(proof))
	}

	return
}

// Sha256VerifyDiffProof is similar to Sha256VerifySectorRangeProof, the only difference is that this function
// can provide multiple ranges
func Sha256VerifyDiffProof(rangeSet []SubTreeLimit, leavesCount uint64, hashProofSet, rootsVerify []common.Hash, merkleRoot common.Hash) (err error) {
	// rangeSet verification
	if err = rangeSetVerification(rangeSet); err != nil {
		return
	}

	byteProofSet := hashSliceToByteSlices(hashProofSet)
	byteRootsVerify := hashSliceToByteSlices(rootsVerify)

	hasher := NewLeafRootCached(byteRootsVerify)
	var m []byte
	if reflect.DeepEqual(merkleRoot, common.Hash{}) {
		m = nil
	} else {
		m = merkleRoot[:]
	}

	err = CheckDiffStorageProof(hasher, leavesCount, sha256.New(), rangeSet, byteProofSet, m)
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
func rangeSetVerification(rangeSet []SubTreeLimit) (err error) {
	for i, r := range rangeSet {
		if r.Left < 0 || r.Left >= r.Right {
			return errors.New("range set validation failed")
		}
		if i > 0 && rangeSet[i-1].Right > r.Left {
			return errors.New("range set validation failed")
		}
	}
	return
}
