// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package merkle

import (
	"crypto/sha256"
	"testing"
)

func TestCachedTree_SetStorageProofIndex(t *testing.T) {
	cachedTree := NewCachedTree(sha256.New(), 0)
	err := cachedTree.SetStorageProofIndex(10)
	if err != nil {
		t.Error(err)
	}
	cachedTree.Tree.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2a"))
	cachedTree.Tree.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2b"))
	cachedTree.Tree.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2c"))
	err = cachedTree.SetStorageProofIndex(10)
	if err == nil {
		t.Error(err)
	}
}

func TestCachedTree_ProofList(t *testing.T) {
	cachedTree := NewCachedTree(sha256.New(), 1)
	err := cachedTree.SetStorageProofIndex(1)
	if err != nil {
		t.Error(err)
	}

	tree1 := NewTree(sha256.New())
	if err := tree1.SetStorageProofIndex(1); err != nil {
		t.Error(err)
	}
	tree1.PushLeaf([]byte("1"))
	tree1.PushLeaf([]byte("2"))
	cachedTree.Tree.PushLeaf(tree1.Root())

	tree2 := NewTree(sha256.New())
	tree2.PushLeaf([]byte("3"))
	tree2.PushLeaf([]byte("4"))
	cachedTree.Tree.PushLeaf(tree2.Root())

	_, treeProofList, _, _ := tree1.ProofList()
	_, storageProofList, index, number := cachedTree.ProofList(treeProofList)

	tree3 := NewTree(sha256.New())
	tree3.PushLeaf([]byte("1"))
	tree3.PushLeaf([]byte("2"))
	tree3.PushLeaf([]byte("3"))
	tree3.PushLeaf([]byte("4"))
	if !CheckStorageProof(cachedTree.hash, tree3.Root(), storageProofList, index, number) {
		t.Error("CheckStorageProof fail")
	}
}
