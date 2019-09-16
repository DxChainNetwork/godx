// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/trie"
)

// ProcessAddCandidate adds a candidate to the DposContext and updated the related fields in stateDB
func ProcessAddCandidate(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	rewardRatio uint64) error {
	prevDeposit := getCandidateDeposit(state, addr)
	if prevDeposit.Cmp(common.BigInt0) != 0 {
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
	markThawingAddress(state, addr, currentEpochID, PrefixCandidateThawing)
	return nil
}

// calcCandidateTotalVotes calculate the total votes for the candidate. The result include the deposit for the
// candidate himself and the delegated votes from delegator
func (ec *EpochContext) calcCandidateTotalVotes(candidateAddr common.Address) common.BigInt {
	state := ec.stateDB
	// Calculate the candidate deposit and delegatedVote
	candidateDeposit := getCandidateDeposit(state, candidateAddr)
	delegatedVote := ec.calcCandidateDelegatedVotes(state, candidateAddr)
	// return the sum of candidate deposit and delegated vote
	return candidateDeposit.Add(delegatedVote)
}

// calcCandidateDelegatedVotes calculate the total votes from delegator for the candidate in the current dposContext
func (ec *EpochContext) calcCandidateDelegatedVotes(state stateDB, candidateAddr common.Address) common.BigInt {
	dt := ec.DposContext.DelegateTrie()
	delegateIterator := trie.NewIterator(dt.PrefixIterator(candidateAddr.Bytes()))

	// loop through each delegator, get all votes
	delegatorVotes := common.BigInt0
	for delegateIterator.Next() {
		delegatorAddr := common.BytesToAddress(delegateIterator.Value)
		// Get the weighted vote
		weightedVote := getVoteWithWeight(state, delegatorAddr)
		// add the weightedVote
		delegatorVotes = delegatorVotes.Add(weightedVote)
	}
	return delegatorVotes
}
