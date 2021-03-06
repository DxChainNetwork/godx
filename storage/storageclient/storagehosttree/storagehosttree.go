// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

// StorageHostTree defined a binary tree structure that used to store all
// storage host information found by the storage client
type storageHostTree struct {
	root     *node
	hostPool map[enode.ID]*node
	lock     sync.Mutex
}

// New will initialize the StorageHostTree object
func New() StorageHostTree {
	return &storageHostTree{
		hostPool: make(map[enode.ID]*node),
		root: &node{
			count: 1,
		},
	}
}

// Insert will insert the StorageHost information into StorageHostTree
func (t *storageHostTree) Insert(hi storage.HostInfo, eval int64) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.insert(hi, eval)
}

// insert will format the storage host information into nodeEntry data type
// and then insert to the tree and update the host pool
func (t *storageHostTree) insert(hi storage.HostInfo, eval int64) error {
	// nodeEntry
	entry := &nodeEntry{
		HostInfo: hi,
		eval:     eval,
	}

	// validation: check if the storagehost exists already
	if _, exists := t.hostPool[hi.EnodeID]; exists {
		return ErrHostExists
	}

	// insert the noe entry into StorageHostTree
	_, node := t.root.nodeInsert(entry)

	// update hostPool
	t.hostPool[hi.EnodeID] = node

	return nil
}

// HostInfoUpdate updates the host information in in the tree based on the enode ID
func (t *storageHostTree) HostInfoUpdate(hi storage.HostInfo, eval int64) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// get the node
	n, exist := t.hostPool[hi.EnodeID]
	if !exist {
		return ErrHostNotExists
	}

	// remove the node from the tree
	n.nodeRemove()

	entry := &nodeEntry{
		HostInfo: hi,
		eval:     eval,
	}

	// insert node and update the hostPool
	_, node := t.root.nodeInsert(entry)
	t.hostPool[hi.EnodeID] = node

	return nil
}

// Remove will remove the node from the hostPool as well as
// making the node not occupied, updating the evaluation
func (t *storageHostTree) Remove(enodeID enode.ID) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	n, exists := t.hostPool[enodeID]
	if !exists {
		return ErrHostNotExists
	}

	// remove node and update the host pool
	n.nodeRemove()
	delete(t.hostPool, enodeID)

	return nil
}

// All will retrieve all host information stored in the tree
func (t *storageHostTree) All() (his []storage.HostInfo) {
	t.lock.Lock()
	defer t.lock.Unlock()

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

// RetrieveHostInfo will get storage host information and evaluation score from the tree based
// on the enode ID
func (t *storageHostTree) RetrieveHostInfo(enodeID enode.ID) (storage.HostInfo, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	node, exist := t.hostPool[enodeID]
	if !exist {
		return storage.HostInfo{}, false
	}

	return node.entry.HostInfo, true
}

// RetrieveHostEval retrieve the host evaluation score of the host with the give ID.
func (t *storageHostTree) RetrieveHostEval(enodeID enode.ID) (int64, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	node, exist := t.hostPool[enodeID]
	if !exist {
		return 0, false
	}
	return node.entry.eval, true
}

// SelectRandom will randomly select nodes from the storage host tree based
// on their evaluation. For any storage host's enode ID contained in the blacklist,
// the storage host cannot be selected. For any storage host's enode ID contained in the
// addrBlacklist, the address's ip network will have to be added into the filter, meaning
// the storage host with same ip network cannot be selected
//  	1. handle addrBlacklist
// 		2. handle blacklist
//      3. get needed storage hosts
//      4. restore storage host tree structure
// NOTE: the number of storage hosts information got may not satisfy the number of storage host
// information needed.
func (t *storageHostTree) SelectRandom(needed int, blacklist, addrBlacklist []enode.ID) []storage.HostInfo {
	t.lock.Lock()
	defer t.lock.Unlock()

	var removedNodeEntries []*nodeEntry
	filter := NewFilter()

	// 1. handle addrBlacklist
	for _, enodeID := range addrBlacklist {
		// TODO: test the functionality of blacklist
		node, exists := t.hostPool[enodeID]
		if !exists {
			continue
		}
		filter.Add(node.entry.HostInfo.IP)
	}

	// 2. handle blacklist
	for _, enodeID := range blacklist {
		// TODO: test the functionality of blacklist
		node, exists := t.hostPool[enodeID]
		if !exists {
			continue
		}

		node.nodeRemove()
		delete(t.hostPool, enodeID)

		removedNodeEntries = append(removedNodeEntries, node.entry)
	}

	// 3. get needed storage hosts information
	var storageHosts []storage.HostInfo
	for len(t.hostPool) > 0 && len(storageHosts) < needed {
		// in case the evaluation is negative, the random will return error
		// however, this should never happen
		if t.root.evalTotal < 0 {
			break
		}

		randEval := r.Int63n(t.root.evalTotal)

		node, err := t.root.nodeWithEval(randEval)

		// TODO (mzhang): better error handling
		if err != nil {
			break
		}

		// node validation
		//   1. must accept contract
		//   2. must be scanned at least once
		//   3. the latest scan must be success
		//   4. ip network should not be the same as once contained in the address blacklist
		if node.entry.AcceptingContracts &&
			len(node.entry.ScanRecords) > 0 &&
			node.entry.ScanRecords[len(node.entry.ScanRecords)-1].Success &&
			!filter.Filtered(node.entry.IP) {
			storageHosts = append(storageHosts, node.entry.HostInfo)
			filter.Add(node.entry.IP)
		}

		// remove the node
		node.nodeRemove()
		delete(t.hostPool, node.entry.EnodeID)
		removedNodeEntries = append(removedNodeEntries, node.entry)
	}

	// 4. restore storage host tree structure
	for _, entry := range removedNodeEntries {
		_, node := t.root.nodeInsert(entry)
		t.hostPool[node.entry.EnodeID] = node
	}

	return storageHosts
}
