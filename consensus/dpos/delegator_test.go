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

type delegator struct {
	addr         common.Address
	balance      common.BigInt
	frozenAssets common.BigInt
	deposit      common.BigInt
	candidates   []common.Address
}

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
	_, err = ProcessVote(stateDB, ctx, addr, deposit, candidates, curTime)
	if err != nil {
		t.Fatal(err)
	}
	err = checkProcessVote(stateDB, ctx, addr, deposit, deposit, candidates, 0, common.BigInt0)
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
	_, err = ProcessVote(stateDB, ctx, addr, prevDeposit, prevCandidates, prevTime)
	if err != nil {
		t.Fatal(err)
	}
	// Vote the second time
	curDeposit, curCandidates, curTime := dx.MultInt64(10), candidates[20:], time.Now().Unix()
	_, err = ProcessVote(stateDB, ctx, addr, curDeposit, curCandidates, curTime)
	if err != nil {
		t.Fatal(err)
	}
	// Check the result
	if err = checkProcessVote(stateDB, ctx, addr, curDeposit, curDeposit, curCandidates, 0, common.BigInt0); err != nil {
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
		setVoteDeposit(state, addr, test.prevDeposit)
		err = checkValidVote(state, addr, test.deposit, test.candidates)
		if err != test.expectedErr {
			t.Errorf("Test %d: error expect [%v], got [%v]", i, test.expectedErr, err)
		}
	}
}

func getPrototypeValidDelegator(addr common.Address) delegator {
	return delegator{
		addr:         addr,
		balance:      dx.MultInt64(10),
		frozenAssets: dx.MultInt64(4),
		deposit:      dx.MultInt64(4),
		candidates:   makeCandidates(30),
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
	thawEpoch int64, thawValue common.BigInt) error {

	// Check voteTrie
	voteTrie := ctx.VoteTrie()
	candidateBytes, err := voteTrie.TryGet(addr.Bytes())
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
	// Check the vote deposit
	voteDeposit := getVoteDeposit(state, addr)
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
	m[addr] = thawValue
	return checkThawingAddressAndValue(state, thawEpoch, m)
}

func newStateAndDposContextWithCandidate(num int) (*state.StateDB, *types.DposContext, []common.Address, error) {
	state, ctx, err := newStateAndDposContext()
	if err != nil {
		return nil, nil, nil, err
	}
	var addresses []common.Address
	for i := 0; i != num; i++ {
		addr := common.BigToAddress(common.NewBigIntUint64(uint64(i)).BigIntPtr())
		addAccountInState(state, addr, minDeposit, common.BigInt0)
		err = ProcessAddCandidate(state, ctx, addr, minDeposit, uint64(50))
		if err != nil {
			return nil, nil, nil, err
		}
		addresses = append(addresses, addr)
	}
	return state, ctx, addresses, nil
}
