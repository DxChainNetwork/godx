// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

// ProcessVote process the process request for state and dpos context
func ProcessVote(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	candidates []common.Address, time int64) (int, error) {
	prevDeposit := getVoteDeposit(state, addr)
	// If previously voted, cannot vote again
	if prevDeposit.Cmp(common.BigInt0) != 0 {
		return 0, ErrAlreadyVote
	}
	// Vote the candidates
	successVote, err := ctx.Vote(addr, candidates)
	if err != nil {
		return 0, err
	}
	// Update vote deposit
	setVoteDeposit(state, addr, deposit)
	setLastVoteTime(state, addr, time)
	// Calculate and apply the vote weight
	weight := calcVoteWeightForDelegator(state, addr, time)
	setVoteWeight(state, addr, weight)

	return successVote, nil
}

// ProcessCancelVote process the cancel vote request for state and dpos context
func ProcessCancelVote(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	if err := ctx.CancelVote(addr); err != nil {
		return err
	}
	currentEpoch := CalculateEpochID(time)
	MarkThawingAddress(state, addr, currentEpoch, PrefixVoteThawing)
	return nil
}

// calcVoteWeightForDelegator calculate the vote weight by address
func calcVoteWeightForDelegator(state stateDB, delegatorAddr common.Address, curTime int64) float64 {
	timeLastVote := getLastVoteTime(state, delegatorAddr)
	return calcVoteWeight(timeLastVote, curTime)
}

// calcVoteWeight calculate the the vote weight. The vote weight decays at a rate of AttenuationRatioPerEpoch
// per epoch
func calcVoteWeight(timeLastVote, curTime int64) float64 {
	lastVoteEpoch := CalculateEpochID(timeLastVote)
	currentEpoch := CalculateEpochID(curTime)
	// Calculate the decay
	epochPassed := currentEpoch - lastVoteEpoch
	if epochPassed <= 0 {
		return 1
	}
	weight := math.Pow(AttenuationRatioPerEpoch, float64(epochPassed))
	if weight < MinVoteWeightRatio {
		weight = MinVoteWeightRatio
	}
	return weight
}

// getVoteWithWeight get the vote after applying the voteWeight for a delegator addr
func getVoteWithWeight(state stateDB, delegatorAddr common.Address) common.BigInt {
	// get the vote deposit
	voteDeposit := getVoteDeposit(state, delegatorAddr)
	// get the voteWeight
	voteWeight := getVoteWeight(state, delegatorAddr)
	if voteWeight == 0 {
		return common.BigInt0
	}
	// apply vote weight and return the actual vote deposit
	weightedVote := voteDeposit.MultFloat64(voteWeight)
	return weightedVote
}
