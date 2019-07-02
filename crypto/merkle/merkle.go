// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package merkle

import (
	"bytes"
	"hash"

	"github.com/pkg/errors"
)

var (
	// prefixes used during hashing, as specified by RFC 6962
	leafPrefix = []byte{0x00}
	dataPrefix = []byte{0x01}
)

type subtree struct {
	next   *subtree
	height int
	sum    []byte
}

// Tree the most basic structure of the merkle tree
type Tree struct {
	top               *subtree
	hash              hash.Hash
	leafIndex         uint64
	storageProofIndex uint64
	storageProofList  [][]byte
	usedAsProof       bool
	usedAsCached      bool
}

// NewTree return a tree
func NewTree(h hash.Hash) *Tree {
	return &Tree{
		hash: h,
	}
}

// SetStorageProofIndex must be called on an empty tree.
func (t *Tree) SetStorageProofIndex(i uint64) error {
	if t.top != nil {
		return errors.New("must be called on an empty tree")
	}
	t.usedAsProof = true
	t.storageProofIndex = i
	return nil
}

// Root traverse all subtrees and calculate the sum of the hashes
func (t *Tree) Root() []byte {
	//top cannot be empty
	if t.top == nil {
		return nil
	}

	current := t.top
	//If the next of top is not empty, the whole
	//merkle tree is not balanced, you need to calculate the hash manually.
	for current.next != nil {
		current = &subtree{
			next:   current.next.next,
			height: current.next.height + 1,
			sum:    dataTotal(t.hash, current.next.sum, current.sum),
		}
	}
	//prevent internal data from being tampered with
	return append(current.sum[:0:0], current.sum...)
}

// ProofList construct a storage proof result set for
// the merkle tree that has established the storage proof index
func (t *Tree) ProofList() (merkleRoot []byte, storageProofList [][]byte, storageProofIndex uint64, numLeaves uint64) {
	//must be a merkle tree that needs to build a proof of storage
	if !t.usedAsProof {
		panic("must be a merkle tree that needs to build a proof of storage")
	}

	//have not yet reached the storage certificate index
	if t.top == nil || len(t.storageProofList) == 0 {
		return t.Root(), nil, t.storageProofIndex, t.leafIndex
	}
	storageProofList = t.storageProofList

	current := t.top

	//If there are subtrees that are not combined,
	//then combine them into a higher subtree
	for current.next != nil && current.next.height < len(storageProofList)-1 {
		current = &subtree{
			next:   current.next.next,
			height: current.next.height + 1,
			sum:    dataTotal(t.hash, current.next.sum, current.sum),
		}
	}

	//If the current top does not contain a storage proof index,
	//then its next will necessarily contain the storage proof index.
	if current.next != nil && current.next.height == len(storageProofList)-1 {
		storageProofList = append(storageProofList, current.sum)
		current = current.next
	}

	current = current.next
	//If the next subtree is not empty, return them
	for current != nil {
		storageProofList = append(storageProofList, current.sum)
		current = current.next
	}
	return t.Root(), storageProofList, t.storageProofIndex, t.leafIndex
}

// PushLeaf the tree only saves the subtrees needed to calculate the merkle root.
// the process of the push will also include the path required for the storage certificate.
func (t *Tree) PushLeaf(data []byte) {

	// the first element of storageProofList is the element under storageProofIndex
	if t.leafIndex == t.storageProofIndex {
		t.storageProofList = append(t.storageProofList, data)
	}

	//insert a leaf node
	t.top = &subtree{
		next:   t.top,
		height: 0,
	}

	//whether to cache data, not cache data, only save data hash
	if t.usedAsCached {
		t.top.sum = data
	} else {
		t.top.sum = leafTotal(t.hash, data)
	}

	//combine all subtrees of the same height.
	t.combineAllSubTrees()

	// Update the index.
	t.leafIndex++
}

//PushSubTree there is no way to judge whether the
// inserted subtree is a balanced tree, which will bring unknown danger.
func (t *Tree) PushSubTree(height int, sum []byte) error {
	//Calculate the new index
	newIndex := t.leafIndex + 1<<uint64(height)

	//If the inserted subtree contains the elements
	//of storageProofIndex. this will lose all meaning.
	//if the leafIndex is already equal to the storageProofIndex,
	//then the next leaf will be added to the storageProofList,
	//the current subtree must already contain the leaf
	//corresponding to the storageProofIndex.
	//which is very dangerous.
	if t.usedAsProof && (t.leafIndex == t.storageProofIndex ||
		(t.leafIndex < t.storageProofIndex && t.storageProofIndex < newIndex)) {
		return errors.New("which is very dangerous,this will lose all meaning")
	}

	//When the tree at this time is not balanced and
	// the inserted subtree height is greater than
	// the current tree, this will not be allowed.
	if t.top != nil && height > t.top.height {
		return errors.New("subtrees that are not allowed to be inserted are larger than the current subtree")
	}

	t.top = &subtree{
		height: height,
		next:   t.top,
		sum:    sum,
	}

	//combine all subtrees of the same height.
	t.combineAllSubTrees()

	t.leafIndex = newIndex

	return nil
}

// combineAllSubTrees combine all subtrees of the same height
func (t *Tree) combineAllSubTrees() {
	//subtrees of the same height are combined into a higher subtree
	for t.top.next != nil && t.top.height == t.top.next.height {

		//check if the subtree can be added to the storageProofList
		if t.top.height == len(t.storageProofList)-1 {
			//one of the two subtrees being combined will be added to the storageProofList
			leaves := uint64(1 << uint(t.top.height))
			mid := (t.leafIndex / leaves) * leaves
			if t.storageProofIndex < mid {
				t.storageProofList = append(t.storageProofList, t.top.sum)
			} else {
				t.storageProofList = append(t.storageProofList, t.top.next.sum)
			}
		}

		//combine two subtrees
		t.top = &subtree{
			next:   t.top.next.next,
			height: t.top.next.height + 1,
			sum:    dataTotal(t.hash, t.top.next.sum, t.top.sum),
		}
	}
}

// leafTotal calculate leaf hash
func leafTotal(h hash.Hash, data []byte) []byte {
	return sum(h, leafPrefix, data)
}

// dataTotal calculate the root hash through the left and right subtrees
func dataTotal(h hash.Hash, a, b []byte) []byte {
	return sum(h, dataPrefix, a, b)
}

// sum calculate and return a hash through the specified interface
func sum(h hash.Hash, data ...[]byte) []byte {
	// Reset resets the Hash to its initial state.
	h.Reset()
	for _, d := range data {
		// ignore the error here because it won't return an error
		h.Write(d)
	}

	return h.Sum(nil)
}

// CheckStorageProof check the merkle tree
func CheckStorageProof(h hash.Hash, merkleRoot []byte, storageProofList [][]byte, storageProofIndex uint64, number uint64) bool {

	//invalid parameter
	if merkleRoot == nil || storageProofIndex >= number || len(storageProofList) <= 0 {
		return false
	}

	height := 0
	//It is possible to cache the data,
	//first calculate the hash of the first element
	sum := leafTotal(h, storageProofList[height])
	height++

	stableEnd := storageProofIndex
	for {

		//check if the merkle tree is complete
		subTreeStartIndex := (storageProofIndex / (1 << uint(height))) * (1 << uint(height))
		subTreeEndIndex := subTreeStartIndex + (1 << (uint(height))) - 1

		if subTreeEndIndex >= number {
			//explain that the merkle tree is not in a balanced state.
			break
		}
		stableEnd = subTreeEndIndex

		//the length of the storageProofList must be
		// greater than the height of the merkle tree.
		if len(storageProofList) <= height {
			return false
		}

		//determine if the leaf is left or right
		if storageProofIndex-subTreeStartIndex < 1<<uint(height-1) {
			sum = dataTotal(h, sum, storageProofList[height])
		} else {
			sum = dataTotal(h, storageProofList[height], sum)
		}
		height++
	}

	//if there is an extra unbalanced leaf,
	//calculate the sum of the hashes
	if stableEnd != number-1 {
		if len(storageProofList) <= height {
			return false
		}
		sum = dataTotal(h, sum, storageProofList[height])
		height++
	}

	//calculate the sum of the hashes of the remaining elements of the storageProofList
	for height < len(storageProofList) {
		sum = dataTotal(h, storageProofList[height], sum)
		height++
	}

	//return true if the two elements are the same
	if bytes.Equal(sum, merkleRoot) {
		return true
	}
	return false
}
