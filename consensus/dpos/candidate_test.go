// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"testing"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
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

func TestProcessAddCandidate(t *testing.T) {
	candidateAddr := common.BytesToAddress([]byte{1})
	db := ethdb.NewMemDatabase()
	state, err := newStateDB(db)
	if err != nil {
		t.Fatal(err)
	}
	dposCtx, err := types.NewDposContext(db)

	c := newCandidatePrototype(candidateAddr)
	createOrigCandidateInState(state, c)
	err := ProcessAddCandidate(state, dposCtx, candidateAddr, c.deposit, c.rewardRatio)
	if err != nil {
		t.Fatal(err)
	}
	// Check the result of being a candidate
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
	}
	for i, test := range tests {
		c := candidatePrototype(candidateAddr)
		test.modify(&c)
		state, err := newStateDB(ethdb.NewMemDatabase())
		if err != nil {
			t.Fatal(err)
		}
		createOrigCandidateInState(state, c)
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

// createOrigCandidateInState create the original candidate data in state db.
func createOrigCandidateInState(state *state.StateDB, c candidate) {
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

//
//func checkProcessAddCandidate(state *state.StateDB, ctx *types.DposContext, addr common.Address,
//	expectedRewardRatio uint64, expectedDeposit common.BigInt, expectedFrozenAssets common.BigInt) error {
//
//}
