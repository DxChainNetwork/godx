// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

// AddCandidate adds a candidate
func AddCandidate(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	rewardRatio uint64) error {
	prevDeposit := getCandidateDeposit(state, addr)
	if prevDeposit.Cmp(common.BigInt0) == 0 {
		return fmt.Errorf("the address is already a candidate")
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
