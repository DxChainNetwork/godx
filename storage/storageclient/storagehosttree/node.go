// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// node defines the storage host tree node
type node struct {
	parent *node
	left   *node
	right  *node

	// count includes the amount of storage hosts including the sum of all its' children
	// and the node itself
	count int

	// indicates if the node contained any storage host information
	occupied bool

	// total evaluation of the storage hosts, including the sum of all its' child's
	// evaluation and the evaluation of node it self
	evalTotal common.BigInt
	entry     *nodeEntry
}

// nodeEntry is the information stored in a storage host tree node
type nodeEntry struct {
	storage.HostInfo
	eval common.BigInt
}

// nodeEntries defines a collection of node entry that implemented the sorting methods
// the sorting will be ranked from the higher evaluation to lower evaluation
type nodeEntries []nodeEntry

// the storage host with higher weight will placed in the front of the list
func (ne nodeEntries) Len() int           { return len(ne) }
func (ne nodeEntries) Less(i, j int) bool { return ne[i].eval.Cmp(ne[j].eval) > 0 }
func (ne nodeEntries) Swap(i, j int)      { ne[i], ne[j] = ne[j], ne[i] }

// newNode will create and initialize a new node object, which will be inserted into
// the StorageHostTree
func newNode(parent *node, entry *nodeEntry) *node {
	return &node{
		parent:    parent,
		occupied:  true,
		evalTotal: entry.eval,
		count:     1,
		entry:     entry,
	}
}

// nodeRemove will not remove the actual node from the tree
// instead, it update the evaluation, and occupied status
func (n *node) nodeRemove() {
	n.evalTotal = n.evalTotal.Sub(n.entry.eval)
	n.occupied = false
	parent := n.parent
	for parent != nil {
		parent.evalTotal = parent.evalTotal.Sub(n.entry.eval)
		parent = parent.parent
	}
}

// nodeInsert will insert the node entry into the StorageHostTree
func (n *node) nodeInsert(entry *nodeEntry) (nodesAdded int, nodeInserted *node) {
	// 1. check if the node is root node
	if n.parent == nil && !n.occupied && n.left == nil && n.right == nil {
		n.occupied = true
		n.entry = entry
		n.evalTotal = entry.eval

		nodesAdded = 0
		nodeInserted = n
		return
	}

	// 2. add all child evaluation
	n.evalTotal = n.evalTotal.Add(entry.eval)

	// 3. check if the node is occupied
	if !n.occupied {
		n.occupied = true
		n.entry = entry

		nodesAdded = 0
		nodeInserted = n
		return nodesAdded, nodeInserted
	}

	// 4. insert new node, binary tree
	if n.left == nil {
		n.left = newNode(n, entry)
		nodesAdded = 1
		nodeInserted = n.left
	} else if n.right == nil {
		n.right = newNode(n, entry)
		nodesAdded = 1
		nodeInserted = n.right
	} else if n.left.count <= n.right.count {
		nodesAdded, nodeInserted = n.left.nodeInsert(entry)
	} else {
		nodesAdded, nodeInserted = n.right.nodeInsert(entry)
	}

	// 5. update the node count
	n.count += nodesAdded

	return
}

// nodeWithEval will retrieve node with the specific evaluation
func (n *node) nodeWithEval(eval common.BigInt) (*node, error) {
	if eval.Cmp(n.evalTotal) > 0 {
		return nil, ErrEvaluationTooLarge
	}

	if n.left != nil {
		if eval.Cmp(n.left.evalTotal) < 0 {
			return n.left.nodeWithEval(eval)
		}
		eval = eval.Sub(n.left.evalTotal)
	}
	if n.right != nil && eval.Cmp(n.right.evalTotal) < 0 {
		return n.right.nodeWithEval(eval)
	}

	if !n.occupied {
		return nil, ErrNodeNotOccupied
	}

	return n, nil
}
