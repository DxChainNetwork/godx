// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

// ProcessVote process the process request for state and dpos context
func ProcessVote(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	candidates []common.Address, time int64) (int, error) {

	// Validation: voting with 0 deposit is not allowed
	if err := checkValidVote(state, addr, deposit, candidates); err != nil {
		return 0, err
	}
	// Vote the candidates
	successVote, err := ctx.Vote(addr, candidates)
	if err != nil {
		return 0, err
	}
	// Compare the new deposit with the previous deposit. Different strategy is applied for
	// different condition. Note if previous deposit is the same as the new deposit, no frozen
	// or thawing fields need to be updated
	prevDeposit := GetVoteDeposit(state, addr)
	if deposit.Cmp(prevDeposit) < 0 {
		// If new deposit is smaller than previous deposit, the diff will be thawed after
		// ThawingEpochDuration
		diff := prevDeposit.Sub(deposit)
		epoch := CalculateEpochID(time)
		markThawingAddressAndValue(state, addr, epoch, diff)
	} else if deposit.Cmp(prevDeposit) > 0 {
		// If the new deposit is larger than previous deposit, the diff will be added directly
		// to the frozenAssets
		diff := deposit.Sub(prevDeposit)
		AddFrozenAssets(state, addr, diff)
	}
	// Update vote deposit
	SetVoteDeposit(state, addr, deposit)

	return successVote, nil
}

// ProcessCancelVote process the cancel vote request for state and dpos context
func ProcessCancelVote(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	if err := ctx.CancelVote(addr); err != nil {
		return err
	}
	prevDeposit := GetVoteDeposit(state, addr)
	currentEpoch := CalculateEpochID(time)
	markThawingAddressAndValue(state, addr, currentEpoch, prevDeposit)
	SetVoteDeposit(state, addr, common.BigInt0)
	return nil
}

// VoteTxDepositValidation will validate the vote transaction before sending it
func VoteTxDepositValidation(state stateDB, delegatorAddress common.Address, voteData types.VoteTxData) error {
	return checkValidVote(state, delegatorAddress, voteData.Deposit, voteData.Candidates)
}

// HasVoted will check whether the provided delegator address is voted
func HasVoted(delegatorAddress common.Address, header *types.Header, diskDB ethdb.Database) bool {
	// re-construct trieDB and get the voteTrie
	voteTrie, err := types.NewDposDb(diskDB).OpenTrie(header.DposContext.VoteRoot)
	if err != nil {
		return false
	}

	// check if the delegator has voted
	if value, err := voteTrie.TryGet(delegatorAddress.Bytes()); err != nil || value == nil {
		return false
	}

	// otherwise, means the delegator has voted
	return true
}

// checkValidVote checks whether the input argument is valid for a vote transaction
func checkValidVote(state stateDB, delegatorAddr common.Address, deposit common.BigInt, candidates []common.Address) error {
	if deposit.Cmp(common.BigInt0) <= 0 {
		return errVoteZeroOrNegativeDeposit
	}
	if len(candidates) == 0 {
		return errVoteZeroCandidates
	}
	if len(candidates) > MaxVoteCount {
		return errVoteTooManyCandidates
	}
	// The delegator should have enough balance for vote if he want to increase the deposit
	prevVoteDeposit := GetVoteDeposit(state, delegatorAddr)
	if deposit.Cmp(prevVoteDeposit) > 0 {
		availableBalance := GetAvailableBalance(state, delegatorAddr)
		diff := deposit.Sub(prevVoteDeposit)
		if availableBalance.Cmp(diff) < 0 {
			return errVoteInsufficientBalance
		}
	}
	return nil
}
