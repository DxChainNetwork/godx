// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/common"
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

// RetrieveHostInfo will get storage host information from the tree based on the
// public key
func (t *StorageHostTree) RetrieveHostInfo(pubkey string) (storage.HostInfo, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	node, exist := t.hostPool[pubkey]
	if !exist {
		return storage.HostInfo{}, false
	}

	return node.entry.HostInfo, true
}

// SetEvaluationFunc will re-assign evaluation function for calculating
// storage host evaluation.
func (t *StorageHostTree) SetEvaluationFunc(ef EvaluationFunc) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// if there is no host exist, set the function and return directly
	if len(t.hostPool) == 0 {
		t.evalFunc = ef
		return nil
	}

	// if not, get all hosts information and reset the tree
	hostinfos := t.all()

	t.root = &node{
		count: 1,
	}
	t.hostPool = make(map[string]*node)
	t.evalFunc = ef

	// re-insert the host information
	var errs error
	for _, hostinfo := range hostinfos {
		err := t.insert(hostinfo)
		if err != nil {
			errs = common.ErrCompose(errs, err)
		}
	}
	return errs
}

// SelectRandom will randomly select nodes from the storage host tree based
// on their evaluation. For any storage host's public key contained in the blacklist,
// the storage host cannot be selected. For any storage host's public contained in the
// addrBlacklist, the address's ip network will have to be added into the filter, meaning
// the storage host with same ip network cannot be selected
//  	1. handle addrBlacklist
// 		2. handle blacklist
//      3. get needed storage hosts
//      4. restore storage host tree structure
// NOTE: the number of storage hosts information got may not satisfy the number of storage host
// information needed.
func (t *StorageHostTree) SelectRandom(needed int, blacklist, addrBlacklist []string) []storage.HostInfo {
	t.lock.Lock()
	defer t.lock.Unlock()

	var removedNodeEntries []*nodeEntry
	filter := NewFilter()

	// 1. handle addrBlacklist
	for _, pubkey := range addrBlacklist {
		node, exists := t.hostPool[pubkey]
		if !exists {
			continue
		}
		filter.Add(node.entry.HostInfo.IP)
	}

	// 2. handle blacklist
	for _, pubkey := range blacklist {
		node, exists := t.hostPool[pubkey]
		if !exists {
			continue
		}

		node.nodeRemove()
		delete(t.hostPool, pubkey)

		removedNodeEntries = append(removedNodeEntries, node.entry)
	}

	// 3. get needed storage hosts information
	var storageHosts []storage.HostInfo
	for len(t.hostPool) > 0 && len(storageHosts) < needed {
		// in case the evaluation is negative, the random will return error
		// however, this should never happen
		if t.root.evalTotal.IsNeg() {
			break
		}

		randEval, err := common.RandomBigInt(t.root.evalTotal)

		if err != nil {
			break
		}
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
		delete(t.hostPool, node.entry.PublicKey)
		removedNodeEntries = append(removedNodeEntries, node.entry)
	}

	// 4. restore storage host tree structure
	for _, entry := range removedNodeEntries {
		_, node := t.root.nodeInsert(entry)
		t.hostPool[node.entry.PublicKey] = node
	}

	return storageHosts
}
