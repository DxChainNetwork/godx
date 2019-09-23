// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math/big"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
)

func TestLookupValidator(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposCtx, _ := types.NewDposContext(db)
	mockEpochContext := &EpochContext{
		DposContext: dposCtx,
	}

	validators := []common.Address{
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
		common.HexToAddress("0x3"),
	}

	err := mockEpochContext.DposContext.SetValidators(validators)
	if err != nil {
		t.Fatalf("Failed to set valdiators,error: %v", err)
	}

	for i, expected := range validators {
		got, _ := mockEpochContext.lookupValidator(int64(i) * BlockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.String(), got.String())
		}
	}

	_, err = mockEpochContext.lookupValidator(BlockInterval - 1)
	if err != errInvalidMinedBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", errInvalidMinedBlockTime, err)
	}
}

func Test_CountVotes(t *testing.T) {

	// mock addresses
	addresses := []common.Address{
		common.HexToAddress("0x58a366c3c1a735bf3d09f2a48a014a8ebc64457c"),
		common.HexToAddress("0x60c8947134be7c0604a866a0462542eb0dcf71f9"),
		common.HexToAddress("0x801ee9587ea0d52fe477755a3e91d7244e6556a3"),
		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"),
		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"),
	}

	// mock state
	db := ethdb.NewMemDatabase()
	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	for i := 0; i < len(addresses); i++ {
		stateDB.SetNonce(addresses[i], 1)
		bal := int64(1e10 * (i + 1))
		stateDB.SetBalance(addresses[i], new(big.Int).SetInt64(bal))
	}
	root, _ := stateDB.Commit(false)
	stateDB, _ = state.New(root, sdb)

	// create epoch context
	dposCtx, _ := types.NewDposContext(db)
	epochContext := &EpochContext{
		DposContext: dposCtx,
		stateDB:     stateDB,
	}

	// mock some vote records
	for i, addr := range addresses {
		addrBytes := addr.Bytes()
		err := epochContext.DposContext.CandidateTrie().TryUpdate(addrBytes, addrBytes)
		if err != nil {
			t.Fatalf("Failed to update candidates,error: %v", err)
		}

		for j := 0; j < len(addresses); j++ {
			key := append(addresses[j].Bytes(), addrBytes...)
			err = epochContext.DposContext.DelegateTrie().TryUpdate(key, addrBytes)
			if err != nil {
				t.Fatalf("Failed to update vote records,error: %v", err)
			}
		}

		_, err = epochContext.DposContext.Commit()
		if err != nil {
			t.Fatalf("Failed to commit mock dpos context,error: %v", err)
		}

		// set candidates deposit
		candidateDeposit := new(big.Int).SetInt64(int64(1e6 * (i + 1)))
		stateDB.SetState(addr, KeyCandidateDeposit, common.BytesToHash(candidateDeposit.Bytes()))

		// set vote deposit
		voteDeposit := new(big.Int).SetInt64(int64(1e6 * (i + 1)))
		stateDB.SetState(addr, KeyVoteDeposit, common.BytesToHash(voteDeposit.Bytes()))

		_, err = stateDB.Commit(false)
		if err != nil {
			t.Fatalf("Failed to commit state,error: %v", err)
		}
	}

	// count votes
	votes, err := epochContext.countVotes()
	if err != nil {
		t.Errorf("Failed to count votes,error: %v", err)
	}

	// check vote weight without attenuation
	expectedVoteWeightWithoutAttenuation := int64(15e6)
	for _, entry := range votes {
		addr, weight := entry.addr, entry.vote
		candidateDeposit := stateDB.GetState(addr, KeyCandidateDeposit).Big()
		wantTotalVoteWeight := expectedVoteWeightWithoutAttenuation + candidateDeposit.Int64()
		if weight.Cmp(common.NewBigInt(wantTotalVoteWeight)) != 0 {
			t.Errorf("%s wanted vote weight: %d,got %v", addr.String(), wantTotalVoteWeight, weight)
		}
	}
}

func Test_KickoutValidators(t *testing.T) {
	now := time.Now().Unix()
	var (
		delegator = common.HexToAddress("0xaaa")
	)

	db := ethdb.NewMemDatabase()
	dposContext, candidates, err := mockDposContext(db, now, delegator)
	if err != nil {
		t.Fatalf("failed to new dpos context,error: %v", err)
	}

	timeOfFirstBlock = 100000

	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	epochContext := &EpochContext{
		DposContext: dposContext,
		TimeStamp:   now,
		stateDB:     stateDB,
	}

	epochID := CalculateEpochID(now)
	err = epochContext.kickoutValidators(epochID)
	if err != nil {
		t.Errorf("something wrong to kick out validators,error: %v", err)
	}

	validatorsFromTrie, err := epochContext.DposContext.GetValidators()
	if err != nil {
		t.Fatalf("failed to retrieve validators,error: %v", err)
	}

	if len(validatorsFromTrie) != MaxValidatorSize {
		t.Errorf("after kick out,wanted validator length: %v,got: %v", MaxValidatorSize, len(validatorsFromTrie))
	}

	for i := 0; i < MaxValidatorSize/3; i++ {
		canFromTrie := epochContext.DposContext.CandidateTrie().Get(candidates[i].Bytes())
		if canFromTrie != nil {
			t.Errorf("failed to delete the kick out one from candidates trie: %s", candidates[i].String())
		}

		delegatorFromTrie := epochContext.DposContext.DelegateTrie().Get(append(candidates[i].Bytes(), delegator.Bytes()...))
		if delegatorFromTrie != nil {
			t.Errorf("failed to delete the kick out one from delegate trie: %s", candidates[i].String())
		}
	}

	votedFromTrie := epochContext.DposContext.VoteTrie().Get(delegator.Bytes())
	if votedFromTrie == nil {
		t.Fatalf("failed to retrieve voted candidates")
	}

	var votedCan []common.Address
	err = rlp.DecodeBytes(votedFromTrie, &votedCan)
	if err != nil {
		t.Fatalf("failed to rlp decode voted candidates")
	}

	if len(votedCan) != MaxValidatorSize+5-MaxValidatorSize/3 {
		t.Errorf("failed to delete the kick out one from vote trie,wanted length: %d,got: %d", MaxValidatorSize+5-MaxValidatorSize/3, len(votedCan))
	}
}
