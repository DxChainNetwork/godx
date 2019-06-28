package merkle

import (
	"bytes"
	"hash"
	"io"
	"math/bits"
)

// BuildDiffProof
func BuildDiffProof(ranges []LeafRange, h SubtreeHasher, numLeaves uint64) (proof [][]byte, err error) {

	if !validRangeSet(ranges) {
		panic("BuildDiffProof: illegal set of proof ranges")
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
	err = consumeUntil(numLeaves)
	if err == io.EOF {
		err = nil
	}
	return proof, err
}

// VerifyDiffProof
func VerifyDiffProof(lh LeafHasher, numLeaves uint64, h hash.Hash, ranges []LeafRange, proof [][]byte, root []byte) (bool, error) {

	if !validRangeSet(ranges) {
		panic("VerifyDiffProof: illegal set of proof ranges")
	}
	tree := NewTree(h)
	var leafIndex uint64
	consumeUntil := func(end uint64) error {
		for leafIndex != end && len(proof) > 0 {
			subtreeSize := nextSubtreeSize(leafIndex, end)
			i := bits.TrailingZeros64(uint64(subtreeSize))
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
	err := consumeUntil(numLeaves)
	return bytes.Equal(tree.Root(), root), err
}
