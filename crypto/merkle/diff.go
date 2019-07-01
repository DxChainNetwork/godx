package merkle

import (
	"bytes"
	"hash"
	"io"
	"math/bits"
)

// GetDiffStorageProof proof of storage of merkle diff from the specified leaf interval
func GetDiffStorageProof(ranges []subTreeRange, h SubtreeRoot, numLeaves uint64) (proof [][]byte, err error) {

	if !checkRangeList(ranges) {
		panic("GetDiffStorageProof: the parameter is invalid")
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
	err = consumeUntil(numLeaves)
	if err == io.EOF {
		err = nil
	}
	return proof, err
}

// CheckDiffStorageProof verify that the merkle diff is stored from the specified leaf interval.
func CheckDiffStorageProof(lh LeafRoot, leafNumber uint64, h hash.Hash, ranges []subTreeRange, storageProofList [][]byte, root []byte) (bool, error) {

	if !checkRangeList(ranges) {
		panic("CheckDiffStorageProof: the parameter is invalid")
	}
	tree := NewTree(h)
	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end && len(storageProofList) > 0 {
			subtreeSize := adjacentSubtreeSize(leafIndex, end)
			i := bits.TrailingZeros64(uint64(subtreeSize))
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
			leafHash, err := lh.GetLeafRoot()
			if err != nil {
				return false, err
			}
			if err := tree.PushSubTree(0, leafHash); err != nil {
				panic(err)
			}
		}
		leafIndex += r.Right - r.Left
	}
	err := consumeUntil(leafNumber)
	return bytes.Equal(tree.Root(), root), err
}
