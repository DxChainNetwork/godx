// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
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
	SetRewardRatioNumerator(state, addr, 0)
	return nil
}

// CandidateTxDataValidation will validate the candidate apply transaction before sending it
func CandidateTxDataValidation(state stateDB, data types.AddCandidateTxData, candidateAddress common.Address) error {
	return checkValidCandidate(state, candidateAddress, data.Deposit, data.RewardRatio)
}

// IsCandidate will check whether or not the given address is a candidate address
func IsCandidate(candidateAddress common.Address, header *types.Header, diskDB ethdb.Database) bool {
	// re-construct trieDB and get the candidateTrie
	trieDb := trie.NewDatabase(diskDB)
	candidateTrie, err := types.NewCandidateTrie(header.DposContext.CandidateRoot, trieDb)
	if err != nil {
		return false
	}
	return isCandidate(candidateTrie, candidateAddress)
}

// isCandidate determines whether the addr is a candidate from a candidateTrie
func isCandidate(candidateTrie *trie.Trie, addr common.Address) bool {
	// check if the candidate exists
	if value, err := candidateTrie.TryGet(addr.Bytes()); err != nil || value == nil {
		return false
	}
	return true
}

// CalcCandidateTotalVotes calculate the total votes for the candidates. The result include the deposit for the
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

// calculateValidatorDepositReward calculates the deposit bonus for validator when producing a new block
func calculateValidatorDepositReward(state stateDB, addr common.Address) common.BigInt {
	rewardPerBlock := common.NewBigInt(0)
	deposit := GetValidatorDepositLastEpoch(state, addr)

	/*
		deposit < 1e3 dx: rewardPerBlock = 1 dx
		deposit < 1e6 dx: rewardPerBlock = 2 dx
		deposit < 1e9 dx: rewardPerBlock = 3 dx
		deposit >= 1e9 dx: rewardPerBlock = 5 dx
	*/
	switch {
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e3)) == -1:
		rewardPerBlock = minRewardPerBlock
		break
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e6)) == -1:
		rewardPerBlock = minRewardPerBlock.MultInt64(2)
		break
	case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e9)) == -1:
		rewardPerBlock = minRewardPerBlock.MultInt64(3)
		break
	default:
		rewardPerBlock = minRewardPerBlock.MultInt64(5)
	}

	return rewardPerBlock
}
