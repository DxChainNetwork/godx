// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package types

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"

	"golang.org/x/crypto/sha3"
)

// DposContext wraps 5 tries to store data needed in dpos consensus
type DposContext struct {
	epochTrie     *trie.Trie
	delegateTrie  *trie.Trie
	voteTrie      *trie.Trie
	candidateTrie *trie.Trie
	mintCntTrie   *trie.Trie

	db *trie.Database
}

var (
	epochPrefix     = []byte("epoch-")
	delegatePrefix  = []byte("delegate-")
	votePrefix      = []byte("vote-")
	candidatePrefix = []byte("candidate-")
	mintCntPrefix   = []byte("mintCnt-")
)

func NewEpochTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, epochPrefix, db)
}

func NewDelegateTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, delegatePrefix, db)
}

func NewVoteTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, votePrefix, db)
}

func NewCandidateTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, candidatePrefix, db)
}

func NewMintCntTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, mintCntPrefix, db)
}

// NewDposContext creates DposContext with the given database
func NewDposContext(db ethdb.Database) (*DposContext, error) {
	dbWithCache := trie.NewDatabaseWithCache(db, 256)

	epochTrie, err := NewEpochTrie(common.Hash{}, dbWithCache)
	if err != nil {
		return nil, err
	}

	delegateTrie, err := NewDelegateTrie(common.Hash{}, dbWithCache)
	if err != nil {
		return nil, err
	}

	voteTrie, err := NewVoteTrie(common.Hash{}, dbWithCache)
	if err != nil {
		return nil, err
	}

	candidateTrie, err := NewCandidateTrie(common.Hash{}, dbWithCache)
	if err != nil {
		return nil, err
	}

	mintCntTrie, err := NewMintCntTrie(common.Hash{}, dbWithCache)
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		mintCntTrie:   mintCntTrie,
		db:            dbWithCache,
	}, nil
}

// NewDposContextFromProto creates DposContext with database and trie root
func NewDposContextFromProto(db ethdb.Database, ctxProto *DposContextProto) (*DposContext, error) {
	dbWithCache := trie.NewDatabaseWithCache(db, 256)

	epochTrie, err := NewEpochTrie(ctxProto.EpochRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	delegateTrie, err := NewDelegateTrie(ctxProto.DelegateRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	voteTrie, err := NewVoteTrie(ctxProto.VoteRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	candidateTrie, err := NewCandidateTrie(ctxProto.CandidateRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	mintCntTrie, err := NewMintCntTrie(ctxProto.MintCntRoot, dbWithCache)
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		mintCntTrie:   mintCntTrie,
		db:            dbWithCache,
	}, nil
}

// Copy creates a new DposContext which has the same content with old one
func (dc *DposContext) Copy() *DposContext {
	epochTrie := *dc.epochTrie
	delegateTrie := *dc.delegateTrie
	voteTrie := *dc.voteTrie
	candidateTrie := *dc.candidateTrie
	mintCntTrie := *dc.mintCntTrie
	return &DposContext{
		epochTrie:     &epochTrie,
		delegateTrie:  &delegateTrie,
		voteTrie:      &voteTrie,
		candidateTrie: &candidateTrie,
		mintCntTrie:   &mintCntTrie,
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dc *DposContext) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dc.epochTrie.Hash())
	rlp.Encode(hw, dc.delegateTrie.Hash())
	rlp.Encode(hw, dc.candidateTrie.Hash())
	rlp.Encode(hw, dc.voteTrie.Hash())
	rlp.Encode(hw, dc.mintCntTrie.Hash())
	hw.Sum(h[:0])
	return h
}

// Snapshot works as same with Copy
func (dc *DposContext) Snapshot() *DposContext {
	return dc.Copy()
}

// RevertToSnapShot revert current DposContext to a previous one
func (dc *DposContext) RevertToSnapShot(snapshot *DposContext) {
	dc.epochTrie = snapshot.epochTrie
	dc.delegateTrie = snapshot.delegateTrie
	dc.candidateTrie = snapshot.candidateTrie
	dc.voteTrie = snapshot.voteTrie
	dc.mintCntTrie = snapshot.mintCntTrie
}

// FromProto will recover the entire DposContext with the given trie root
func (dc *DposContext) FromProto(dcp *DposContextProto) error {
	var err error
	dc.epochTrie, err = NewEpochTrie(dcp.EpochRoot, dc.db)
	if err != nil {
		return err
	}

	dc.delegateTrie, err = NewDelegateTrie(dcp.DelegateRoot, dc.db)
	if err != nil {
		return err
	}

	dc.candidateTrie, err = NewCandidateTrie(dcp.CandidateRoot, dc.db)
	if err != nil {
		return err
	}

	dc.voteTrie, err = NewVoteTrie(dcp.VoteRoot, dc.db)
	if err != nil {
		return err
	}

	dc.mintCntTrie, err = NewMintCntTrie(dcp.MintCntRoot, dc.db)
	return err
}

// DposContextProto wrap 5 trie root hash
type DposContextProto struct {
	EpochRoot     common.Hash `json:"epochRoot"        gencodec:"required"`
	DelegateRoot  common.Hash `json:"delegateRoot"     gencodec:"required"`
	CandidateRoot common.Hash `json:"candidateRoot"    gencodec:"required"`
	VoteRoot      common.Hash `json:"voteRoot"         gencodec:"required"`
	MintCntRoot   common.Hash `json:"mintCntRoot"      gencodec:"required"`
}

// ToProto convert DposContext to DposContextProto
func (dc *DposContext) ToProto() *DposContextProto {
	return &DposContextProto{
		EpochRoot:     dc.epochTrie.Hash(),
		DelegateRoot:  dc.delegateTrie.Hash(),
		CandidateRoot: dc.candidateTrie.Hash(),
		VoteRoot:      dc.voteTrie.Hash(),
		MintCntRoot:   dc.mintCntTrie.Hash(),
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dcp *DposContextProto) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dcp.EpochRoot)
	rlp.Encode(hw, dcp.DelegateRoot)
	rlp.Encode(hw, dcp.CandidateRoot)
	rlp.Encode(hw, dcp.VoteRoot)
	rlp.Encode(hw, dcp.MintCntRoot)
	hw.Sum(h[:0])
	return h
}

// KickoutCandidate will kick out the given candidate
func (dc *DposContext) KickoutCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	err := dc.candidateTrie.TryDelete(candidate)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
	}

	iter := trie.NewIterator(dc.delegateTrie.PrefixIterator(candidate))
	for iter.Next() {
		delegator := iter.Value
		key := append(candidate, delegator...)
		err = dc.delegateTrie.TryDelete(key)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return err
			}
		}

		v, err := dc.voteTrie.TryGet(delegator)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return err
			}
		}

		if err == nil && bytes.Equal(v, candidate) {
			err = dc.voteTrie.TryDelete(delegator)
			if err != nil {
				if _, ok := err.(*trie.MissingNodeError); !ok {
					return err
				}
			}
		}
	}

	return nil
}

// BecomeCandidate will store the given candidate into candidateTrie
func (dc *DposContext) BecomeCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	return dc.candidateTrie.TryUpdate(candidate, candidate)
}

// Delegate will store the delegation record
func (dc *DposContext) Delegate(delegatorAddr, candidateAddr common.Address) error {
	delegator, candidate := delegatorAddr.Bytes(), candidateAddr.Bytes()

	// the given candidate must have been candidate
	candidateInTrie, err := dc.candidateTrie.TryGet(candidate)
	if err != nil {
		return err
	}

	if candidateInTrie == nil {
		return errors.New("invalid candidate to delegate")
	}

	// delete old candidate if exists
	oldCandidate, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
	}

	if oldCandidate != nil {
		dc.delegateTrie.Delete(append(oldCandidate, delegator...))
	}

	if err = dc.delegateTrie.TryUpdate(append(candidate, delegator...), delegator); err != nil {
		return err
	}

	return dc.voteTrie.TryUpdate(delegator, candidate)
}

// UnDelegate will will remove the delegation record
func (dc *DposContext) UnDelegate(delegatorAddr, candidateAddr common.Address) error {
	delegator, candidate := delegatorAddr.Bytes(), candidateAddr.Bytes()

	// the candidate must be candidate
	candidateInTrie, err := dc.candidateTrie.TryGet(candidate)
	if err != nil {
		return err
	}

	if candidateInTrie == nil {
		return errors.New("invalid candidate to undelegate")
	}

	oldCandidate, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		return err
	}

	if !bytes.Equal(candidate, oldCandidate) {
		return errors.New("mismatch candidate to undelegate")
	}

	if err = dc.delegateTrie.TryDelete(append(candidate, delegator...)); err != nil {
		return err
	}

	return dc.voteTrie.TryDelete(delegator)
}

// Commit writes the data in 5 tries to db
func (dc *DposContext) Commit() (*DposContextProto, error) {

	// commit dpos context into memory
	epochRoot, err := dc.epochTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	delegateRoot, err := dc.delegateTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	voteRoot, err := dc.voteTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	candidateRoot, err := dc.candidateTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	mintCntRoot, err := dc.mintCntTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	// commit dpos context into disk, and this is the finally commit
	err = dc.DB().Commit(epochRoot, false)
	if err != nil {
		return nil, err
	}

	err = dc.DB().Commit(candidateRoot, false)
	if err != nil {
		return nil, err
	}

	err = dc.DB().Commit(delegateRoot, false)
	if err != nil {
		return nil, err
	}

	err = dc.DB().Commit(mintCntRoot, false)
	if err != nil {
		return nil, err
	}

	err = dc.DB().Commit(voteRoot, false)
	if err != nil {
		return nil, err
	}

	return &DposContextProto{
		EpochRoot:     epochRoot,
		DelegateRoot:  delegateRoot,
		VoteRoot:      voteRoot,
		CandidateRoot: candidateRoot,
		MintCntRoot:   mintCntRoot,
	}, nil
}

func (dc *DposContext) CandidateTrie() *trie.Trie         { return dc.candidateTrie }
func (dc *DposContext) DelegateTrie() *trie.Trie          { return dc.delegateTrie }
func (dc *DposContext) VoteTrie() *trie.Trie              { return dc.voteTrie }
func (dc *DposContext) EpochTrie() *trie.Trie             { return dc.epochTrie }
func (dc *DposContext) MintCntTrie() *trie.Trie           { return dc.mintCntTrie }
func (dc *DposContext) DB() *trie.Database                { return dc.db }
func (dc *DposContext) SetEpoch(epoch *trie.Trie)         { dc.epochTrie = epoch }
func (dc *DposContext) SetDelegate(delegate *trie.Trie)   { dc.delegateTrie = delegate }
func (dc *DposContext) SetVote(vote *trie.Trie)           { dc.voteTrie = vote }
func (dc *DposContext) SetCandidate(candidate *trie.Trie) { dc.candidateTrie = candidate }
func (dc *DposContext) SetMintCnt(mintCnt *trie.Trie)     { dc.mintCntTrie = mintCnt }

// GetValidators retrieves validator list in current epoch
func (dc *DposContext) GetValidators() ([]common.Address, error) {
	var validators []common.Address
	key := []byte("validator")
	validatorsRLP := dc.epochTrie.Get(key)
	if err := rlp.DecodeBytes(validatorsRLP, &validators); err != nil {
		return nil, fmt.Errorf("failed to decode validators: %s", err)
	}

	return validators, nil
}

// SetValidators update validators into epochTrie
func (dc *DposContext) SetValidators(validators []common.Address) error {
	key := []byte("validator")
	validatorsRLP, err := rlp.EncodeToBytes(validators)
	if err != nil {
		return fmt.Errorf("failed to encode validators to rlp bytes: %s", err)
	}

	dc.epochTrie.Update(key, validatorsRLP)
	return nil
}
