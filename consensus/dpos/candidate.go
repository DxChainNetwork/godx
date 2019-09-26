// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/trie"
)

// ProcessAddCandidate adds a candidates to the DposContext and updated the related fields in stateDB
func ProcessAddCandidate(state stateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt,
	rewardRatio uint64) error {

	if err := checkValidCandidate(state, addr, deposit, rewardRatio); err != nil {
		return err
	}
	// Add the candidates to DposContext
	if err := ctx.BecomeCandidate(addr); err != nil {
		return err
	}
	// After validation, the candidates deposit could not decrease. Update the frozen asset field
	prevDeposit := GetCandidateDeposit(state, addr)
	if deposit.Cmp(prevDeposit) > 0 {
		diff := deposit.Sub(prevDeposit)
		AddFrozenAssets(state, addr, diff)
	}
	// Apply the candidates settings
	SetCandidateDeposit(state, addr, deposit)
	SetRewardRatioNumerator(state, addr, rewardRatio)
	return nil
}

// ProcessCancelCandidate cancel the addr being an candidates
func ProcessCancelCandidate(state stateDB, ctx *types.DposContext, addr common.Address, time int64) error {
	// Kick out the candidates in DposContext
	if err := ctx.KickoutCandidate(addr); err != nil {
		return err
	}
	// Mark the thawing address in the future
	prevDeposit := GetCandidateDeposit(state, addr)
	currentEpochID := CalculateEpochID(time)
	markThawingAddressAndValue(state, addr, currentEpochID, prevDeposit)
	// set the candidates deposit to 0
	SetCandidateDeposit(state, addr, common.BigInt0)
	return nil
}

// CandidateTxDepositValidation will validate the candidate apply transaction before sending it
func CandidateTxDepositValidation(state stateDB, data types.AddCandidateTxData, candidateAddress common.Address) error {
	// deposit validation
	if data.Deposit.Cmp(minDeposit) < 0 {
		return errCandidateInsufficientDeposit
	}

	// available balance validation
	candidateBalance := common.PtrBigInt(state.GetBalance(candidateAddress))
	candidateAvailableBalance := candidateBalance.Sub(GetFrozenAssets(state, candidateAddress))
	if candidateAvailableBalance.Cmp(data.Deposit) < 0 {
		return errCandidateInsufficientBalance
	}

	// previous candidate's deposit validation
	prevDeposit := GetCandidateDeposit(state, candidateAddress)
	if data.Deposit.Cmp(prevDeposit) < 0 {
		return errCandidateDecreasingDeposit
	}

	// previous candidate's reward distribution ratio validation
	prevRewardRatio := GetRewardRatioNumerator(state, candidateAddress)
	if data.RewardRatio < prevRewardRatio {
		return errCandidateDecreasingRewardRatio
	}

	return nil
}

// IsCandidate will check whether or not the given address is a candidate address
// by checking the candidate deposit
func IsCandidate(candidateAddress common.Address, state stateDB) bool {
	// check if the candidate deposit is not zero
	candidateDeposit := GetCandidateDeposit(state, candidateAddress)
	if candidateDeposit.Cmp(common.BigInt0) <= 0 {
		return false
	}

	// if the candidate deposit is not 0, meaning it is the candidate
	return true
}

// calcCandidateTotalVotes calculate the total votes for the candidates. The result include the deposit for the
// candidates himself and the delegated votes from delegator
func CalcCandidateTotalVotes(candidateAddr common.Address, state stateDB, delegateTrie *trie.Trie) common.BigInt {
	// Calculate the candidates deposit and delegatedVote
	candidateDeposit := GetCandidateDeposit(state, candidateAddr)
	delegatedVote := calcCandidateDelegatedVotes(state, candidateAddr, delegateTrie)
	// return the sum of candidates deposit and delegated vote
	return candidateDeposit.Add(delegatedVote)
}

// calcCandidateDelegatedVotes calculate the total votes from delegator for the candidates in the current dposContext
func calcCandidateDelegatedVotes(state stateDB, candidateAddr common.Address, dt *trie.Trie) common.BigInt {
	delegateIterator := trie.NewIterator(dt.PrefixIterator(candidateAddr.Bytes()))
	// loop through each delegator, get all votes
	delegatorVotes := common.BigInt0
	for delegateIterator.Next() {
		delegatorAddr := common.BytesToAddress(delegateIterator.Value)
		// Get the weighted vote
		vote := GetVoteDeposit(state, delegatorAddr)
		// add the weightedVote
		delegatorVotes = delegatorVotes.Add(vote)
	}
	return delegatorVotes
}

// getAllDelegatorForCandidate get all delegator who votes for the candidates
func getAllDelegatorForCandidate(ctx *types.DposContext, candidateAddr common.Address) []common.Address {
	dt := ctx.DelegateTrie()
	delegateIterator := trie.NewIterator(dt.PrefixIterator(candidateAddr.Bytes()))
	var addresses []common.Address
	for delegateIterator.Next() {
		delegatorAddr := common.BytesToAddress(delegateIterator.Value)
		addresses = append(addresses, delegatorAddr)
	}
	return addresses
}

// checkValidCandidate checks whether the candidateAddr in transaction is valid for becoming a candidates.
// If not valid, an error is returned.
func checkValidCandidate(state stateDB, candidateAddr common.Address, deposit common.BigInt, rewardRatio uint64) error {
	// Candidate deposit should be great than the threshold
	if deposit.Cmp(minDeposit) < 0 {
		return errCandidateInsufficientDeposit
	}
	// Reward ratio should be between 0 and 100
	if rewardRatio > RewardRatioDenominator {
		return errCandidateInvalidRewardRatio
	}
	// Deposit should be only increasing
	prevDeposit := GetCandidateDeposit(state, candidateAddr)
	if deposit.Cmp(prevDeposit) < 0 {
		return errCandidateDecreasingDeposit
	}
	// Reward ratio should also forbid decreasing
	prevRewardRatio := GetRewardRatioNumerator(state, candidateAddr)
	if rewardRatio < prevRewardRatio {
		return errCandidateDecreasingRewardRatio
	}

	// The candidate should have enough balance for the transaction
	availableBalance := GetAvailableBalance(state, candidateAddr)
	increasedDeposit := deposit.Sub(prevDeposit)
	if availableBalance.Cmp(increasedDeposit) < 0 {
		return errCandidateInsufficientBalance
	}
	return nil
}
