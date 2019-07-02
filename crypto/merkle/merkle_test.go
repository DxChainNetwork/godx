// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package merkle

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestTree_SetStorageProofIndex(t *testing.T) {
	tree1 := NewTree(sha256.New())
	err := tree1.SetStorageProofIndex(10)
	if err != nil {
		t.Error(err)
	}
	tree2 := NewTree(sha256.New())
	tree2.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2a"))
	tree2.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2b"))
	tree2.PushLeaf([]byte("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2c"))
	err = tree2.SetStorageProofIndex(10)
	if err == nil {
		t.Error("this should return error")
	}
}

func TestTree_Root(t *testing.T) {
	tree := NewTree(sha256.New())
	tree.usedAsCached = true
	for i := 0; i < 6; i++ {
		tree.PushLeaf([]byte(fmt.Sprintf("%d", i)))
		treeRoot := tree.Root()
		switch i {
		case 0:
			if !bytes.Equal(treeRoot, tree.top.sum) {
				t.Error("error,id:1")
			}
		case 1:
			if !bytes.Equal(treeRoot, tree.top.sum) {
				t.Error("error,id:2")
			}
		case 2:
			sum := dataTotal(tree.hash, tree.top.next.sum, tree.top.sum)
			if !bytes.Equal(treeRoot, sum) {
				t.Error("error,id:3")
			}
		case 3:
			if !bytes.Equal(treeRoot, tree.top.sum) {
				t.Error("error,id:4")
			}
		case 4:
			sum := dataTotal(tree.hash, tree.top.next.sum, tree.top.sum)
			if !bytes.Equal(treeRoot, sum) {
				t.Error("error,id:5")
			}
		case 5:
			sum := dataTotal(tree.hash, tree.top.next.sum, tree.top.sum)
			if !bytes.Equal(treeRoot, sum) {
				t.Error("error,id:6")
			}
		}

	}
}

func TestTree_ProofList(t *testing.T) {
	tree := NewTree(sha256.New())

	if err := tree.SetStorageProofIndex(2); err != nil {
		t.Error(err)
	}

	root := tree.Root()
	if root != nil {
		t.Error("root must be empty")
	}

	tree.PushLeaf([]byte("1"))
	tree.PushLeaf([]byte("2"))

	_, storageProofList2, _, _ := tree.ProofList()
	if len(storageProofList2) != 0 {
		t.Error("storageProofList must be empty")
	}

	tree.PushLeaf([]byte("3"))
	_, storageProofList3, _, _ := tree.ProofList()
	if len(storageProofList3) != 2 {
		t.Error("The length of storageProofList should be one")
	}
	tree.PushLeaf([]byte("4"))
	_, storageProofList4, _, _ := tree.ProofList()
	if len(storageProofList4) != 3 {
		t.Error("The length of storageProofList should be two")
	}
	tree.PushLeaf([]byte("5"))
	_, storageProofList5, _, _ := tree.ProofList()
	if len(storageProofList5) != 4 {
		t.Error("The length of storageProofList should be two")
	}

	tree.PushLeaf([]byte("6"))
	_, storageProofList6, _, _ := tree.ProofList()
	if len(storageProofList6) != 4 {
		t.Error("The length of storageProofList should be two")
	}

	tree.PushLeaf([]byte("7"))
	_, storageProofList7, _, _ := tree.ProofList()
	if len(storageProofList7) != 4 {
		t.Error("The length of storageProofList should be two")
	}

	tree.PushLeaf([]byte("8"))
	_, storageProofList8, _, _ := tree.ProofList()
	if len(storageProofList8) != 4 {
		t.Error("The length of storageProofList should be two")
	}

}

func TestCheckStorageProof(t *testing.T) {
	tree := NewTree(sha256.New())

	if err := tree.SetStorageProofIndex(2); err != nil {
		t.Error(err)
	}

	root := tree.Root()
	if root != nil {
		t.Error("root must be empty")
	}
	tree.PushLeaf([]byte("1"))
	tree.PushLeaf([]byte("2"))
	tree.PushLeaf([]byte("3"))
	root3, list3, _, _ := tree.ProofList()
	if !CheckStorageProof(tree.hash, root3, list3, tree.storageProofIndex, tree.leafIndex) {
		t.Error("Check failed")
	}
	tree.PushLeaf([]byte("4"))
	root4, list4, _, _ := tree.ProofList()
	if !CheckStorageProof(tree.hash, root4, list4, tree.storageProofIndex, tree.leafIndex) {
		t.Error("Check failed")
	}
	tree.PushLeaf([]byte("5"))
	tree.PushLeaf([]byte("6"))
	tree.PushLeaf([]byte("7"))
	tree.PushLeaf([]byte("8"))
	root8, list8, _, _ := tree.ProofList()
	if !CheckStorageProof(tree.hash, root8, list8, tree.storageProofIndex, tree.leafIndex) {
		t.Error("Check failed")
	}

}
