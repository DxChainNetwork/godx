// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package types

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
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
	minedCntTrie  *trie.Trie

	db *trie.Database
}

var (
	epochPrefix     = []byte("epoch-")
	delegatePrefix  = []byte("delegate-")
	votePrefix      = []byte("vote-")
	candidatePrefix = []byte("candidate-")
	minedCntPrefix  = []byte("minedCnt-")
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

func NewMinedCntTrie(root common.Hash, db *trie.Database) (*trie.Trie, error) {
	return trie.NewTrieWithPrefix(root, minedCntPrefix, db)
}

// NewDposContext creates DposContext with the given database
func NewDposContext(diskdb ethdb.Database) (*DposContext, error) {
	db := trie.NewDatabase(diskdb)

	epochTrie, err := NewEpochTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	delegateTrie, err := NewDelegateTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	voteTrie, err := NewVoteTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	candidateTrie, err := NewCandidateTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	minedCntTrie, err := NewMinedCntTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		minedCntTrie:  minedCntTrie,
		db:            db,
	}, nil
}

// NewDposContextFromProto creates DposContext with database and trie root
func NewDposContextFromProto(diskdb ethdb.Database, ctxProto *DposContextRoot) (*DposContext, error) {
	db := trie.NewDatabase(diskdb)

	epochTrie, err := NewEpochTrie(ctxProto.EpochRoot, db)
	if err != nil {
		return nil, err
	}

	delegateTrie, err := NewDelegateTrie(ctxProto.DelegateRoot, db)
	if err != nil {
		return nil, err
	}

	voteTrie, err := NewVoteTrie(ctxProto.VoteRoot, db)
	if err != nil {
		return nil, err
	}

	candidateTrie, err := NewCandidateTrie(ctxProto.CandidateRoot, db)
	if err != nil {
		return nil, err
	}

	minedCntTrie, err := NewMinedCntTrie(ctxProto.MinedCntRoot, db)
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		minedCntTrie:  minedCntTrie,
		db:            db,
	}, nil
}

// Copy creates a new DposContext which has the same content with old one
func (dc *DposContext) Copy() *DposContext {
	epochTrie := *dc.epochTrie
	delegateTrie := *dc.delegateTrie
	voteTrie := *dc.voteTrie
	candidateTrie := *dc.candidateTrie
	minedCntTrie := *dc.minedCntTrie
	return &DposContext{
		epochTrie:     &epochTrie,
		delegateTrie:  &delegateTrie,
		voteTrie:      &voteTrie,
		candidateTrie: &candidateTrie,
		minedCntTrie:  &minedCntTrie,
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dc *DposContext) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dc.epochTrie.Hash())
	rlp.Encode(hw, dc.delegateTrie.Hash())
	rlp.Encode(hw, dc.candidateTrie.Hash())
	rlp.Encode(hw, dc.voteTrie.Hash())
	rlp.Encode(hw, dc.minedCntTrie.Hash())
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
	dc.minedCntTrie = snapshot.minedCntTrie
}

// DposContextRoot wrap 5 trie root hash
type DposContextRoot struct {
	EpochRoot     common.Hash `json:"epochRoot"        gencodec:"required"`
	DelegateRoot  common.Hash `json:"delegateRoot"     gencodec:"required"`
	CandidateRoot common.Hash `json:"candidateRoot"    gencodec:"required"`
	VoteRoot      common.Hash `json:"voteRoot"         gencodec:"required"`
	MinedCntRoot  common.Hash `json:"minedCntRoot"     gencodec:"required"`
}

// ToRoot convert DposContext to DposContextRoot
func (dc *DposContext) ToRoot() *DposContextRoot {
	return &DposContextRoot{
		EpochRoot:     dc.epochTrie.Hash(),
		DelegateRoot:  dc.delegateTrie.Hash(),
		CandidateRoot: dc.candidateTrie.Hash(),
		VoteRoot:      dc.voteTrie.Hash(),
		MinedCntRoot:  dc.minedCntTrie.Hash(),
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dcp *DposContextRoot) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dcp.EpochRoot)
	rlp.Encode(hw, dcp.DelegateRoot)
	rlp.Encode(hw, dcp.CandidateRoot)
	rlp.Encode(hw, dcp.VoteRoot)
	rlp.Encode(hw, dcp.MinedCntRoot)
	hw.Sum(h[:0])
	return h
}

// KickoutCandidate will kick out the given candidate
func (dc *DposContext) KickoutCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	err := dc.candidateTrie.TryDelete(candidate)
	if err != nil {

		// if got a MissingNodeError, that means cannot find the key of candidate in trie, it's normal case.
		// if not a MissingNodeError, that means something wrong with the db of trie
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

		votedCandidateBytes, err := dc.voteTrie.TryGet(delegator)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return err
			}
		}

		if votedCandidateBytes == nil {
			return fmt.Errorf("no any voted candiate for %s", common.BytesToAddress(delegator).String())
		}

		votedCandidates := make([]common.Address, 0)
		err = rlp.DecodeBytes(votedCandidateBytes, &votedCandidates)
		if err != nil {
			return fmt.Errorf("failed to rlp decode candidate bytes for voteTrie,error: %v", err)
		}

		// remove the given candidate from the vote records of delegator
		index := 0
		for i, votedCandidate := range votedCandidates {
			if votedCandidate == candidateAddr {
				index = i
				break
			}
		}
		pre := votedCandidates[:index]
		suf := make([]common.Address, 0)
		if index != len(votedCandidates)-1 {
			suf = votedCandidates[index+1:]
		}
		candidatesAfterRemove := make([]common.Address, 0)
		candidatesAfterRemove = append(candidatesAfterRemove, pre...)
		candidatesAfterRemove = append(candidatesAfterRemove, suf...)

		value, err := rlp.EncodeToBytes(candidatesAfterRemove)
		if err != nil {
			return fmt.Errorf("failed to rlp encode candidate bytes for voteTrie,error: %v", err)
		}
		err = dc.voteTrie.TryUpdate(delegator, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// BecomeCandidate will store the given candidate into candidateTrie
func (dc *DposContext) BecomeCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	return dc.candidateTrie.TryUpdate(candidate, candidate)
}

// Vote will store the vote record
func (dc *DposContext) Vote(delegatorAddr common.Address, candidateList []common.Address) (int, error) {
	delegator := delegatorAddr.Bytes()
	successVoted := make([]common.Address, 0)

	oldCandidateBytes, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return 0, fmt.Errorf("failed to retrieve from voteTrie,err: %v", err)
		}
	}

	// if voted before, then delete old vote record
	if oldCandidateBytes != nil {
		err = dc.voteTrie.TryDelete(delegator)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return 0, fmt.Errorf("failed to delete old votes from voteTrie,err: %v", err)
			}
		}

		oldCandidateList := make([]common.Address, 0)
		err = rlp.DecodeBytes(oldCandidateBytes, &oldCandidateList)
		if err != nil {
			return 0, fmt.Errorf("failed to rlp decode old candidate bytes,err: %v", err)
		}

		for _, oldCandidate := range oldCandidateList {
			err = dc.delegateTrie.TryDelete(append(oldCandidate.Bytes(), delegator...))
			if err != nil {
				if _, ok := err.(*trie.MissingNodeError); !ok {
					return 0, fmt.Errorf("failed to delete old votes from delegateTrie,err: %v", err)
				}
			}
		}
	}

	for _, candidateAddr := range candidateList {
		candidate := candidateAddr.Bytes()

		// the given candidate must have been candidate
		candidateInTrie, err := dc.candidateTrie.TryGet(candidate)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); ok {
				log.Error("No this candidate on chain", "candidate", candidateAddr.String())
				continue
			} else {
				return 0, fmt.Errorf("failed to retrieve from candidateTrie,err: %v", err)
			}
		}

		if candidateInTrie == nil {
			log.Error("Voted to a invalid candidate", "candidate", candidateAddr.String())
			continue
		}

		err = dc.delegateTrie.TryUpdate(append(candidate, delegator...), delegator)
		if err != nil {
			log.Error("Failed to update a new vote to delegateTrie", "error", err)
			continue
		}

		// successfully vote a candidate, then add it
		successVoted = append(successVoted, candidateAddr)
	}

	// store all successful voted candidate
	if len(successVoted) != 0 {
		votedCandidateBytes, err := rlp.EncodeToBytes(successVoted)
		if err != nil {
			return 0, fmt.Errorf("failed to rlp encode voted candidates,err: %v", err)
		}

		err = dc.voteTrie.TryUpdate(delegator, votedCandidateBytes)
		if err != nil {
			return 0, fmt.Errorf("failed to update new vote to voteTrie,err: %v", err)
		}
		return len(successVoted), nil
	}

	return 0, errors.New("failed to vote all candidates")
}

// CancelVote will remove all vote records
func (dc *DposContext) CancelVote(delegatorAddr common.Address) error {
	delegator := delegatorAddr.Bytes()

	oldCandidateBytes, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return fmt.Errorf("failed to retrieve from voteTrie,err: %v", err)
		}
	}

	if oldCandidateBytes == nil {
		return fmt.Errorf("no vote records on chain for %s", delegatorAddr.String())
	}

	oldCandidateList := make([]common.Address, 0)
	err = rlp.DecodeBytes(oldCandidateBytes, &oldCandidateList)
	if err != nil {
		return fmt.Errorf("failed to rlp decode old candidate bytes,err: %v", err)
	}

	// delete all vote records from delegateTrie
	for _, oldCandidate := range oldCandidateList {
		err = dc.delegateTrie.TryDelete(append(oldCandidate.Bytes(), delegator...))
		if err != nil {
			return fmt.Errorf("failed to delete old votes from delegateTrie,err: %v", err)
		}
	}

	// delete vote records from voteTrie
	err = dc.voteTrie.TryDelete(delegator)
	if err != nil {
		return fmt.Errorf("failed to delete old votes from voteTrie,err: %v", err)
	}

	return nil
}

// Commit writes the data in 5 tries to db
func (dc *DposContext) Commit() (*DposContextRoot, error) {

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

	minedCntRoot, err := dc.minedCntTrie.Commit(nil)
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

	err = dc.DB().Commit(minedCntRoot, false)
	if err != nil {
		return nil, err
	}

	err = dc.DB().Commit(voteRoot, false)
	if err != nil {
		return nil, err
	}

	return &DposContextRoot{
		EpochRoot:     epochRoot,
		DelegateRoot:  delegateRoot,
		VoteRoot:      voteRoot,
		CandidateRoot: candidateRoot,
		MinedCntRoot:  minedCntRoot,
	}, nil
}

func (dc *DposContext) CandidateTrie() *trie.Trie         { return dc.candidateTrie }
func (dc *DposContext) DelegateTrie() *trie.Trie          { return dc.delegateTrie }
func (dc *DposContext) VoteTrie() *trie.Trie              { return dc.voteTrie }
func (dc *DposContext) EpochTrie() *trie.Trie             { return dc.epochTrie }
func (dc *DposContext) MinedCntTrie() *trie.Trie          { return dc.minedCntTrie }
func (dc *DposContext) DB() *trie.Database                { return dc.db }
func (dc *DposContext) SetEpoch(epoch *trie.Trie)         { dc.epochTrie = epoch }
func (dc *DposContext) SetDelegate(delegate *trie.Trie)   { dc.delegateTrie = delegate }
func (dc *DposContext) SetVote(vote *trie.Trie)           { dc.voteTrie = vote }
func (dc *DposContext) SetCandidate(candidate *trie.Trie) { dc.candidateTrie = candidate }
func (dc *DposContext) SetMinedCnt(minedCnt *trie.Trie)   { dc.minedCntTrie = minedCnt }

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

// GetVotedCandidatesByAddress retrieve all voted candidates of given delegator
func (dc *DposContext) GetVotedCandidatesByAddress(delegator common.Address) ([]common.Address, error) {
	key := delegator.Bytes()
	candidatesRLP := dc.voteTrie.Get(key)
	var result []common.Address
	err := rlp.DecodeBytes(candidatesRLP, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vaoted candidates: %s", err)
	}

	return result, nil
}
