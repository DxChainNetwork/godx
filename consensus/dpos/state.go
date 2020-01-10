// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"encoding/binary"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

type stateDB interface {
	GetState(addr common.Address, key common.Hash) common.Hash
	SetState(addr common.Address, key, value common.Hash)
	ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) error
	Exist(addr common.Address) bool
	CreateAccount(addr common.Address)
	GetNonce(common.Address) uint64
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

	// KeyPreEpochSnapshotDelegateTrieRoot is the key of block number where snapshot delegate trie
	KeyPreEpochSnapshotDelegateTrieRoot = common.BytesToHash([]byte("pre-epoch-dtr"))

	// KeyValueCommonAddress is the address for some common key-value storage
	KeyValueCommonAddress = common.BigToAddress(big.NewInt(0))

	// KeySumAllocatedReward is the key of sum allocated block reward until now
	KeySumAllocatedReward = common.BytesToHash([]byte("sum-allocated-reward"))

	// KeyValidatorAllocatedReward is the key of reward allocated to validator at last block
	KeyValidatorAllocatedReward = common.BytesToHash([]byte("validator-allocated-reward-last-block"))

	// KeyDelegatorAllocatedReward is the key of reward allocated to delegator at last block
	KeyDelegatorAllocatedReward = common.BytesToHash([]byte("delegator-allocated-reward-last-block"))
)

// GetCandidateDeposit get the candidates deposit of the addr from the state
func GetCandidateDeposit(state stateDB, addr common.Address) common.BigInt {
	depositHash := state.GetState(addr, KeyCandidateDeposit)
	return common.PtrBigInt(depositHash.Big())
}

// SetCandidateDeposit set the candidates deposit of the addr in the state
func SetCandidateDeposit(state stateDB, addr common.Address, deposit common.BigInt) {
	hash := common.BigToHash(deposit.BigIntPtr())
	state.SetState(addr, KeyCandidateDeposit, hash)
}

// GetVoteDeposit get the vote deposit of the addr from the state
func GetVoteDeposit(state stateDB, addr common.Address) common.BigInt {
	depositHash := state.GetState(addr, KeyVoteDeposit)
	return common.PtrBigInt(depositHash.Big())
}

// SetVoteDeposit set the vote deposit of the addr in the state
func SetVoteDeposit(state stateDB, addr common.Address, deposit common.BigInt) {
	hash := common.BigToHash(deposit.BigIntPtr())
	state.SetState(addr, KeyVoteDeposit, hash)
}

// GetRewardRatioNumerator get the reward ratio for a candidates for the addr in state.
// The value is used in calculating block reward for miner and his delegator
func GetRewardRatioNumerator(state stateDB, addr common.Address) uint64 {
	rewardRatioHash := state.GetState(addr, KeyRewardRatioNumerator)
	return hashToUint64(rewardRatioHash)
}

// SetRewardRatioNumerator set the CandidateRewardRatioNumerator for the addr
// in state
func SetRewardRatioNumerator(state stateDB, addr common.Address, value uint64) {
	hash := uint64ToHash(value)
	state.SetState(addr, KeyRewardRatioNumerator, hash)
}

// GetRewardRatioNumeratorLastEpoch get the rewardRatio for the validator in the last epoch
func GetRewardRatioNumeratorLastEpoch(state stateDB, addr common.Address) uint64 {
	hash := state.GetState(addr, KeyRewardRatioNumeratorLastEpoch)
	return hashToUint64(hash)
}

// SetRewardRatioNumeratorLastEpoch set the rewardRatio for the validator in the last epoch
func SetRewardRatioNumeratorLastEpoch(state stateDB, addr common.Address, value uint64) {
	hash := uint64ToHash(value)
	state.SetState(addr, KeyRewardRatioNumeratorLastEpoch, hash)
}

// GetTotalVote get the total vote for the candidates address
func GetTotalVote(state stateDB, addr common.Address) common.BigInt {
	hash := state.GetState(addr, KeyTotalVote)
	return common.PtrBigInt(hash.Big())
}

// SetTotalVote set the total vote to value for the candidates address
func SetTotalVote(state stateDB, addr common.Address, totalVotes common.BigInt) {
	hash := common.BigToHash(totalVotes.BigIntPtr())
	state.SetState(addr, KeyTotalVote, hash)
}

// GetFrozenAssets returns the frozen assets for an addr
func GetFrozenAssets(state stateDB, addr common.Address) common.BigInt {
	hash := state.GetState(addr, KeyFrozenAssets)
	return common.PtrBigInt(hash.Big())
}

// SetFrozenAssets set the frozen assets for an addr
func SetFrozenAssets(state stateDB, addr common.Address, value common.BigInt) {
	hash := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, KeyFrozenAssets, hash)
}

// AddFrozenAssets add the diff to the frozen assets of the address
func AddFrozenAssets(state stateDB, addr common.Address, diff common.BigInt) {
	prev := GetFrozenAssets(state, addr)
	newValue := prev.Add(diff)
	SetFrozenAssets(state, addr, newValue)
}

// SubFrozenAssets sub the diff from the frozen assets of the address
func SubFrozenAssets(state stateDB, addr common.Address, diff common.BigInt) error {
	prev := GetFrozenAssets(state, addr)
	if prev.Cmp(diff) < 0 {
		return errInsufficientFrozenAssets
	}
	newValue := prev.Sub(diff)
	SetFrozenAssets(state, addr, newValue)
	return nil
}

// GetBalance returns the balance of the address. This is simply an adapter function
// to convert the type from *big.Int to common.BigInt
func GetBalance(state stateDB, addr common.Address) common.BigInt {
	balance := state.GetBalance(addr)
	return common.PtrBigInt(balance)
}

// GetAvailableBalance get the available balance, which is the result of balance minus
// frozen assets.
func GetAvailableBalance(state stateDB, addr common.Address) common.BigInt {
	balance := GetBalance(state, addr)
	frozenAssets := GetFrozenAssets(state, addr)
	return balance.Sub(frozenAssets)
}

// GetThawingAssets return the thawing asset amount of the address in a certain epoch
func GetThawingAssets(state stateDB, addr common.Address, epoch int64) common.BigInt {
	key := makeThawingAssetsKey(epoch)
	hash := state.GetState(addr, key)
	return common.PtrBigInt(hash.Big())
}

// SetThawingAssets set the thawing assets in the epoch field for the addr in state
func SetThawingAssets(state stateDB, addr common.Address, epoch int64, value common.BigInt) {
	key := makeThawingAssetsKey(epoch)
	hash := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, key, hash)
}

// AddThawingAssets add the thawing assets of diff in the epoch field for the addr in state
func AddThawingAssets(state stateDB, addr common.Address, epoch int64, diff common.BigInt) {
	prev := GetThawingAssets(state, addr, epoch)
	newValue := prev.Add(diff)
	SetThawingAssets(state, addr, epoch, newValue)
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

// GetVoteLastEpoch get the vote deposit in the last epoch
func GetVoteLastEpoch(state stateDB, addr common.Address) common.BigInt {
	h := state.GetState(addr, KeyVoteLastEpoch)
	return common.PtrBigInt(h.Big())
}

// SetVoteLastEpoch set the vote epoch in the last epoch
func SetVoteLastEpoch(state stateDB, addr common.Address, value common.BigInt) {
	h := common.BigToHash(value.BigIntPtr())
	state.SetState(addr, KeyVoteLastEpoch, h)
}

// removeAddressInState remove the address from the state. Note currently only set nonce to 0.
// The balance field is not checked thus there is no guarantee that the account is removed.
// If this is the case, simply leave the address there.
func removeAddressInState(state stateDB, addr common.Address) {
	state.SetNonce(addr, 0)
}

// GetPreEpochSnapshotDelegateTrieRoot get the block number of snapshot delegate trie
func GetPreEpochSnapshotDelegateTrieRoot(state stateDB, genesis *types.Header) common.Hash {
	h := state.GetState(KeyValueCommonAddress, KeyPreEpochSnapshotDelegateTrieRoot)
	if h == types.EmptyHash {
		h = genesis.DposContext.DelegateRoot
	}
	return h
}

// setPreEpochSnapshotDelegateTrieRoot set the block number of snapshot delegate trie
func setPreEpochSnapshotDelegateTrieRoot(state stateDB, value common.Hash) {
	state.SetNonce(KeyValueCommonAddress, state.GetNonce(KeyValueCommonAddress)+1)
	state.SetState(KeyValueCommonAddress, KeyPreEpochSnapshotDelegateTrieRoot, value)
}

// setSumAllocatedReward set the sum allocated block reward until now
func setSumAllocatedReward(state stateDB, sumAllocatedReward common.Hash) {
	if !state.Exist(KeyValueCommonAddress) {
		state.CreateAccount(KeyValueCommonAddress)
	}
	state.SetNonce(KeyValueCommonAddress, state.GetNonce(KeyValueCommonAddress)+1)
	state.SetState(KeyValueCommonAddress, KeySumAllocatedReward, sumAllocatedReward)
}

// getSumAllocatedReward get the sum allocated block reward until now
func getSumAllocatedReward(state stateDB) common.BigInt {
	sumAllocatedRewardHash := state.GetState(KeyValueCommonAddress, KeySumAllocatedReward)
	return common.PtrBigInt(sumAllocatedRewardHash.Big())
}

// SetValidatorAllocatedReward set the current allocated reward for validator
func SetValidatorAllocatedReward(state stateDB, reward common.BigInt, validator common.Address) {
	v := common.BigToHash(reward.BigIntPtr())
	state.SetState(validator, KeyValidatorAllocatedReward, v)
}

// GetValidatorAllocatedReward get the current allocated reward for validator
func GetValidatorAllocatedReward(state stateDB, validator common.Address) *big.Int {
	v := state.GetState(validator, KeyValidatorAllocatedReward)
	return v.Big()
}

// SetDelegatorAllocatedReward set the current allocated reward for delegator
func SetDelegatorAllocatedReward(state stateDB, reward common.BigInt, delegator common.Address) {
	v := common.BigToHash(reward.BigIntPtr())
	state.SetState(delegator, KeyDelegatorAllocatedReward, v)
}

// GetDelegatorAllocatedReward get the current allocated reward for delegator
func GetDelegatorAllocatedReward(state stateDB, delegator common.Address) *big.Int {
	v := state.GetState(delegator, KeyDelegatorAllocatedReward)
	return v.Big()
}
