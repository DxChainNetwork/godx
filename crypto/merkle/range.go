package merkle

import (
	"bytes"
	"hash"
	"io"
	"io/ioutil"
	"math"
	"math/bits"
)

type subTreeRange struct {
	Left  uint64
	Right uint64
}

// adjacentSubtreeSize get the size of the adjacent subtree
func adjacentSubtreeSize(left, right uint64) int {
	leftInt := bits.TrailingZeros64(left)
	max := bits.Len64(right-left) - 1
	if leftInt > max {
		return 1 << uint(max)
	}
	return 1 << uint(leftInt)
}

// checkRangeList check parameter legality
func checkRangeList(ranges []subTreeRange) bool {
	for i, r := range ranges {
		if r.Left < 0 || r.Left >= r.Right {
			return false
		}
		if i > 0 && ranges[i-1].Right > r.Left {
			return false
		}
	}
	return true
}

// SubtreeRoot
type SubtreeRoot interface {

	//GetSubtreeRoot get the root hash of the subtree of n leaf node combinations
	GetSubtreeRoot(n int) ([]byte, error)

	//Skip skip the subtree of n leaf node combinations
	Skip(n int) error
}

// SubtreeRootReader implements SubtreeRoot.
type SubtreeRootReader struct {
	r    io.Reader
	h    hash.Hash
	leaf []byte
}

// GetSubtreeRoot implements SubtreeRoot.
func (rsh *SubtreeRootReader) GetSubtreeRoot(leafIndex int) ([]byte, error) {
	tree := NewTree(rsh.h)
	for i := 0; i < leafIndex; i++ {
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

// Skip implements SubtreeRoot.
func (rsh *SubtreeRootReader) Skip(n int) (err error) {
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

// NewSubtreeRootReader
func NewSubtreeRootReader(r io.Reader, leafNumber int, h hash.Hash) *SubtreeRootReader {
	return &SubtreeRootReader{
		r:    r,
		h:    h,
		leaf: make([]byte, leafNumber),
	}
}

// CachedSubtreeRoot implements SubtreeRoot.
type CachedSubtreeRoot struct {
	leafHashes [][]byte
	h          hash.Hash
}

// GetSubtreeRoot implements SubtreeRoot.
func (csh *CachedSubtreeRoot) GetSubtreeRoot(leafIndex int) ([]byte, error) {
	if len(csh.leafHashes) == 0 {
		return nil, io.EOF
	}
	tree := NewTree(csh.h)
	for i := 0; i < leafIndex && len(csh.leafHashes) > 0; i++ {
		if err := tree.PushSubTree(0, csh.leafHashes[0]); err != nil {
			return nil, err
		}
		csh.leafHashes = csh.leafHashes[1:]
	}
	return tree.Root(), nil
}

// Skip implements SubtreeRoot.
func (csh *CachedSubtreeRoot) Skip(n int) error {
	if n > len(csh.leafHashes) {
		return io.ErrUnexpectedEOF
	}
	csh.leafHashes = csh.leafHashes[n:]
	return nil
}

// NewCachedSubtreeRoot
func NewCachedSubtreeRoot(leafHashes [][]byte, h hash.Hash) *CachedSubtreeRoot {
	return &CachedSubtreeRoot{
		leafHashes: leafHashes,
		h:          h,
	}
}

// getRangeStorageProof get a proof of storage for a range of subtrees
func getRangeStorageProof(ranges []subTreeRange, h SubtreeRoot) (proof [][]byte, err error) {
	if len(ranges) == 0 {
		return nil, nil
	}
	if !checkRangeList(ranges) {
		panic("getRangeStorageProof: the parameter is invalid")
	}

	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end {
			subtreeSize := adjacentSubtreeSize(leafIndex, end)
			root, err := h.GetSubtreeRoot(subtreeSize)
			if err != nil {
				return err
			}
			proof = append(proof, root)
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}

	for _, r := range ranges {
		if err := consumeUntil(r.Left); err != nil {
			return nil, err
		}
		if err := h.Skip(int(r.Right - r.Left)); err != nil {
			return nil, err
		}
		leafIndex += r.Right - r.Left
	}
	err = consumeUntil(math.MaxUint64)
	if err == io.EOF {
		err = nil // EOF is expected
	}
	return proof, err
}

// GetRangeStorageProof get a proof of storage for a range of subtrees
func GetRangeStorageProof(proofStart, proofEnd int, h SubtreeRoot) (proof [][]byte, err error) {
	if proofStart < 0 || proofStart > proofEnd || proofStart == proofEnd {
		panic("GetRangeStorageProof: getRangeStorageProof")
	}
	return getRangeStorageProof([]subTreeRange{{uint64(proofStart), uint64(proofEnd)}}, h)
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
func VerifyMultiRangeProof(lh LeafHasher, h hash.Hash, ranges []subTreeRange, proof [][]byte, root []byte) (bool, error) {
	if len(ranges) == 0 {
		return true, nil
	}
	if !checkRangeList(ranges) {
		panic("BuildMultiRangeProof: illegal set of proof ranges")
	}

	tree := NewTree(h)
	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end && len(proof) > 0 {
			subtreeSize := adjacentSubtreeSize(leafIndex, end)
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
		if err := consumeUntil(r.Left); err != nil {
			return false, err
		}

		for i := r.Left; i < r.Right; i++ {
			leafHash, err := lh.NextLeafHash()
			if err != nil {
				return false, err
			}
			if err := tree.PushSubTree(0, leafHash); err != nil {
				panic(err)
			}
		}
		leafIndex += r.Right - r.Left
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
	return VerifyMultiRangeProof(lh, h, []subTreeRange{{uint64(proofStart), uint64(proofEnd)}}, proof, root)
}
