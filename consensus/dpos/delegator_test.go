// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rlp"
)

var dx = common.NewBigIntUint64(1e18)

// TestProcessVoteIncreaseDeposit test function ProcessDeposit of previously not a delegator
func TestProcessVoteNewDelegator(t *testing.T) {
	addr := randomAddress()
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(30)
	if err != nil {
		t.Fatal(err)
	}
	deposit, curTime := dx.MultInt64(10), time.Now().Unix()
	addAccountInState(stateDB, addr, deposit, common.BigInt0)
	// Process vote
	_, err = ProcessVote(stateDB, ctx, addr, deposit, candidates, uint64(0), curTime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	err = checkProcessVote(stateDB, ctx, addr, deposit, deposit, candidates, calcThawingEpoch(CalculateEpochID(curTime)),
		common.BigInt0, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessVoteIncreasingDeposit(t *testing.T) {
	addr := randomAddress()
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(50)
	if err != nil {
		t.Fatal(err)
	}
	addAccountInState(stateDB, addr, dx.MultInt64(10), common.BigInt0)
	// Vote the first time
	prevDeposit, prevCandidates, prevTime := dx, candidates[:30], time.Now().AddDate(0, 0, -1).Unix()
	_, err = ProcessVote(stateDB, ctx, addr, prevDeposit, prevCandidates, uint64(0), prevTime)
	if err != nil {
		t.Fatal(err)
	}
	// Vote the second time
	curDeposit, curCandidates, curTime := dx.MultInt64(10), candidates[20:], time.Now().Unix()
	_, err = ProcessVote(stateDB, ctx, addr, curDeposit, curCandidates, uint64(0), curTime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	// Check the result
	err = checkProcessVote(stateDB, ctx, addr, curDeposit, curDeposit, curCandidates,
		calcThawingEpoch(CalculateEpochID(curTime)), common.BigInt0, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessVoteDecreasingDeposit(t *testing.T) {
	addr := randomAddress()
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(50)
	if err != nil {
		t.Fatal(err)
	}
	addAccountInState(stateDB, addr, dx.MultInt64(10), common.BigInt0)
	// Vote the first time
	prevDeposit, prevCandidates, prevTime := dx.MultInt64(10), candidates[:30], time.Now().AddDate(0, 0, -1).Unix()
	_, err = ProcessVote(stateDB, ctx, addr, prevDeposit, prevCandidates, uint64(0), prevTime)
	if err != nil {
		t.Fatal(err)
	}
	// Vote the second time
	curDeposit, curCandidates, curTime := dx.MultInt64(1), candidates[20:], time.Now().Unix()
	_, err = ProcessVote(stateDB, ctx, addr, curDeposit, curCandidates, uint64(0), curTime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	// Check the result
	err = checkProcessVote(stateDB, ctx, addr, prevDeposit, curDeposit, curCandidates,
		calcThawingEpoch(CalculateEpochID(curTime)), prevDeposit.Sub(curDeposit), true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessVoteErr(t *testing.T) {
	addr := randomAddress()
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(30)
	if err != nil {
		t.Fatal(err)
	}
	addAccountInState(stateDB, addr, dx.MultInt64(10), common.BigInt0)
	curTime := time.Now().Unix()
	thawingEpoch := calcThawingEpoch(CalculateEpochID(curTime))
	// Error 1: error from checkValidVote
	_, err = ProcessVote(stateDB, ctx, addr, dx.MultInt64(11), candidates, uint64(0), curTime)
	if err == nil {
		t.Fatal("should raise error not enough balance")
	}
	if _, err := stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	// check the result
	err = checkProcessVote(stateDB, ctx, addr, common.BigInt0, common.BigInt0, []common.Address{},
		thawingEpoch, common.BigInt0, false)
	if err != nil {
		t.Fatal(err)
	}
	// Error 2: no valid candidates
	_, err = ProcessVote(stateDB, ctx, addr, dx.MultInt64(1), []common.Address{randomAddress()}, uint64(0), curTime)
	if err == nil {
		t.Fatal("should raise no candidate voted error")
	}
	if _, err := stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	err = checkProcessVote(stateDB, ctx, addr, common.BigInt0, common.BigInt0, []common.Address{},
		thawingEpoch, common.BigInt0, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessCancelVote(t *testing.T) {
	addr := randomAddress()
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(30)
	if err != nil {
		t.Fatal(err)
	}
	prevFrozen, deposit, curTime := dx.MultInt64(1), dx.MultInt64(8), time.Now().Unix()
	addAccountInState(stateDB, addr, dx.MultInt64(10), prevFrozen)
	thawingEpoch := calcThawingEpoch(CalculateEpochID(curTime))
	// Process Vote
	_, err = ProcessVote(stateDB, ctx, addr, deposit, candidates, uint64(0), curTime)
	if err != nil {
		t.Fatal(err)
	}
	// Cancel Vote
	if err = ProcessCancelVote(stateDB, ctx, addr, curTime); err != nil {
		t.Fatal(err)
	}
	if _, err = stateDB.Commit(true); err != nil {
		t.Fatal(err)
	}
	err = checkProcessVote(stateDB, ctx, addr, prevFrozen.Add(deposit), common.BigInt0, []common.Address{},
		thawingEpoch, deposit, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckValidVote(t *testing.T) {
	addr := randomAddress()
	tests := []struct {
		balance      common.BigInt
		frozenAssets common.BigInt
		prevDeposit  common.BigInt
		deposit      common.BigInt
		candidates   []common.Address
		newDeposit   common.BigInt
		expectedErr  error
	}{
		{
			balance:      dx.MultInt64(10),
			frozenAssets: dx.MultInt64(4),
			prevDeposit:  common.BigInt0,
			deposit:      common.BigInt0,
			candidates:   makeCandidates(30),
			expectedErr:  errVoteZeroOrNegativeDeposit,
		},
		{
			balance:      dx.MultInt64(10),
			frozenAssets: dx.MultInt64(4),
			prevDeposit:  common.BigInt0,
			deposit:      dx.MultInt64(4),
			candidates:   makeCandidates(0),
			expectedErr:  errVoteZeroCandidates,
		},
		{
			balance:      dx.MultInt64(10),
			frozenAssets: dx.MultInt64(4),
			prevDeposit:  common.BigInt0,
			deposit:      dx.MultInt64(4),
			candidates:   makeCandidates(31),
			expectedErr:  errVoteTooManyCandidates,
		},
		{
			balance:      dx.MultInt64(10),
			frozenAssets: dx.MultInt64(4),
			prevDeposit:  dx.MultInt64(1),
			deposit:      dx.MultInt64(10),
			candidates:   makeCandidates(30),
			expectedErr:  errVoteInsufficientBalance,
		},
		{
			balance:      dx.MultInt64(10),
			frozenAssets: dx.MultInt64(4),
			prevDeposit:  dx.MultInt64(4),
			deposit:      dx.MultInt64(10),
			candidates:   makeCandidates(30),
			expectedErr:  nil,
		},
	}
	for i, test := range tests {
		state, _, err := newStateAndDposContext()
		if err != nil {
			t.Fatal(err)
		}
		addAccountInState(state, addr, test.balance, test.frozenAssets)
		SetVoteDeposit(state, addr, test.prevDeposit)
		err = checkValidVote(state, addr, test.deposit, test.candidates)
		if err != test.expectedErr {
			t.Errorf("Test %d: error expect [%v], got [%v]", i, test.expectedErr, err)
		}
	}
}

func makeCandidates(num int) []common.Address {
	addresses := make([]common.Address, 0, num)
	for i := 0; i != num; i++ {
		addr := common.BigToAddress(common.NewBigInt(int64(i)).BigIntPtr())
		addresses = append(addresses, addr)
	}
	return addresses
}

func checkProcessVote(state *state.StateDB, ctx *types.DposContext, addr common.Address,
	expectedFrozenAssets common.BigInt, expectedDeposit common.BigInt, expectedCandidates []common.Address,
	thawEpoch int64, thawValue common.BigInt, addrInTrie bool) error {

	// Check voteTrie
	voteTrie := ctx.VoteTrie()
	candidateBytes, err := voteTrie.TryGet(addr.Bytes())
	if !addrInTrie && (err == nil && candidateBytes != nil && len(candidateBytes) != 0) {
		return fmt.Errorf("address %x should not in vote trie", addr)
	}
	if addrInTrie {
		if err != nil || candidateBytes == nil || len(candidateBytes) == 0 {
			return fmt.Errorf("address %x not in vote trie", addr)
		}
		// check whether the candidates are written to voteTrie
		var candidates []common.Address
		if err := rlp.DecodeBytes(candidateBytes, &candidates); err != nil {
			return fmt.Errorf("rlp candidates decode error: %v", err)
		}
		if err := checkSameValidatorSet(candidates, expectedCandidates); err != nil {
			return fmt.Errorf("validator set not expected: %v", err)
		}
	}
	// Check the vote deposit
	voteDeposit := GetVoteDeposit(state, addr)
	if voteDeposit.Cmp(expectedDeposit) != 0 {
		return fmt.Errorf("vote deposit not expected. Got %v, Expect %v", voteDeposit, expectedDeposit)
	}
	// Check frozenAssets
	frozenAssets := GetFrozenAssets(state, addr)
	if frozenAssets.Cmp(expectedFrozenAssets) != 0 {
		return fmt.Errorf("frozen assets not expected. Got %v, Expect %v", frozenAssets, expectedFrozenAssets)
	}
	// Check thawing field
	if thawEpoch == 0 && thawValue.Cmp(common.BigInt0) == 0 {
		return nil
	}
	m := make(map[common.Address]common.BigInt)
	if thawValue.Cmp(common.BigInt0) != 0 {
		m[addr] = thawValue
	}
	return checkThawingAddressAndValue(state, thawEpoch, m)
}

func newStateAndDposContextWithCandidate(num int) (*state.StateDB, *types.DposContext, []common.Address, error) {
	stateDB, ctx, err := newStateAndDposContext()
	if err != nil {
		return nil, nil, nil, err
	}
	var addresses []common.Address
	for i := 0; i != num; i++ {
		addr := common.BigToAddress(common.NewBigIntUint64(uint64(i)).BigIntPtr())
		addAccountInState(stateDB, addr, minDeposit, common.BigInt0)
		err = ProcessAddCandidate(stateDB, ctx, addr, minDeposit, uint64(50))
		if err != nil {
			return nil, nil, nil, err
		}
		addresses = append(addresses, addr)
	}
	return stateDB, ctx, addresses, nil
}
