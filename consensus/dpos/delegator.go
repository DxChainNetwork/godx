// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	time2 "time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/trie"
)

// ProcessVote process the process request for state and dpos context
func ProcessVote(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	candidates []common.Address, duration uint64, time int64) (int, error) {

	// Validation: voting with 0 deposit is not allowed
	if err := checkValidVote(state, addr, deposit, candidates, duration); err != nil {
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

	// store vote duration and vote time
	SetVoteDuration(state, addr, duration)
	SetVoteTime(state, addr, uint64(time2.Now().Unix()))

	return successVote, nil
}

// ProcessCancelVote process the cancel vote request for state and dpos context
func ProcessCancelVote(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	// check whether the given delegator remains in locked duration
	duration := GetVoteDuration(state, addr)
	voteTime := GetVoteTime(state, addr)
	now := uint64(time2.Now().Unix())
	if (voteTime + duration) >= now {
		return fmt.Errorf("failed to process cancel vote for remaining in locked duration")
	}

	if err := ctx.CancelVote(addr); err != nil {
		return err
	}
	prevDeposit := GetVoteDeposit(state, addr)
	currentEpoch := CalculateEpochID(time)
	markThawingAddressAndValue(state, addr, currentEpoch, prevDeposit)
	SetVoteDeposit(state, addr, common.BigInt0)

	// calculate the deposit bonus sent by dx reward account
	bonus := calculateDepositReward(state, addr, now, voteTime)

	// TODO: if reward account balance is not enough for paying bonus, just skip transferring ??
	// transfer from reward account to delegator
	if bonus.BigIntPtr().Cmp(state.GetBalance(rewardAccount)) == -1 {
		state.SubBalance(rewardAccount, bonus.BigIntPtr())
		state.AddBalance(addr, bonus.BigIntPtr())
	}

	// set vote duration and time to 0
	SetVoteDuration(state, addr, 0)
	SetVoteTime(state, addr, 0)
	return nil
}

// VoteTxDepositValidation will validate the vote transaction before sending it
func VoteTxDepositValidation(state stateDB, delegatorAddress common.Address, voteData types.VoteTxData) error {
	return checkValidVote(state, delegatorAddress, voteData.Deposit, voteData.Candidates, voteData.Duration)
}

// HasVoted will check whether the provided delegator address is voted
func HasVoted(delegatorAddress common.Address, header *types.Header, diskDB ethdb.Database) bool {
	// re-construct trieDB and get the voteTrie
	trieDb := trie.NewDatabase(diskDB)
	voteTrie, err := types.NewVoteTrie(header.DposContext.VoteRoot, trieDb)
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
func checkValidVote(state stateDB, delegatorAddr common.Address, deposit common.BigInt, candidates []common.Address, duration uint64) error {
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

	// check if vote duration is too small
	if duration < MinVoteLockDuration {
		return errVoteLockDurationTooLow
	}

	return nil
}

// calculateDepositReward calculate the deposit bonus sent by dx reward account
func calculateDepositReward(state stateDB, addr common.Address, now, voteTime uint64) common.BigInt {
	deposit := GetVoteLastEpoch(state, addr)
	realDepositDuration := now - voteTime
	rewardPerEpoch := common.NewBigInt(0)
	passedEpochs := realDepositDuration / uint64(EpochInterval)

	/*
		deposit < 1e3 dx: rewardPerEpoch = 1 dx
		deposit < 1e6 dx: rewardPerEpoch = 10 dx
		deposit < 1e9 dx: rewardPerEpoch = 100 dx
		deposit >= 1e9 dx: rewardPerEpoch = 1000 dx
	*/
	switch {
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e3)) == -1:
		rewardPerEpoch = minRewardPerEpoch
		break
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e6)) == -1:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(10)
		break
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e9)) == -1:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(100)
		break
	default:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(1000)
	}

	return rewardPerEpoch.MultUint64(passedEpochs)
}
