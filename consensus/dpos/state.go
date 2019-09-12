// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"encoding/binary"

	"github.com/DxChainNetwork/godx/common"
)

type stateDB interface {
	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)
	ForEachStorage(common.Address, func(common.Hash, common.Hash) bool)
}

var (
	// KeyRewardRatioNumerator is the key of block reward ration numerator indicates the percent of share validator with its delegators
	KeyRewardRatioNumerator = common.BytesToHash([]byte("reward-ratio-numerator"))

	// KeyVoteDeposit is the key of vote deposit
	KeyVoteDeposit = common.BytesToHash([]byte("vote-deposit"))

	// KeyRealVoteWeightRatio is the weight ratio of vote
	KeyRealVoteWeightRatio = common.BytesToHash([]byte("real-vote-weight-ratio"))

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

// hashToUint64 convert the hash to uint64. Only the last 8 bytes in the hash is interpreted as
// uint64
func hashToUint64(hash common.Hash) uint64 {
	return binary.LittleEndian.Uint64(hash[common.HashLength-8:])
}

// uint64ToHash convert the uint64 value to the hash. The value is written in the last 8 bytes
// in the hash
func uint64ToHash(value uint64) common.Hash {
	var h common.Hash
	binary.LittleEndian.PutUint64(h[common.HashLength-8:], value)
	return h
}
