// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

// ProcessAddCandidate adds a candidate to the DposContext and updated the related fields in stateDB
func ProcessAddCandidate(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	rewardRatio uint64) error {
	prevDeposit := getCandidateDeposit(state, addr)
	if prevDeposit.Cmp(common.BigInt0) == 0 {
		return ErrAlreadyCandidate
	}
	// Add the candidate to DposContext
	if err := ctx.BecomeCandidate(addr); err != nil {
		return err
	}
	// Apply the candidate settings
	setCandidateDeposit(state, addr, deposit)
	setCandidateRewardRatioNumerator(state, addr, rewardRatio)
	return nil
}

// ProcessCancelCandidate cancel the addr being an candidate
func ProcessCancelCandidate(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	// Kick out the candidate in DposContext
	if err := ctx.KickoutCandidate(addr); err != nil {
		return err
	}
	// Mark the thawing address in the future
	currentEpochID := CalculateEpochID(time)
	MarkThawingAddress(state, addr, currentEpochID, PrefixCandidateThawing)
	return nil
}
