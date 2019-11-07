// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package light

import (
	"context"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/trie"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

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

func NewOdrDposDatabase(ctx context.Context, header *types.Header, odr OdrBackend) *dposOdrDatabase {
	return &dposOdrDatabase{
		ctx:     ctx,
		id:      DposDatabaseIDFromHeader(header),
		backend: odr,
	}
}

// DposTrieIDFromHeader returns a DposTrieID from a block header
func DposDatabaseIDFromHeader(header *types.Header, trieSpec DposTrieSpecifier) *dposDatabaseID {
	return &dposDatabaseID{
		blockHash:   header.Hash(),
		blockNumber: header.Number.Uint64(),
		roots:       *header.DposContext.Copy(),
	}
}

// dposDatabaseIDToTrieID convert a dposDatabaseID to DposTrieID
func dposDatabaseIDToTrieID(id *dposDatabaseID, spec DposTrieSpecifier) *DposTrieID {
	return &DposTrieID{
		BlockHash: id.blockHash,
		BlockNumber: id.blockNumber,
		Roots: id.roots,
		TrieSpec: spec,
	}
}

// OpenEpochTrie opens the epoch trie
func (db *dposOdrDatabase) OpenEpochTrie(root common.Hash) (*odrDposTrie, error) {
	id := dposDatabaseIDToTrieID(db.id, EpochTrieSpec)
	return &odrDposTrie {
		db: db,
		id: id,
	}, nil
}

// OpenLastEpochTrie open the epoch trie in the last epoch
func (db *dposOdrDatabase) OpenLastEpochTrie(root common.Hash) (*odrDposTrie, error) {
	return nil, errors.New("light node does not support accumulate reward")
}

// OpenDelegateTrie open the delegate trie
func (db *dposOdrDatabase) OpenDelegateTrie(root common.Hash) (*odrDposTrie, error) {
	id := dposDatabaseIDToTrieID(db.id, DelegateTrieSpec)
	return &odrDposTrie{
		db: db,
		id: id,
	}, nil
}

func (db *dposOdrDatabase) OpenVoteTrie(root common.Hash) (*odrDposTrie, error) {
	//id :=
}

func (db *dposOdrDatabase) openTrie(root common.Hash, spec DposTrieSpecifier) (*odrDposTrie, error) {

}

type odrDposTrie struct {
	db *dposOdrDatabase
	id *DposTrieID
	trie *trie.Trie
}

func (t *odrTrie)
