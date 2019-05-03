// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/storage"
	"sort"
	"sync"
)

type StorageHostTree struct {
	root     *node
	hostPool map[string]*node
	evalFunc EvaluationFunc
	lock     sync.Mutex
}

func New(ef EvaluationFunc) *StorageHostTree {
	return &StorageHostTree{
		hostPool: make(map[string]*node),
		root: &node{
			count: 1,
		},
		evalFunc: ef,
	}
}

// Insert will insert the StorageHost information into StorageHostTree
func (t *StorageHostTree) Insert(hi storage.HostInfo) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.insert(hi)
}

func (t *StorageHostTree) insert(hi storage.HostInfo) error {
	// nodeEntry
	entry := &nodeEntry{
		HostInfo: hi,
		eval:     t.evalFunc(hi).Evaluation(),
	}

	// validation: check if the storagehost exists already
	if _, exists := t.hostPool[hi.PublicKey]; exists {
		return ErrHostExists
	}

	// insert the noe entry into StorageHostTree
	_, node := t.root.nodeInsert(entry)

	// update hostPool
	t.hostPool[hi.PublicKey] = node

	return nil
}

// HostInfoUpdate updates the host information in in the tree based on the public key
func (t *StorageHostTree) HostInfoUpdate(hi storage.HostInfo) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// get the node
	n, exist := t.hostPool[hi.PublicKey]
	if !exist {
		return ErrHostNotExists
	}

	// remove the node from the tree
	n.nodeRemove()

	entry := &nodeEntry{
		HostInfo: hi,
		eval:     t.evalFunc(hi).Evaluation(),
	}

	// insert node and update the hostPool
	_, node := n.nodeInsert(entry)
	t.hostPool[hi.PublicKey] = node

	return nil
}

// Remove will remove the node from the hostPool as well as
// making the node unocuppied, updating the evaluation
func (t *StorageHostTree) Remove(pubkey string) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// TODO (mzhang): verify if the public key is string with HZ
	n, exists := t.hostPool[pubkey]
	if !exists {
		return ErrHostNotExists
	}

	// remove node and update the host pool
	n.nodeRemove()
	delete(t.hostPool, pubkey)

	return nil
}

// All will retrieve all host information stored in the tree
func (t *StorageHostTree) All() []storage.HostInfo {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.all()
}

func (t *StorageHostTree) all() (his []storage.HostInfo) {
	// collect all node entries
	var entries []nodeEntry
	for _, node := range t.hostPool {
		entries = append(entries, *node.entry)
	}

	// sort based on the evaluation
	sort.Sort(nodeEntries(entries))

	// get all host information
	for _, entry := range entries {
		his = append(his, entry.HostInfo)
	}

	return
}
