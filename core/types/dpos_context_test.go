// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package types

import (
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
	"github.com/stretchr/testify/assert"
)

var (
	addresses = []common.Address{
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
		common.HexToAddress("0x3"),
	}

	notExistAddress = common.HexToAddress("0x0")

	epochID      int64 = 1000000000
	epochIdBytes       = common.Int64ToBytes(epochID)
)

func TestDposContextSnapshot(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)

	snapshot := dposContext.Snapshot()
	assert.Equal(t, dposContext.Root(), snapshot.Root())
	assert.NotEqual(t, dposContext, snapshot)

	// change dposContext
	assert.Nil(t, dposContext.BecomeCandidate(addresses[0]))
	assert.NotEqual(t, dposContext.Root(), snapshot.Root())

	// revert snapshot
	dposContext.RevertToSnapShot(snapshot)
	assert.Equal(t, dposContext.Root(), snapshot.Root())
	assert.NotEqual(t, dposContext, snapshot)
}

func TestDposContextBecomeCandidate(t *testing.T) {
	candidates := addresses
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	for _, candidate := range candidates {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}

	candidateMap := map[common.Address]bool{}
	candidateIter := trie.NewIterator(dposContext.candidateTrie.NodeIterator(nil))
	for candidateIter.Next() {
		candidateMap[common.BytesToAddress(candidateIter.Value)] = true
	}

	assert.Equal(t, len(candidates), len(candidateMap))
	for _, candidate := range candidates {
		assert.True(t, candidateMap[candidate])
	}
}

func TestDposContextKickoutCandidate(t *testing.T) {
	candidates := addresses
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	for _, candidate := range candidates {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
		_, err := dposContext.Vote(candidate, candidates, epochID)
		assert.Nil(t, err)
	}

	kickIdx := 1
	assert.Nil(t, dposContext.KickoutCandidate(candidates[kickIdx], epochID))
	candidateMap := map[common.Address]bool{}
	candidateIter := trie.NewIterator(dposContext.candidateTrie.NodeIterator(nil))
	for candidateIter.Next() {
		candidateMap[common.BytesToAddress(candidateIter.Value)] = true
	}

	voteIter := trie.NewIterator(dposContext.voteTrie.NodeIterator(nil))
	voteMap := map[common.Address]bool{}
	for voteIter.Next() {
		voteMap[common.BytesToAddress(voteIter.Value)] = true
	}

	for i, candidate := range candidates {
		delegateIter := trie.NewIterator(dposContext.delegateTrie.PrefixIterator(append(epochIdBytes,candidate.Bytes()...)))
		if i == kickIdx {
			assert.False(t, delegateIter.Next())
			assert.False(t, candidateMap[candidate])
			assert.False(t, voteMap[candidate])
			continue
		}
		assert.True(t, delegateIter.Next())
		assert.True(t, candidateMap[candidate])
		assert.True(t, voteMap[candidate])
	}
}

func TestDposContextVoteAndCancelVote(t *testing.T) {
	candidate := addresses[0]
	newCandidate := addresses[1]
	delegator := addresses[2]
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	assert.Nil(t, dposContext.BecomeCandidate(candidate))
	assert.Nil(t, dposContext.BecomeCandidate(newCandidate))

	// delegator vote to not exist candidate
	candidateIter := trie.NewIterator(dposContext.candidateTrie.NodeIterator(nil))
	candidateMap := map[string]bool{}
	for candidateIter.Next() {
		candidateMap[string(candidateIter.Value)] = true
	}

	_, err = dposContext.Vote(delegator, []common.Address{notExistAddress}, epochID)
	assert.NotNil(t, err)

	// delegator vote to old candidate
	_, err = dposContext.Vote(delegator, []common.Address{candidate}, epochID)
	assert.Nil(t, err)
	delegateIter := trie.NewIterator(dposContext.delegateTrie.PrefixIterator(append(epochIdBytes,candidate.Bytes()...)))
	if assert.True(t, delegateIter.Next()) {
		assert.Equal(t, append(delegatePrefix, append(epochIdBytes,append(candidate.Bytes(), delegator.Bytes()...)...)...), delegateIter.Key)
		assert.Equal(t, delegator, common.BytesToAddress(delegateIter.Value))
	}

	voteIter := trie.NewIterator(dposContext.voteTrie.NodeIterator(nil))
	if assert.True(t, voteIter.Next()) {
		assert.Equal(t, append(votePrefix, delegator.Bytes()...), voteIter.Key)
		assert.Equal(t, candidate, common.BytesToAddress(voteIter.Value))
	}

	// delegator vote to new candidate
	_, err = dposContext.Vote(delegator, []common.Address{newCandidate}, epochID)
	assert.Nil(t, err)
	delegateIter = trie.NewIterator(dposContext.delegateTrie.PrefixIterator(append(epochIdBytes,candidate.Bytes()...)))
	assert.False(t, delegateIter.Next())
	delegateIter = trie.NewIterator(dposContext.delegateTrie.PrefixIterator(append(epochIdBytes, newCandidate.Bytes()...)))
	if assert.True(t, delegateIter.Next()) {
		assert.Equal(t, append(delegatePrefix, append(epochIdBytes,append(newCandidate.Bytes(), delegator.Bytes()...)...)...), delegateIter.Key)
		assert.Equal(t, delegator, common.BytesToAddress(delegateIter.Value))
	}

	voteIter = trie.NewIterator(dposContext.voteTrie.NodeIterator(nil))
	if assert.True(t, voteIter.Next()) {
		assert.Equal(t, append(votePrefix, delegator.Bytes()...), voteIter.Key)
		assert.Equal(t, newCandidate, common.BytesToAddress(voteIter.Value))
	}

	// a not exist delegator cancel vote record
	assert.NotNil(t, dposContext.CancelVote(notExistAddress, epochID))

	// delegator cancel all vote record
	assert.Nil(t, dposContext.CancelVote(delegator, epochID))
	delegateIter = trie.NewIterator(dposContext.delegateTrie.PrefixIterator(newCandidate.Bytes()))
	assert.False(t, delegateIter.Next())
	voteIter = trie.NewIterator(dposContext.voteTrie.NodeIterator(nil))
	assert.False(t, voteIter.Next())
}

func TestDposContextValidators(t *testing.T) {
	validators := addresses
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)

	assert.Nil(t, err)
	assert.Nil(t, dposContext.SetValidators(validators))

	result, err := dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, len(validators), len(result))

	validatorMap := map[common.Address]bool{}
	for _, validator := range validators {
		validatorMap[validator] = true
	}

	for _, validator := range result {
		assert.True(t, validatorMap[validator])
	}
}

func TestDposContext_GetVotedCandidatesByAddress(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)

	bytes, err := rlp.EncodeToBytes(addresses)
	assert.Nil(t, err)

	delegator := common.HexToAddress("0x666")
	err = dposContext.voteTrie.TryUpdate(delegator.Bytes(), bytes)
	assert.Nil(t, err)

	candidateListFromTrie, err := dposContext.GetVotedCandidatesByAddress(delegator)
	assert.Nil(t, err)
	assert.Equal(t, addresses, candidateListFromTrie)
}
