// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"gitlab.com/NebulousLabs/Sia/encoding"
	"hash"
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"gitlab.com/NebulousLabs/merkletree"
	"golang.org/x/crypto/sha3"
)

const (
	// SegmentSize is the chunk size that is used when taking the Merkle root
	// of a file. 64 is chosen because bandwidth is scarce and it optimizes for
	// the smallest possible storage proofs. Using a larger base, even 256
	// bytes, would result in substantially faster hashing, but the bandwidth
	// tradeoff was deemed to be more important, as blockchain space is scarce.
	SegmentSize = 64
)

// A ProofRange is a contiguous range of segments or sectors.
type ProofRange = merkletree.LeafRange

// MerkleTree wraps merkletree.Tree, changing some of the function definitions
// to assume sia-specific constants and return sia-specific types.
type MerkleTree struct {
	merkletree.Tree
}

// NewTree returns a MerkleTree, which can be used for getting Merkle roots and
// Merkle proofs on data. See merkletree.Tree for more details.
func NewTree() *MerkleTree {
	return &MerkleTree{*merkletree.New(NewHash())}
}

// PushObject encodes and adds the hash of the encoded object to the tree as a
// leaf.
func (t *MerkleTree) PushObject(obj interface{}) {
	t.Push(encoding.Marshal(obj))
}

// Root is a redefinition of merkletree.Tree.Root, returning a Hash instead of
// a []byte.
func (t *MerkleTree) Root() (h common.Hash) {
	copy(h[:], t.Tree.Root())
	return
}

// CachedMerkleTree wraps merkletree.CachedTree, changing some of the function
// definitions to assume sia-specific constants and return sia-specific types.
type CachedMerkleTree struct {
	merkletree.CachedTree
}

// NewCachedTree returns a CachedMerkleTree, which can be used for getting
// Merkle roots and proofs from data that has cached subroots. See
// merkletree.CachedTree for more details.
func NewCachedTree(height uint64) *CachedMerkleTree {
	return &CachedMerkleTree{*merkletree.NewCachedTree(NewHash(), height)}
}

// Prove is a redefinition of merkletree.CachedTree.Prove, so that Sia-specific
// types are used instead of the generic types used by the parent package. The
// base is not a return value because the base is used as input.
func (ct *CachedMerkleTree) Prove(base []byte, cachedHashSet []common.Hash) []common.Hash {
	// Turn the input in to a proof set that will be recognized by the high
	// level tree.
	cachedProofSet := make([][]byte, len(cachedHashSet)+1)
	cachedProofSet[0] = base
	for i := range cachedHashSet {
		cachedProofSet[i+1] = cachedHashSet[i][:]
	}
	_, proofSet, _, _ := ct.CachedTree.Prove(cachedProofSet)

	// convert proofSet to base and hashSet
	hashSet := make([]common.Hash, len(proofSet)-1)
	for i, proof := range proofSet[1:] {
		copy(hashSet[i][:], proof)
	}
	return hashSet
}

// Push is a redefinition of merkletree.CachedTree.Push, with the added type
// safety of only accepting a hash.
func (ct *CachedMerkleTree) Push(h common.Hash) {
	ct.CachedTree.Push(h[:])
}

// PushSubTree is a redefinition of merkletree.CachedTree.PushSubTree, with the
// added type safety of only accepting a hash.
func (ct *CachedMerkleTree) PushSubTree(height int, h common.Hash) error {
	return ct.CachedTree.PushSubTree(height, h[:])
}

// Root is a redefinition of merkletree.CachedTree.Root, returning a Hash
// instead of a []byte.
func (ct *CachedMerkleTree) Root() (h common.Hash) {
	copy(h[:], ct.CachedTree.Root())
	return
}

// CalculateLeaves calculates the number of leaves that would be pushed from
// data of size 'dataSize'.
func CalculateLeaves(dataSize uint64) uint64 {
	numSegments := dataSize / SegmentSize
	if dataSize == 0 || dataSize%SegmentSize != 0 {
		numSegments++
	}
	return numSegments
}

// MerkleRoot returns the Merkle root of the input data.
func MerkleRoot(b []byte) common.Hash {
	t := NewTree()
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		t.Push(buf.Next(SegmentSize))
	}
	return t.Root()
}

// MerkleProof builds a Merkle proof that the data at segment 'proofIndex' is a
// part of the Merkle root formed by 'b'.
//
// MerkleProof is NOT equivalent to MerkleRangeProof for a single segment.
func MerkleProof(b []byte, proofIndex uint64) (base []byte, hashSet []common.Hash) {
	// Create the tree.
	t := NewTree()
	t.SetIndex(proofIndex)

	// Fill the tree.
	buf := bytes.NewBuffer(b)
	for buf.Len() > 0 {
		t.Push(buf.Next(SegmentSize))
	}

	// Get the proof and convert it to a base + hash set.
	_, proof, _, _ := t.Prove()
	if len(proof) == 0 {
		// There's no proof, because there's no data. Return blank values.
		return nil, nil
	}

	base = proof[0]
	hashSet = make([]common.Hash, len(proof)-1)
	for i, p := range proof[1:] {
		copy(hashSet[i][:], p)
	}
	return base, hashSet
}

// VerifySegment will verify that a segment, given the proof, is a part of a
// Merkle root.
//
// VerifySegment is NOT equivalent to VerifyRangeProof for a single segment.
func VerifySegment(base []byte, hashSet []common.Hash, numSegments, proofIndex uint64, root common.Hash) bool {
	// convert base and hashSet to proofSet
	proofSet := make([][]byte, len(hashSet)+1)
	proofSet[0] = base
	for i := range hashSet {
		proofSet[i+1] = hashSet[i][:]
	}
	return merkletree.VerifyProof(NewHash(), root[:], proofSet, proofIndex, numSegments)
}

// MerkleRangeProof builds a Merkle proof for the segment range [start,end).
//
// MerkleRangeProof for a single segment is NOT equivalent to MerkleProof.
func MerkleRangeProof(b []byte, start, end int) []common.Hash {
	proof, _ := merkletree.BuildRangeProof(start, end, merkletree.NewReaderSubtreeHasher(bytes.NewReader(b), SegmentSize, NewHash()))
	proofHashes := make([]common.Hash, len(proof))
	for i := range proofHashes {
		copy(proofHashes[i][:], proof[i])
	}
	return proofHashes
}

// VerifyRangeProof verifies a proof produced by MerkleRangeProof.
//
// VerifyRangeProof for a single segment is NOT equivalent to VerifySegment.
func VerifyRangeProof(segments []byte, proof []common.Hash, start, end int, root common.Hash) bool {
	proofBytes := make([][]byte, len(proof))
	for i := range proof {
		proofBytes[i] = proof[i][:]
	}
	result, _ := merkletree.VerifyRangeProof(merkletree.NewReaderLeafHasher(bytes.NewReader(segments), NewHash(), SegmentSize), NewHash(), start, end, proofBytes, root[:])
	return result
}

// MerkleSectorRangeProof builds a Merkle proof for the sector range [start,end).
func MerkleSectorRangeProof(roots []common.Hash, start, end int) []common.Hash {
	leafHashes := make([][]byte, len(roots))
	for i := range leafHashes {
		leafHashes[i] = roots[i][:]
	}
	sh := merkletree.NewCachedSubtreeHasher(leafHashes, NewHash())
	proof, _ := merkletree.BuildRangeProof(start, end, sh)
	proofHashes := make([]common.Hash, len(proof))
	for i := range proofHashes {
		copy(proofHashes[i][:], proof[i])
	}
	return proofHashes
}

// VerifySectorRangeProof verifies a proof produced by MerkleSectorRangeProof.
func VerifySectorRangeProof(roots []common.Hash, proof []common.Hash, start, end int, root common.Hash) bool {
	leafHashes := make([][]byte, len(roots))
	for i := range leafHashes {
		leafHashes[i] = roots[i][:]
	}
	lh := merkletree.NewCachedLeafHasher(leafHashes)
	proofBytes := make([][]byte, len(proof))
	for i := range proof {
		proofBytes[i] = proof[i][:]
	}
	result, _ := merkletree.VerifyRangeProof(lh, NewHash(), start, end, proofBytes, root[:])
	return result
}

// MerkleDiffProof builds a Merkle proof for multiple segment ranges.
func MerkleDiffProof(ranges []ProofRange, numLeaves uint64, updatedSectors [][]byte, sectorRoots []common.Hash) []common.Hash {
	leafHashes := make([][]byte, len(sectorRoots))
	for i := range leafHashes {
		leafHashes[i] = sectorRoots[i][:]
	}
	sh := merkletree.NewCachedSubtreeHasher(leafHashes, NewHash()) // TODO: needs to include updatedSectors somehow
	proof, _ := merkletree.BuildDiffProof(ranges, sh, numLeaves)
	proofHashes := make([]common.Hash, len(proof))
	for i := range proofHashes {
		copy(proofHashes[i][:], proof[i])
	}
	return proofHashes
}

// VerifyDiffProof verifies a proof produced by MerkleDiffProof.
func VerifyDiffProof(ranges []ProofRange, numLeaves uint64, proofHashes, leafHashes []common.Hash, root common.Hash) bool {
	proofBytes := make([][]byte, len(proofHashes))
	for i := range proofHashes {
		proofBytes[i] = proofHashes[i][:]
	}
	leafBytes := make([][]byte, len(leafHashes))
	for i := range leafHashes {
		leafBytes[i] = leafHashes[i][:]
	}
	rootBytes := root[:]
	if root == (common.Hash{}) {
		rootBytes = nil // empty trees hash to nil, not 32 zeros
	}
	lh := merkletree.NewCachedLeafHasher(leafBytes)
	ok, _ := merkletree.VerifyDiffProof(lh, numLeaves, NewHash(), ranges, proofBytes, rootBytes)
	return ok
}

func NewHash() hash.Hash {
	h := sha3.NewLegacyKeccak256()
	return h
}

func CachedMerkleRoot(roots []common.Hash) common.Hash {
	log2SectorSize := uint64(0)
	for 1<<log2SectorSize < (SectorSize / SegmentSize) {
		log2SectorSize++
	}
	ct := NewCachedTree(log2SectorSize)
	for _, root := range roots {
		ct.Push(root)
	}
	return ct.Root()
}

// calculateProofRanges returns the proof ranges that should be used to verify a
// pre-modification Merkle diff proof for the specified actions.
func CalculateProofRanges(actions []UploadAction, oldNumSectors uint64) []ProofRange {
	newNumSectors := oldNumSectors
	sectorsChanged := make(map[uint64]struct{})
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			sectorsChanged[newNumSectors] = struct{}{}
			newNumSectors++
		}
	}

	oldRanges := make([]ProofRange, 0, len(sectorsChanged))
	for index := range sectorsChanged {
		if index < oldNumSectors {
			oldRanges = append(oldRanges, ProofRange{
				Start: index,
				End:   index + 1,
			})
		}
	}
	sort.Slice(oldRanges, func(i, j int) bool {
		return oldRanges[i].Start < oldRanges[j].Start
	})

	return oldRanges
}

// modifyProofRanges modifies the proof ranges produced by calculateProofRanges
// to verify a post-modification Merkle diff proof for the specified actions.
func ModifyProofRanges(proofRanges []ProofRange, actions []UploadAction, numSectors uint64) []ProofRange {
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			proofRanges = append(proofRanges, ProofRange{
				Start: numSectors,
				End:   numSectors + 1,
			})
			numSectors++
		}
	}
	return proofRanges
}

// modifyLeaves modifies the leaf hashes of a Merkle diff proof to verify a
// post-modification Merkle diff proof for the specified actions.
func ModifyLeaves(leafHashes []common.Hash, actions []UploadAction, numSectors uint64) []common.Hash {
	// determine which sector index corresponds to each leaf hash
	var indices []uint64
	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			indices = append(indices, numSectors)
			numSectors++
		}
	}
	sort.Slice(indices, func(i, j int) bool {
		return indices[i] < indices[j]
	})
	indexMap := make(map[uint64]int, len(leafHashes))
	for i, index := range indices {
		if i > 0 && index == indices[i-1] {
			continue // remove duplicates
		}
		indexMap[index] = i
	}

	for _, action := range actions {
		switch action.Type {
		case UploadActionAppend:
			leafHashes = append(leafHashes, MerkleRoot(action.Data))
		}
	}
	return leafHashes
}
