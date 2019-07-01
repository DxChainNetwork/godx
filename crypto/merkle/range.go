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
	leafRoots [][]byte
	h         hash.Hash
}

// GetSubtreeRoot implements SubtreeRoot.
func (csh *CachedSubtreeRoot) GetSubtreeRoot(leafIndex int) ([]byte, error) {
	if len(csh.leafRoots) == 0 {
		return nil, io.EOF
	}
	tree := NewTree(csh.h)
	for i := 0; i < leafIndex && len(csh.leafRoots) > 0; i++ {
		if err := tree.PushSubTree(0, csh.leafRoots[0]); err != nil {
			return nil, err
		}
		csh.leafRoots = csh.leafRoots[1:]
	}
	return tree.Root(), nil
}

// Skip implements SubtreeRoot.
func (csh *CachedSubtreeRoot) Skip(n int) error {
	if n > len(csh.leafRoots) {
		return io.ErrUnexpectedEOF
	}
	csh.leafRoots = csh.leafRoots[n:]
	return nil
}

// NewCachedSubtreeRoot return cachedSubtreeRoot
func NewCachedSubtreeRoot(roots [][]byte, h hash.Hash) *CachedSubtreeRoot {
	return &CachedSubtreeRoot{
		leafRoots: roots,
		h:         h,
	}
}

// getRangeStorageProof get a proof of storage for a range of subtrees
func getRangeStorageProof(ranges []subTreeRange, sr SubtreeRoot) (storageProofList [][]byte, err error) {
	if len(ranges) == 0 {
		return nil, nil
	}
	if !checkRangeList(ranges) {
		panic("getRangeStorageProof: the parameter is invalid")
	}

	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end {
			//get the size of the adjacent subtree
			subtreeSize := adjacentSubtreeSize(leafIndex, end)
			//get the root hash of the subtree of n leaf node combinations
			root, err := sr.GetSubtreeRoot(subtreeSize)
			if err != nil {
				return err
			}
			storageProofList = append(storageProofList, root)
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}

	for _, r := range ranges {
		if err := consumeUntil(r.Left); err != nil {
			return nil, err
		}
		//skip the subtree of n leaf node combinations
		if err := sr.Skip(int(r.Right - r.Left)); err != nil {
			return nil, err
		}
		leafIndex += r.Right - r.Left
	}

	//always check the leafIndex of the tree.
	err = consumeUntil(math.MaxUint64)
	//if it is exceeded, this is not an error to be solved.
	if err == io.EOF {
		err = nil
	}
	return storageProofList, err
}

// GetRangeStorageProof get a proof of storage for a range of subtrees
func GetRangeStorageProof(proofStart, proofEnd int, h SubtreeRoot) (proof [][]byte, err error) {
	if proofStart < 0 || proofStart > proofEnd || proofStart == proofEnd {
		panic("GetRangeStorageProof: the parameter is invalid")
	}
	return getRangeStorageProof([]subTreeRange{{uint64(proofStart), uint64(proofEnd)}}, h)
}

// LeafRoot
type LeafRoot interface {
	//GetLeafRoot get the hash of the leaf node
	GetLeafRoot() ([]byte, error)
}

// ReaderLeafHasher
type LeafRootReader struct {
	r    io.Reader
	h    hash.Hash
	leaf []byte
}

// GetLeafRoot implements LeafRoot.
func (rlh *LeafRootReader) GetLeafRoot() ([]byte, error) {
	n, err := io.ReadFull(rlh.r, rlh.leaf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	} else if n == 0 {
		return nil, io.EOF
	}
	return leafTotal(rlh.h, rlh.leaf[:n]), nil
}

// NewLeafRootReader return LeafRootReader
func NewLeafRootReader(r io.Reader, h hash.Hash, leafSize int) *LeafRootReader {
	return &LeafRootReader{
		r:    r,
		h:    h,
		leaf: make([]byte, leafSize),
	}
}

// CachedLeafHasher
type LeafRootCached struct {
	leafRoots [][]byte
}

// GetLeafRoot implements LeafRoot.
func (clh *LeafRootCached) GetLeafRoot() ([]byte, error) {
	if len(clh.leafRoots) == 0 {
		return nil, io.EOF
	}
	h := clh.leafRoots[0]
	clh.leafRoots = clh.leafRoots[1:]
	return h, nil
}

// NewLeafRootCached return LeafRootCached
func NewLeafRootCached(leafHashes [][]byte) *LeafRootCached {
	return &LeafRootCached{
		leafRoots: leafHashes,
	}
}

// checkRangeStorageProof
func checkRangeStorageProof(lh LeafRoot, h hash.Hash, ranges []subTreeRange, storageProofList [][]byte, root []byte) (bool, error) {
	if len(ranges) == 0 {
		return true, nil
	}
	if !checkRangeList(ranges) {
		panic("checkRangeStorageProof: the parameter is invalid")
	}

	tree := NewTree(h)
	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end && len(storageProofList) > 0 {
			//get the size of the adjacent subtree
			subtreeSize := adjacentSubtreeSize(leafIndex, end)
			i := bits.TrailingZeros64(uint64(subtreeSize))
			//insert a subtree of the specified height
			if err := tree.PushSubTree(i, storageProofList[0]); err != nil {
				return err
			}
			storageProofList = storageProofList[1:]
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}

	for _, r := range ranges {
		if err := consumeUntil(r.Left); err != nil {
			return false, err
		}

		for i := r.Left; i < r.Right; i++ {
			//get the hash of the leaf node
			leafHash, err := lh.GetLeafRoot()
			if err != nil {
				return false, err
			}
			//insert a subtree of the specified height
			if err := tree.PushSubTree(0, leafHash); err != nil {
				panic(err)
			}
		}
		leafIndex += r.Right - r.Left
	}

	//always check the leafIndex of the tree.
	if err := consumeUntil(math.MaxUint64); err != nil {
		return false, err
	}

	return bytes.Equal(tree.Root(), root), nil
}

// CheckRangeStorageProof
func CheckRangeStorageProof(lh LeafRoot, h hash.Hash, left, right int, storageProofList [][]byte, root []byte) (bool, error) {
	if left < 0 || left > right || left == right {
		panic("CheckRangeStorageProof: the parameter is invalid")
	}
	return checkRangeStorageProof(lh, h, []subTreeRange{{uint64(left), uint64(right)}}, storageProofList, root)
}
