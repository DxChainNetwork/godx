// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

// ProcessVote process the process request for state and dpos context
func ProcessVote(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	candidates []common.Address, time int64) (int, error) {

	// Validation: voting with 0 deposit is not allowed
	if err := checkValidVote(deposit, candidates); err != nil {
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
	prevDeposit := getVoteDeposit(state, addr)
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
		addFrozenAssets(state, addr, diff)
	}
	// Update vote deposit
	setVoteDeposit(state, addr, deposit)

	return successVote, nil
}

// ProcessCancelVote process the cancel vote request for state and dpos context
func ProcessCancelVote(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	if err := ctx.CancelVote(addr); err != nil {
		return err
	}
	prevDeposit := getVoteDeposit(state, addr)
	currentEpoch := CalculateEpochID(time)
	markThawingAddressAndValue(state, addr, currentEpoch, prevDeposit)
	setVoteDeposit(state, addr, common.BigInt0)
	return nil
}

func VoteTxDepositValidation(state stateDB, delegatorAddress common.Address, voteData types.VoteTxData) error {
	// validate the vote deposit and available balance
	delegatorBalance := common.PtrBigInt(state.GetBalance(delegatorAddress))
	delegatorAvailableBalance := delegatorBalance.Sub(getFrozenAssets(state, delegatorAddress))
	if delegatorAvailableBalance.Cmp(voteData.Deposit) < 0 {
		return errDelegatorInsufficientBalance
	}
	return nil
}

func HasVoted(delegatorAddress common.Address, state stateDB) bool {
	// check if the delegator has voted
	voteDeposit := getVoteDeposit(state, delegatorAddress)
	if voteDeposit.Cmp(common.BigInt0) <= 0 {
		return false
	}

	// if the vote deposit is greater than 0, meaning the delegator has voted
	return true
}

// checkValidVote checks whether the input argument is valid for a vote transaction
func checkValidVote(deposit common.BigInt, candidates []common.Address) error {
	if deposit.Cmp(common.BigInt0) <= 0 {
		return errVoteZeroOrNegativeDeposit
	}
	if len(candidates) == 0 {
		return errVoteZeroCandidates
	}
	if len(candidates) > MaxVoteCount {
		return errVoteTooManyCandidates
	}
	return nil
}
