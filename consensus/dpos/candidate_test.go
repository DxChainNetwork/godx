// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

// candidate is the structure of necessary information about a candidate
type candidate struct {
	address         common.Address
	balance         common.BigInt
	frozenAssets    common.BigInt
	deposit         common.BigInt
	rewardRatio     uint64
	prevDeposit     common.BigInt
	prevRewardRatio uint64
}

// TestProcessAddCandidate test the normal case of ProcessAddCandidate.
// Two scenarios are tests: 1. a new candidate 2. previously already a candidate
func TestProcessAddCandidate(t *testing.T) {
	candidateAddr := common.BytesToAddress([]byte{1})
	state, dposCtx, err := newStateAndDposContext()
	if err != nil {
		t.Fatal(err)
	}
	c := newCandidatePrototype(candidateAddr)
	addOrigCandidateInState(state, c)
	err = ProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio)
	if err != nil {
		t.Fatal(err)
	}
	// Check the result of being a candidate
	err = checkProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio, c.deposit)
	if err != nil {
		t.Fatal(err)
	}
	// The candidate submit a candidate transaction for the second time. Increase
	// the rewardRatio and deposit
	c.deposit = c.deposit.AddInt64(1e18)
	c.rewardRatio = c.rewardRatio + 1
	err = ProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio)
	if err != nil {
		t.Fatal(err)
	}
	err = checkProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio, c.deposit)
	if err != nil {
		t.Fatal(err)
	}
}

// TestProcessAddCandidateError test the error case for processAddCandidate.
// Note that not all error cases are tested in this function. More error cases please see
// the test case TestCheckValidCandidate
func TestProcessAddCandidateError(t *testing.T) {
	candidateAddr := common.BytesToAddress([]byte{1})
	state, dposCtx, err := newStateAndDposContext()
	if err != nil {
		t.Fatal(err)
	}
	c := candidatePrototype(candidateAddr)
	err = addOrigCandidateData(state, dposCtx, c)
	if err != nil {
		t.Fatal(err)
	}
	// Decrease the deposit and add candidate.
	c.deposit = c.prevDeposit.SubInt64(1000)
	err = ProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio)
	if err == nil {
		t.Fatal("decrease the deposit should report error")
	}
}

// TestProcessCancelCandidate test the functionality of ProcessCancelCandidate
func TestProcessCancelCandidate(t *testing.T) {
	addr := common.BytesToAddress([]byte{1})
	state, dposCtx, err := newStateAndDposContext()
	if err != nil {
		t.Fatal(err)
	}
	c := candidatePrototype(addr)
	addAccountInState(state, c.address, c.balance, c.frozenAssets)
	if err = ProcessAddCandidate(state, dposCtx, c.address, c.deposit, c.rewardRatio); err != nil {
		t.Fatal(err)
	}
	// cancel the candidate and commit
	curTime := time.Now().Unix()
	if err = ProcessCancelCandidate(state, dposCtx, addr, curTime); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Commit(true); err != nil {
		t.Fatal(err)
	}
	// check the results
	// The candidate should not be in the candidate trie
	if b, err := dposCtx.CandidateTrie().TryGet(addr.Bytes()); err == nil && b != nil && len(b) != 0 {
		t.Fatal("after cancel candidate, the candidate still in candidate trie")
	}
	// Check thawing logic
	m := map[common.Address]common.BigInt{
		addr: c.deposit,
	}
	epoch := calcThawingEpoch(CalculateEpochID(curTime))
	if err = checkThawingAddressAndValue(state, epoch, m); err != nil {
		t.Fatal(err)
	}
	// Check deposit
	if deposit := getCandidateDeposit(state, addr); deposit.Cmp(common.BigInt0) != 0 {
		t.Fatalf("after cancel candidate, the candidate deposit not zero: %v", deposit)
	}
}

func TestCheckValidCandidate(t *testing.T) {
	candidateAddr := common.BytesToAddress([]byte{1})
	tests := []struct {
		modify    func(*candidate)
		expectErr error
	}{
		// normal case
		{
			func(c *candidate) {},
			nil,
		},
		// less balance
		{
			func(c *candidate) { c.balance = minDeposit.MultFloat64(1.5) },
			errCandidateInsufficientBalance,
		},
		// more frozenAssets
		{
			func(c *candidate) { c.frozenAssets = minDeposit.MultFloat64(3.5) },
			errCandidateInsufficientBalance,
		},
		// higher previous deposit
		{
			func(c *candidate) { c.prevDeposit = minDeposit.MultFloat64(2.5) },
			errCandidateDecreasingDeposit,
		},
		// higher previous reward ratio
		{
			func(c *candidate) { c.prevRewardRatio = 70 },
			errCandidateDecreasingRewardRatio,
		},
		// Invalid reward ratio
		{
			func(c *candidate) { c.rewardRatio = 101 },
			errCandidateInvalidRewardRatio,
		},
	}
	for i, test := range tests {
		c := candidatePrototype(candidateAddr)
		test.modify(&c)
		state, err := newStateDB(ethdb.NewMemDatabase())
		if err != nil {
			t.Fatal(err)
		}
		addOrigCandidateInState(state, c)
		err = checkValidCandidate(state, c.address, c.deposit, c.rewardRatio)
		if err != test.expectErr {
			t.Errorf("check valid candidate %d error: \nexpect [%v]\ngot [%v]", i, test.expectErr, err)
		}
	}
}

// newCandidatePrototype get a prototype of a candidate which is previously not a candidate
func newCandidatePrototype(addr common.Address) candidate {
	return candidate{
		address:         addr,
		balance:         minDeposit.MultInt64(4),
		frozenAssets:    common.BigInt0,
		deposit:         minDeposit,
		rewardRatio:     uint64(50),
		prevDeposit:     common.BigInt0,
		prevRewardRatio: uint64(0),
	}
}

// candidatePrototype get a prototype of a valid candidate info which is a candidate in previous
// epoch
func candidatePrototype(addr common.Address) candidate {
	return candidate{
		address:         addr,
		balance:         minDeposit.MultInt64(4),
		frozenAssets:    minDeposit,
		deposit:         minDeposit.MultInt64(2),
		rewardRatio:     uint64(50),
		prevDeposit:     minDeposit,
		prevRewardRatio: uint64(30),
	}
}

// addOrigCandidateData add the candidate data to StateDB and DposContext
func addOrigCandidateData(state *state.StateDB, ctx *types.DposContext, c candidate) error {
	if err := ctx.BecomeCandidate(c.address); err != nil {
		return err
	}
	addOrigCandidateInState(state, c)
	return nil
}

// addOrigCandidateInState create the original candidate data in state db.
func addOrigCandidateInState(state *state.StateDB, c candidate) {
	addr := c.address
	state.CreateAccount(addr)
	state.SetBalance(addr, c.balance.BigIntPtr())
	if c.frozenAssets.Cmp(common.BigInt0) != 0 {
		setFrozenAssets(state, addr, c.frozenAssets)
	}
	if c.prevDeposit.Cmp(common.BigInt0) != 0 {
		setCandidateDeposit(state, addr, c.prevDeposit)
	}
	if c.prevRewardRatio != uint64(0) {
		setRewardRatioNumerator(state, addr, c.prevRewardRatio)
	}
}

// checkProcessAddCandidate checks whether the addr stores the correct information in
// DposContext and state
func checkProcessAddCandidate(state *state.StateDB, ctx *types.DposContext, addr common.Address,
	expectedDeposit common.BigInt, expectedRewardRatio uint64, expectedFrozenAssets common.BigInt) error {
	// Check whether the candidate address is in the candidateTrie
	ct := ctx.CandidateTrie()
	if b, err := ct.TryGet(addr.Bytes()); err != nil || !bytes.Equal(b, addr.Bytes()) {
		return fmt.Errorf("addr not in candidate trie")
	}
	// Check expectedRewardRatio
	if rewardRatio := getRewardRatioNumerator(state, addr); rewardRatio != expectedRewardRatio {
		return fmt.Errorf("reward ratio not expected. Got %v, Expect %v", rewardRatio, expectedRewardRatio)
	}
	// Check expectedDeposit
	if deposit := getCandidateDeposit(state, addr); deposit.Cmp(expectedDeposit) != 0 {
		return fmt.Errorf("deposit not expected. Got %v, Expect %v", deposit, expectedDeposit)
	}
	// Check frozenAssets
	if frozenAssets := GetFrozenAssets(state, addr); frozenAssets.Cmp(expectedFrozenAssets) != 0 {
		return fmt.Errorf("frozenAssets not expected. Got %v, Expect %v", frozenAssets, expectedFrozenAssets)
	}
	return nil
}
