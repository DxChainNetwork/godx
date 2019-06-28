package merkle

import (
	"bytes"
	"hash"
	"io"
	"io/ioutil"
	"math"
	"math/bits"
)

type LeafRange struct {
	Start uint64
	End   uint64
}

// nextSubtreeSize
func nextSubtreeSize(start, end uint64) int {
	ideal := bits.TrailingZeros64(start)
	max := bits.Len64(end-start) - 1
	if ideal > max {
		return 1 << uint(max)
	}
	return 1 << uint(ideal)
}

// validRangeSet
func validRangeSet(ranges []LeafRange) bool {
	for i, r := range ranges {
		if r.Start < 0 || r.Start >= r.End {
			return false
		}
		if i > 0 && ranges[i-1].End > r.Start {
			return false
		}
	}
	return true
}

// A SubtreeHasher
type SubtreeHasher interface {
	NextSubtreeRoot(n int) ([]byte, error)

	Skip(n int) error
}

// ReaderSubtreeHasher
type ReaderSubtreeHasher struct {
	r    io.Reader
	h    hash.Hash
	leaf []byte
}

// NextSubtreeRoot
func (rsh *ReaderSubtreeHasher) NextSubtreeRoot(subtreeSize int) ([]byte, error) {
	tree := NewTree(rsh.h)
	for i := 0; i < subtreeSize; i++ {
		n, err := io.ReadFull(rsh.r, rsh.leaf)
		if n > 0 {
			tree.PushLeaf(rsh.leaf[:n])
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	root := tree.Root()
	if root == nil {
		return nil, io.EOF
	}
	return root, nil
}

// Skip implements SubtreeHasher.
func (rsh *ReaderSubtreeHasher) Skip(n int) (err error) {
	skipSize := int64(len(rsh.leaf) * n)
	skipped, err := io.CopyN(ioutil.Discard, rsh.r, skipSize)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		if skipped == skipSize {
			return nil
		}
		return io.ErrUnexpectedEOF
	}
	return err
}

// NewReaderSubtreeHasher
func NewReaderSubtreeHasher(r io.Reader, leafSize int, h hash.Hash) *ReaderSubtreeHasher {
	return &ReaderSubtreeHasher{
		r:    r,
		h:    h,
		leaf: make([]byte, leafSize),
	}
}

// CachedSubtreeHasher
type CachedSubtreeHasher struct {
	leafHashes [][]byte
	h          hash.Hash
}

// NextSubtreeRoot implements SubtreeHasher.
func (csh *CachedSubtreeHasher) NextSubtreeRoot(subtreeSize int) ([]byte, error) {
	if len(csh.leafHashes) == 0 {
		return nil, io.EOF
	}
	tree := NewTree(csh.h)
	for i := 0; i < subtreeSize && len(csh.leafHashes) > 0; i++ {
		if err := tree.PushSubTree(0, csh.leafHashes[0]); err != nil {
			return nil, err
		}
		csh.leafHashes = csh.leafHashes[1:]
	}
	return tree.Root(), nil
}

// Skip
func (csh *CachedSubtreeHasher) Skip(n int) error {
	if n > len(csh.leafHashes) {
		return io.ErrUnexpectedEOF
	}
	csh.leafHashes = csh.leafHashes[n:]
	return nil
}

// NewCachedSubtreeHasher
func NewCachedSubtreeHasher(leafHashes [][]byte, h hash.Hash) *CachedSubtreeHasher {
	return &CachedSubtreeHasher{
		leafHashes: leafHashes,
		h:          h,
	}
}

// BuildMultiRangeProof
func BuildMultiRangeProof(ranges []LeafRange, h SubtreeHasher) (proof [][]byte, err error) {
	if len(ranges) == 0 {
		return nil, nil
	}
	if !validRangeSet(ranges) {
		panic("BuildMultiRangeProof: illegal set of proof ranges")
	}

	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end {
			subtreeSize := nextSubtreeSize(leafIndex, end)
			root, err := h.NextSubtreeRoot(subtreeSize)
			if err != nil {
				return err
			}
			proof = append(proof, root)
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}

	for _, r := range ranges {
		if err := consumeUntil(r.Start); err != nil {
			return nil, err
		}
		if err := h.Skip(int(r.End - r.Start)); err != nil {
			return nil, err
		}
		leafIndex += r.End - r.Start
	}
	err = consumeUntil(math.MaxUint64)
	if err == io.EOF {
		err = nil // EOF is expected
	}
	return proof, err
}

// BuildRangeProof
func BuildRangeProof(proofStart, proofEnd int, h SubtreeHasher) (proof [][]byte, err error) {
	if proofStart < 0 || proofStart > proofEnd || proofStart == proofEnd {
		panic("BuildRangeProof: illegal proof range")
	}
	return BuildMultiRangeProof([]LeafRange{{uint64(proofStart), uint64(proofEnd)}}, h)
}

// A LeafHasher
type LeafHasher interface {
	NextLeafHash() ([]byte, error)
}

// ReaderLeafHasher
type ReaderLeafHasher struct {
	r    io.Reader
	h    hash.Hash
	leaf []byte
}

// NextLeafHash
func (rlh *ReaderLeafHasher) NextLeafHash() ([]byte, error) {
	n, err := io.ReadFull(rlh.r, rlh.leaf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	} else if n == 0 {
		return nil, io.EOF
	}
	return leafTotal(rlh.h, rlh.leaf[:n]), nil
}

// NewReaderLeafHasher
func NewReaderLeafHasher(r io.Reader, h hash.Hash, leafSize int) *ReaderLeafHasher {
	return &ReaderLeafHasher{
		r:    r,
		h:    h,
		leaf: make([]byte, leafSize),
	}
}

// CachedLeafHasher
type CachedLeafHasher struct {
	leafHashes [][]byte
}

// NextLeafHash
func (clh *CachedLeafHasher) NextLeafHash() ([]byte, error) {
	if len(clh.leafHashes) == 0 {
		return nil, io.EOF
	}
	h := clh.leafHashes[0]
	clh.leafHashes = clh.leafHashes[1:]
	return h, nil
}

// NewCachedLeafHasher
func NewCachedLeafHasher(leafHashes [][]byte) *CachedLeafHasher {
	return &CachedLeafHasher{
		leafHashes: leafHashes,
	}
}

// VerifyMultiRangeProof
func VerifyMultiRangeProof(lh LeafHasher, h hash.Hash, ranges []LeafRange, proof [][]byte, root []byte) (bool, error) {
	if len(ranges) == 0 {
		return true, nil
	}
	if !validRangeSet(ranges) {
		panic("BuildMultiRangeProof: illegal set of proof ranges")
	}

	tree := NewTree(h)
	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end && len(proof) > 0 {
			subtreeSize := nextSubtreeSize(leafIndex, end)
			i := bits.TrailingZeros64(uint64(subtreeSize)) // log2
			if err := tree.PushSubTree(i, proof[0]); err != nil {
				return err
			}
			proof = proof[1:]
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}

	for _, r := range ranges {
		if err := consumeUntil(r.Start); err != nil {
			return false, err
		}

		for i := r.Start; i < r.End; i++ {
			leafHash, err := lh.NextLeafHash()
			if err != nil {
				return false, err
			}
			if err := tree.PushSubTree(0, leafHash); err != nil {
				panic(err)
			}
		}
		leafIndex += r.End - r.Start
	}

	if err := consumeUntil(math.MaxUint64); err != nil {
		return false, err
	}

	return bytes.Equal(tree.Root(), root), nil
}

// VerifyRangeProof
func VerifyRangeProof(lh LeafHasher, h hash.Hash, proofStart, proofEnd int, proof [][]byte, root []byte) (bool, error) {
	if proofStart < 0 || proofStart > proofEnd || proofStart == proofEnd {
		panic("VerifyRangeProof: illegal proof range")
	}
	return VerifyMultiRangeProof(lh, h, []LeafRange{{uint64(proofStart), uint64(proofEnd)}}, proof, root)
}

// proofMapping
func proofMapping(proofSize, proofIndex int) (mapping []int) {
	numRights := proofSize - bits.OnesCount(uint(proofIndex))
	var left, right []int
	for i := 0; len(left)+len(right) < proofSize; i++ {
		subtreeSize := 1 << uint64(i)
		if proofIndex&subtreeSize != 0 {
			left = append(left, len(left)+len(right))
		} else if len(right) < numRights {
			right = append(right, len(left)+len(right))
		}
	}

	for i := range left {
		mapping = append(mapping, left[len(left)-i-1])
	}
	return append(mapping, right...)
}

// ConvertSingleProofToRangeProof
func ConvertSingleProofToRangeProof(proof [][]byte, proofIndex int) [][]byte {
	newproof := make([][]byte, len(proof))
	mapping := proofMapping(len(proof), proofIndex)
	for i, j := range mapping {
		newproof[i] = proof[j]
	}
	return newproof
}

// ConvertRangeProofToSingleProof
func ConvertRangeProofToSingleProof(proof [][]byte, proofIndex int) [][]byte {
	oldproof := make([][]byte, len(proof))
	mapping := proofMapping(len(proof), proofIndex)
	for i, j := range mapping {
		oldproof[j] = proof[i]
	}
	return oldproof
}
