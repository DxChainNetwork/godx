// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"github.com/DxChainNetwork/godx/common"
)

type stateDB interface {
	GetState(addr common.Address, key common.Hash) common.Hash
	SetState(addr common.Address, key, value common.Hash)
	ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool)
	Exist(addr common.Address) bool
	CreateAccount(addr common.Address)
	SetNonce(addr common.Address, nonce uint64)
}

var (
	// KeyRewardRatioNumerator is the key of block reward ration numerator indicates the percent of share validator with its delegators
	KeyRewardRatioNumerator = common.BytesToHash([]byte("reward-ratio-numerator"))

	// KeyVoteDeposit is the key of vote deposit
	KeyVoteDeposit = common.BytesToHash([]byte("vote-deposit"))

	// KeyVoteWeight is the weight ratio of vote
	KeyVoteWeight = common.BytesToHash([]byte("vote-weight"))

	// KeyCandidateDeposit is the key of candidate deposit
	KeyCandidateDeposit = common.BytesToHash([]byte("candidate-deposit"))

	// KeyLastVoteTime is the key of last vote time
	KeyLastVoteTime = common.BytesToHash([]byte("last-vote-time"))

	// KeyTotalVoteWeight is the key of total vote weight for every candidate
	KeyTotalVoteWeight = common.BytesToHash([]byte("total-vote-weight"))
)

// getCandidateDeposit get the candidate deposit of the addr from the state
func getCandidateDeposit(state stateDB, addr common.Address) common.BigInt {
	depositHash := state.GetState(addr, KeyCandidateDeposit)
	return common.PtrBigInt(depositHash.Big())
}

// setCandidateDeposit set the candidate deposit of the addr in the state
func setCandidateDeposit(state stateDB, addr common.Address, deposit common.BigInt) {
	hash := common.BigToHash(deposit.BigIntPtr())
	state.SetState(addr, KeyCandidateDeposit, hash)
}

// addCandidateDepsoit add the candidate deposit of diff value for the addr in state
func addCandidateDepsoit(state stateDB, addr common.Address, diff common.BigInt) {
	prevDeposit := getCandidateDeposit(state, addr)
	newDeposit := prevDeposit.Add(diff)
	setCandidateDeposit(state, addr, newDeposit)
}

// getCandidateDeposit get the vote deposit of the addr from the state
func getVoteDeposit(state stateDB, addr common.Address) common.BigInt {
	depositHash := state.GetState(addr, KeyVoteDeposit)
	return common.PtrBigInt(depositHash.Big())
}

// setVoteDeposit set the vote deposit of the addr in the state
func setVoteDeposit(state stateDB, addr common.Address, deposit common.BigInt) {
	hash := common.BigToHash(deposit.BigIntPtr())
	state.SetState(addr, KeyVoteDeposit, hash)
}

// addVoteDeposit add the vote deposit of diff value for the addr in state
func addVoteDeposit(state stateDB, addr common.Address, diff common.BigInt) {
	prevDeposit := getVoteDeposit(state, addr)
	newDeposit := prevDeposit.Add(diff)
	setVoteDeposit(state, addr, newDeposit)
}

// getCandidateRewardRatioNumerator get the reward ratio for a candidate for the addr in state.
// The value is used in calculating block reward for miner and his delegator
func getCandidateRewardRatioNumerator(state stateDB, addr common.Address) uint64 {
	rewardRatioHash := state.GetState(addr, KeyRewardRatioNumerator)
	return hashToUint64(rewardRatioHash)
}

// setCandidateRewardRatioNumerator set the CandidateRewardRatioNumerator for the addr
// in state
func setCandidateRewardRatioNumerator(state stateDB, addr common.Address, value uint64) {
	hash := uint64ToHash(value)
	state.SetState(addr, KeyRewardRatioNumerator, hash)
}

// getLastVoteTime return the last vote time for the address in the stateDB
func getLastVoteTime(state stateDB, addr common.Address) int64 {
	timeHash := state.GetState(addr, KeyLastVoteTime)
	return int64(hashToUint64(timeHash))
}

// setLastVoteTime set the last vote time for the address to the specified value time in the
// stateDB
func setLastVoteTime(state stateDB, addr common.Address, time int64) {
	timeHash := uint64ToHash(uint64(time))
	state.SetState(addr, KeyLastVoteTime, timeHash)
}

// getVoteWeight get the vote weight for the address in the stateDB
func getVoteWeight(state stateDB, addr common.Address) float64 {
	ratioHash := state.GetState(addr, KeyVoteWeight)
	return hashToFloat64(ratioHash)
}

// setVoteWeight set the vote weight to the value of the addr in stateDB
func setVoteWeight(state stateDB, addr common.Address, value float64) {
	ratioHash := float64ToHash(value)
	state.SetState(addr, KeyVoteWeight, ratioHash)
}
