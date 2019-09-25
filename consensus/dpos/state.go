// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"encoding/binary"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
)

type stateDB interface {
	GetState(addr common.Address, key common.Hash) common.Hash
	SetState(addr common.Address, key, value common.Hash)
	ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) error
	Exist(addr common.Address) bool
	CreateAccount(addr common.Address)
	SetNonce(addr common.Address, nonce uint64)
	GetBalance(addr common.Address) *big.Int
}

var (
	// KeyVoteDeposit is the key of vote deposit
	KeyVoteDeposit = common.BytesToHash([]byte("vote-deposit"))

	// KeyCandidateDeposit is the key of candidates deposit
	KeyCandidateDeposit = common.BytesToHash([]byte("candidates-deposit"))

	// KeyRewardRatioNumerator is the key of block reward ration numerator indicates the percent of share validator with its delegators
	KeyRewardRatioNumerator = common.BytesToHash([]byte("reward-ratio-numerator"))

	// KeyRewardRatioNumeratorLastEpoch is the key for storing the reward ratio in
	// the last epoch.
	KeyRewardRatioNumeratorLastEpoch = common.BytesToHash([]byte("reward-ratio-last-epoch"))

	// KeyVoteLastEpoch is the vote deposit in the last epoch
	KeyVoteLastEpoch = common.BytesToHash([]byte("vote-last-epoch"))

	// KeyTotalVote is the key of total vote for each candidates
	KeyTotalVote = common.BytesToHash([]byte("total-vote"))

	// KeyFrozenAssets is the key for frozen assets for in an account
	KeyFrozenAssets = common.BytesToHash([]byte("frozen-assets"))

	// PrefixThawingAssets is the prefix recording the amount to be thawed in a specified epoch
	PrefixThawingAssets = []byte("thawing-assets")

	// KeyPreEpochSnapshotDelegateTrieBlockNumber is the key of block number where snapshot delegate trie
	KeyPreEpochSnapshotDelegateTrieBlockNumber = common.BytesToHash([]byte("pre-epoch-bn"))

	// KeyValueCommonAddress is the address for some common key-value storage
	KeyValueCommonAddress = common.BigToAddress(big.NewInt(0))
)

// getCandidateDeposit get the candidates deposit of the addr from the state
func getCandidateDeposit(state stateDB, addr common.Address) common.BigInt {
	depositHash := state.GetState(addr, KeyCandidateDeposit)
	return common.PtrBigInt(depositHash.Big())
}

// setCandidateDeposit set the candidates deposit of the addr in the state
func setCandidateDeposit(state stateDB, addr common.Address, deposit common.BigInt) {
	hash := common.BigToHash(deposit.BigIntPtr())
	state.SetState(addr, KeyCandidateDeposit, hash)
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

// getRewardRatioNumerator get the reward ratio for a candidates for the addr in state.
// The value is used in calculating block reward for miner and his delegator
func getRewardRatioNumerator(state stateDB, addr common.Address) uint64 {
	rewardRatioHash := state.GetState(addr, KeyRewardRatioNumerator)
	return hashToUint64(rewardRatioHash)
}

// setRewardRatioNumerator set the CandidateRewardRatioNumerator for the addr
// in state
func setRewardRatioNumerator(state stateDB, addr common.Address, value uint64) {
	hash := uint64ToHash(value)
	state.SetState(addr, KeyRewardRatioNumerator, hash)
}

// getRewardRatioNumeratorLastEpoch get the rewardRatio for the validator in the last epoch
func getRewardRatioNumeratorLastEpoch(state stateDB, addr common.Address) uint64 {
	hash := state.GetState(addr, KeyRewardRatioNumeratorLastEpoch)
	return hashToUint64(hash)
}

// setRewardRatioNumeratorLastEpoch set the rewardRatio for the validator in the last epoch
func setRewardRatioNumeratorLastEpoch(state stateDB, addr common.Address, value uint64) {
	hash := uint64ToHash(value)
	state.SetState(addr, KeyRewardRatioNumeratorLastEpoch, hash)
}

// getTotalVote get the total vote for the candidates address
func getTotalVote(state stateDB, addr common.Address) common.BigInt {
	hash := state.GetState(addr, KeyTotalVote)
	return common.PtrBigInt(hash.Big())
}

// setTotalVote set the total vote to value for the candidates address
func setTotalVote(state stateDB, addr common.Address, totalVotes common.BigInt) {
	hash := common.BigToHash(totalVotes.BigIntPtr())
	state.SetState(addr, KeyTotalVote, hash)
}

// GetFrozenAssets returns the frozen assets for an addr
func GetFrozenAssets(state stateDB, addr common.Address) common.BigInt {
	hash := state.GetState(addr, KeyFrozenAssets)
	return common.PtrBigInt(hash.Big())
}

// setFrozenAssets set the frozen assets for an addr
func setFrozenAssets(state stateDB, addr common.Address, value common.BigInt) {
	hash := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, KeyFrozenAssets, hash)
}

// addFrozenAssets add the diff to the frozen assets of the address
func addFrozenAssets(state stateDB, addr common.Address, diff common.BigInt) {
	prev := GetFrozenAssets(state, addr)
	newValue := prev.Add(diff)
	setFrozenAssets(state, addr, newValue)
}

// subFrozenAssets sub the diff from the frozen assets of the address
func subFrozenAssets(state stateDB, addr common.Address, diff common.BigInt) error {
	prev := GetFrozenAssets(state, addr)
	if prev.Cmp(diff) < 0 {
		return errInsufficientFrozenAssets
	}
	newValue := prev.Sub(diff)
	setFrozenAssets(state, addr, newValue)
	return nil
}

// getBalance returns the balance of the address. This is simply an adapter function
// to convert the type from *big.Int to common.BigInt
func getBalance(state stateDB, addr common.Address) common.BigInt {
	balance := state.GetBalance(addr)
	return common.PtrBigInt(balance)
}

// getAvailableBalance get the available balance, which is the result of balance minus
// frozen assets.
func getAvailableBalance(state stateDB, addr common.Address) common.BigInt {
	balance := getBalance(state, addr)
	frozenAssets := GetFrozenAssets(state, addr)
	return balance.Sub(frozenAssets)
}

// getThawingAssets return the thawing asset amount of the address in a certain epoch
func getThawingAssets(state stateDB, addr common.Address, epoch int64) common.BigInt {
	key := makeThawingAssetsKey(epoch)
	hash := state.GetState(addr, key)
	return common.PtrBigInt(hash.Big())
}

// setThawingAssets set the thawing assets in the epoch field for the addr in state
func setThawingAssets(state stateDB, addr common.Address, epoch int64, value common.BigInt) {
	key := makeThawingAssetsKey(epoch)
	hash := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, key, hash)
}

// addThawingAssets add the thawing assets of diff in the epoch field for the addr in state
func addThawingAssets(state stateDB, addr common.Address, epoch int64, diff common.BigInt) {
	prev := getThawingAssets(state, addr, epoch)
	newValue := prev.Add(diff)
	setThawingAssets(state, addr, epoch, newValue)
}

// removeThawingAssets remove the thawing assets in a certain epoch for the address
func removeThawingAssets(state stateDB, addr common.Address, epoch int64) {
	key := makeThawingAssetsKey(epoch)
	state.SetState(addr, key, common.Hash{})
}

// makeThawingAssetsKey makes the key for the thawing assets in a certain epoch
func makeThawingAssetsKey(epoch int64) common.Hash {
	epochByte := make([]byte, 8)
	binary.BigEndian.PutUint64(epochByte, uint64(epoch))
	return common.BytesToHash(append(PrefixThawingAssets, epochByte...))
}

// getVoteLastEpoch get the vote deposit in the last epoch
func getVoteLastEpoch(state stateDB, addr common.Address) common.BigInt {
	h := state.GetState(addr, KeyVoteLastEpoch)
	return common.PtrBigInt(h.Big())
}

// setVoteLastEpoch set the vote epoch in the last epoch
func setVoteLastEpoch(state stateDB, addr common.Address, value common.BigInt) {
	h := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, KeyVoteLastEpoch, h)
}

// removeAddressInState remove the address from the state. Note currently only set nonce to 0.
// The balance field is not checked thus there is no guarantee that the account is removed.
// If this is the case, simply leave the address there.
func removeAddressInState(state stateDB, addr common.Address) {
	state.SetNonce(addr, 0)
}

// getPreEpochSnapshotDelegateTrieBlockNumber get the block number of snapshot delegate trie
func getPreEpochSnapshotDelegateTrieBlockNumber(state stateDB) common.BigInt {
	h := state.GetState(KeyValueCommonAddress, KeyPreEpochSnapshotDelegateTrieBlockNumber)
	if h == (common.Hash{}) {
		return common.BigInt0
	}
	return common.PtrBigInt(h.Big())
}

// setPreEpochSnapshotDelegateTrieBlockNumber set the block number of snapshot delegate trie
func setPreEpochSnapshotDelegateTrieBlockNumber(state stateDB, value common.BigInt) {
	h := common.BigToHash(value.BigIntPtr())
	state.SetState(KeyValueCommonAddress, KeyPreEpochSnapshotDelegateTrieBlockNumber, h)
}
