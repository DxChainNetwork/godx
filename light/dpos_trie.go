// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package light

import (
	"context"
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/trie"
)

// NewDposContext creates a new dposContext
func NewDposContext(ctx context.Context, header *types.Header, odr OdrBackend) *types.DposContext {
	db := NewOdrDposDatabase(ctx, header, odr)
	dposCtx, _ := types.NewDposContextFromRoot(db, header.DposContext)
	return dposCtx
}

type dposDatabaseID struct {
	blockHash   common.Hash
	blockNumber uint64
	roots       types.DposContextRoot
}

type dposOdrDatabase struct {
	ctx     context.Context
	id      *dposDatabaseID
	backend OdrBackend
}

// NewOdrDposDatabase creates a new odr dpos database
func NewOdrDposDatabase(ctx context.Context, header *types.Header, odr OdrBackend) *dposOdrDatabase {
	return &dposOdrDatabase{
		ctx:     ctx,
		id:      DposDatabaseIDFromHeader(header),
		backend: odr,
	}
}

// DposDatabaseIDFromHeader returns a DposTrieID from a block header
func DposDatabaseIDFromHeader(header *types.Header) *dposDatabaseID {
	return &dposDatabaseID{
		blockHash:   header.Hash(),
		blockNumber: header.Number.Uint64(),
		roots:       *header.DposContext.Copy(),
	}
}

// dposDatabaseIDToTrieID convert a dposDatabaseID to DposTrieID
func dposDatabaseIDToTrieID(id *dposDatabaseID, spec DposTrieSpecifier) *DposTrieID {
	return &DposTrieID{
		BlockHash:   id.blockHash,
		BlockNumber: id.blockNumber,
		Roots:       id.roots,
		TrieSpec:    spec,
	}
}

// OpenEpochTrie opens the epoch trie
func (db *dposOdrDatabase) OpenEpochTrie(root common.Hash) (types.DposTrie, error) {
	return db.openTrie(EpochTrieSpec)
}

// OpenDelegateTrie open the delegate trie
func (db *dposOdrDatabase) OpenDelegateTrie(root common.Hash) (types.DposTrie, error) {
	return db.openTrie(DelegateTrieSpec)
}

// OpenLastEpochTrie open the epoch trie in the last epoch
func (db *dposOdrDatabase) OpenLastDelegateTrie(root common.Hash) (types.DposTrie, error) {
	return nil, errors.New("light node does not support accumulate reward")
}

// OpenVoteTrie open the vote trie
func (db *dposOdrDatabase) OpenVoteTrie(root common.Hash) (types.DposTrie, error) {
	return db.openTrie(VoteTrieSpec)
}

// OpenCandidateTrie opens the candidate trie
func (db *dposOdrDatabase) OpenCandidateTrie(root common.Hash) (types.DposTrie, error) {
	return db.openTrie(CandidateTrieSpec)
}

// OpenMinedCntTrie opens the mined count trie
func (db *dposOdrDatabase) OpenMinedCntTrie(root common.Hash) (types.DposTrie, error) {
	return db.openTrie(MinedCntTrieSpec)
}

func (db *dposOdrDatabase) openTrie(spec DposTrieSpecifier) (types.DposTrie, error) {
	id := dposDatabaseIDToTrieID(db.id, spec)
	return &odrDposTrie{db: db, id: id}, nil
}

// CopyTrie copies a dpos trie
func (db *dposOdrDatabase) CopyTrie(t types.DposTrie) types.DposTrie {
	switch t := t.(type) {
	case *odrDposTrie:
		cpy := &odrDposTrie{db: t.db, id: t.id}
		if t.trie != nil {
			cpyTrie := *t.trie
			cpy.trie = &cpyTrie
		}
		return cpy
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// odrDposTrie is the trie data structure for light node which sits on top of lesOdrBackend that
// helps retrieve trie data from the les server.
type odrDposTrie struct {
	db   *dposOdrDatabase
	id   *DposTrieID
	trie *trie.Trie
}

// TryGet try to retrieve the value for the key
func (t *odrDposTrie) TryGet(key []byte) ([]byte, error) {
	var res []byte
	err := t.do(key, func() (err error) {
		res, err = t.trie.TryGet(key)
		return err
	})
	return res, err
}

// TryUpdate try to update the key value pair in the trie
func (t *odrDposTrie) TryUpdate(key, value []byte) error {
	return t.do(key, func() error {
		return t.trie.TryUpdate(key, value)
	})
}

// TryDelete try to delete the key in the trie
func (t *odrDposTrie) TryDelete(key []byte) error {
	return t.do(key, func() error {
		return t.trie.TryDelete(key)
	})
}

// Commit commit the trie to trie db
func (t *odrDposTrie) Commit(onleaf trie.LeafCallback) (common.Hash, error) {
	if t.trie == nil {
		return t.id.TargetRoot(), nil
	}
	return t.trie.Commit(onleaf)
}

// Hash returns the hash of the trie
func (t *odrDposTrie) Hash() common.Hash {
	if t.trie == nil {
		return t.id.TargetRoot()
	}
	return t.trie.Hash()
}

// NodeIterator return the node iterator of the odrDposTrie
func (t *odrDposTrie) NodeIterator(startKey []byte) trie.NodeIterator {
	return newDposNodeIterator(t, startKey)
}

// PrefixIterator return the prefix iterator of the given prefix for the odrDposTrie.
// Note that
func (t *odrDposTrie) PrefixIterator(prefix []byte) trie.NodeIterator {
	return newDposPrefixIterator(t, prefix)
}

// Prove put the proof of the specified key to proofDb
func (t *odrDposTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.Putter) error {
	return errors.New("not implemented")
}

// do tries and retries to execute a function until it returns with no error or
// an error type other than MissingNodeError
func (t *odrDposTrie) do(key []byte, fn func() error) error {
	for {
		var err error
		if t.trie == nil {
			t.trie, err = trie.New(t.id.TargetRoot(), trie.NewDatabase(t.db.backend.Database()))
		}
		if err == nil {
			err = fn()
		}
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
		r := &DposTrieRequest{ID: t.id, Key: key}
		if err := t.db.backend.Retrieve(t.db.ctx, r); err != nil {
			return err
		}
	}
}

type dposNodeIterator struct {
	trie.NodeIterator
	t   *odrDposTrie
	err error
}

// newDposPrefixIterator creates a dposNodeIterator which iterates over the given prefix
// for the give odrDposTrie
func newDposPrefixIterator(t *odrDposTrie, prefix []byte) trie.NodeIterator {
	it := &dposNodeIterator{t: t}
	if t.trie == nil {
		it.do(func() error {
			t, err := trie.New(t.id.TargetRoot(), trie.NewDatabase(t.db.backend.Database()))
			if err == nil {
				it.t.trie = t
			}
			return err
		})
	}
	it.do(func() error {
		it.NodeIterator = it.t.trie.PrefixIterator(prefix)
		return it.NodeIterator.Error()
	})
	return it
}

// newDposNodeIterator creates a dposNodeIterator which iterates from the given key
func newDposNodeIterator(t *odrDposTrie, start []byte) trie.NodeIterator {
	it := &dposNodeIterator{t: t}
	if t.trie == nil {
		it.do(func() error {
			t, err := trie.New(t.id.TargetRoot(), trie.NewDatabase(t.db.backend.Database()))
			if err == nil {
				it.t.trie = t
			}
			return err
		})
	}
	it.do(func() error {
		it.NodeIterator = it.t.trie.NodeIterator(start)
		return it.NodeIterator.Error()
	})
	return it
}

// Next iterate to the next entry.
func (it *dposNodeIterator) Next(descend bool) bool {
	var ok bool
	it.do(func() error {
		ok = it.NodeIterator.Next(descend)
		return it.NodeIterator.Error()
	})
	return ok
}

func (it *dposNodeIterator) do(fn func() error) {
	var lastHash common.Hash
	for {
		it.err = fn()
		missing, ok := it.err.(*trie.MissingNodeError)
		if !ok {
			return
		}
		if missing.NodeHash == lastHash {
			it.err = fmt.Errorf("retrieve loop for trie node %x", missing.NodeHash)
			return
		}
		lastHash = missing.NodeHash
		r := &DposTrieRequest{ID: it.t.id, Key: nibblesToKey(missing.Path)}
		if it.err = it.t.db.backend.Retrieve(it.t.db.ctx, r); it.err != nil {
			return
		}
	}
}

// Error return the error for the node iterator
func (it *dposNodeIterator) Error() error {
	if it.err != nil {
		return it.err
	}
	return it.NodeIterator.Error()
}
