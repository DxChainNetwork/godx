// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/trie"
)

// ProcessVote process the process request for state and dpos context
func ProcessVote(state stateDB, ctx *types.DposContext, addr common.Address, voteData *types.VoteTxData, currentBlockTime, voteTime int64) (int, error) {

	// Validation: voting with 0 deposit is not allowed
	if err := checkValidVote(state, addr, *voteData); err != nil {
		return 0, err
	}
	// Vote the candidates
	successVote, err := ctx.Vote(addr, voteData.Candidates)
	if err != nil {
		return 0, err
	}
	// Compare the new deposit with the previous deposit. Different strategy is applied for
	// different condition. Note if previous deposit is the same as the new deposit, no frozen
	// or thawing fields need to be updated
	prevDeposit := GetVoteDeposit(state, addr)
	if voteData.Deposit.Cmp(prevDeposit) < 0 {
		// If new deposit is smaller than previous deposit, the diff will be thawed after
		// ThawingEpochDuration
		diff := prevDeposit.Sub(voteData.Deposit)
		epoch := CalculateEpochID(currentBlockTime)
		markThawingAddressAndValue(state, addr, epoch, diff)
	} else if voteData.Deposit.Cmp(prevDeposit) > 0 {
		// If the new deposit is larger than previous deposit, the diff will be added directly
		// to the frozenAssets
		diff := voteData.Deposit.Sub(prevDeposit)
		AddFrozenAssets(state, addr, diff)
	}
	// Update vote deposit
	SetVoteDeposit(state, addr, voteData.Deposit)

	// store vote duration and vote time
	SetVoteDuration(state, addr, voteData.Duration)
	SetVoteTime(state, addr, uint64(voteTime))

	return successVote, nil
}

// ProcessCancelVote process the cancel vote request for state and dpos context
func ProcessCancelVote(state stateDB, ctx *types.DposContext, addr common.Address, currentBlockTime, now int64) error {

	// check whether the given delegator remains in locked duration
	if err := CheckVoteDuration(state, addr, uint64(now)); err != nil {
		return fmt.Errorf("failed to process cancel vote transaction, error: %v", err)
	}

	if err := ctx.CancelVote(addr); err != nil {
		return err
	}
	prevDeposit := GetVoteDeposit(state, addr)
	currentEpoch := CalculateEpochID(currentBlockTime)
	markThawingAddressAndValue(state, addr, currentEpoch, prevDeposit)
	SetVoteDeposit(state, addr, common.BigInt0)

	// calculate the deposit bonus sent by dx reward account
	bonus := calculateDelegatorDepositReward(state, addr)

	// directly add reward to delegator, just like adding block reward to validator
	state.AddBalance(addr, bonus.BigIntPtr())

	// set vote duration and time to 0
	SetVoteDuration(state, addr, 0)
	SetVoteTime(state, addr, 0)
	return nil
}

// VoteTxDepositValidation will validate the vote transaction before sending it
func VoteTxDepositValidation(state stateDB, delegatorAddress common.Address, voteData types.VoteTxData) error {
	return checkValidVote(state, delegatorAddress, voteData)
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
func checkValidVote(state stateDB, delegatorAddr common.Address, voteData types.VoteTxData) error {
	if voteData.Deposit.Cmp(common.BigInt0) <= 0 {
		return errVoteZeroOrNegativeDeposit
	}
	if len(voteData.Candidates) == 0 {
		return errVoteZeroCandidates
	}
	if len(voteData.Candidates) > MaxVoteCount {
		return errVoteTooManyCandidates
	}
	// The delegator should have enough balance for vote if he want to increase the deposit
	prevVoteDeposit := GetVoteDeposit(state, delegatorAddr)
	if voteData.Deposit.Cmp(prevVoteDeposit) > 0 {
		availableBalance := GetAvailableBalance(state, delegatorAddr)
		diff := voteData.Deposit.Sub(prevVoteDeposit)
		if availableBalance.Cmp(diff) < 0 {
			return errVoteInsufficientBalance
		}
	}

	// check if vote duration is too small
	if voteData.Duration < MinVoteLockDuration {
		return errVoteLockDurationTooLow
	}

	return nil
}

// calculateDelegatorDepositReward calculate the deposit bonus for delegator
func calculateDelegatorDepositReward(state stateDB, addr common.Address) common.BigInt {
	rewardRatio := float64(0)
	deposit := GetVoteLastEpoch(state, addr)
	duration := GetVoteDuration(state, addr)

	/*
		duration >= 160 epoch : rewardRatio = 8%
		duration >= 80 epoch : rewardRatio = 6%
		duration >= 40 epoch : rewardRatio = 4%
		duration >= 20 epoch : rewardRatio = 2%
		duration >= 10 epoch : rewardRatio = 1%
		duration >= 5 epoch : rewardRatio = 0.5%
	*/
	switch {
	case duration >= Epoch160:
		rewardRatio = Ratio160
		break
	case duration >= Epoch80:
		rewardRatio = Ratio80
		break
	case duration >= Epoch40:
		rewardRatio = Ratio40
		break
	case duration >= Epoch20:
		rewardRatio = Ratio20
		break
	case duration >= Epoch10:
		rewardRatio = Ratio10
		break
	case duration >= Epoch5:
		rewardRatio = Ratio5
		break
	}

	return deposit.MultFloat64(rewardRatio)
}

// CheckVoteDuration check whether now remains in locked duration
func CheckVoteDuration(state stateDB, addr common.Address, now uint64) error {
	duration := GetVoteDuration(state, addr)
	voteTime := GetVoteTime(state, addr)
	if (voteTime + duration) >= now {
		return fmt.Errorf("delegator remains in locked duration")
	}
	return nil
}
