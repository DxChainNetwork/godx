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
	if prevDeposit.Cmp(common.BigInt0) == 0 {
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
	weight := calcVoteWeightByAddress(state, addr, time)
	setVoteWeight(state, addr, weight)

	return successVote, nil
}

// calcVoteWeightByAddress calculate the vote weight by address
func calcVoteWeightByAddress(state stateDB, addr common.Address, curTime int64) float64 {
	timeLastVote := getLastVoteTime(state, addr)
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
