// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package merkle

import (
	"bytes"
	"hash"
	"io"
	"math/bits"
)

// GetDiffStorageProof proof of storage of merkle diff from the specified leaf interval
func GetDiffStorageProof(limits []SubTreeLimit, h SubtreeRoot, leafNumber uint64) (storageProofList [][]byte, err error) {

	if !checkLimitList(limits) {
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
			storageProofList = append(storageProofList, root)
			leafIndex += uint64(subtreeSize)
		}
		return nil
	}
	for _, r := range limits {
		if err := consumeUntil(r.Left); err != nil {
			return nil, err
		}
		if err := h.Skip(int(r.Right - r.Left)); err != nil {
			return nil, err
		}
		leafIndex += r.Right - r.Left
	}
	err = consumeUntil(leafNumber)
	if err == io.EOF {
		err = nil
	}
	return storageProofList, err
}

// CheckDiffStorageProof verify that the merkle diff is stored from the specified leaf interval.
func CheckDiffStorageProof(lh LeafRoot, leafNumber uint64, h hash.Hash, limits []SubTreeLimit, storageProofList [][]byte, root []byte) (bool, error) {

	if !checkLimitList(limits) {
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
	for _, r := range limits {
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
