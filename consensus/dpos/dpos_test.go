// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/trie"
	"github.com/stretchr/testify/assert"
)

var (
	MockEpochValidators = []string{
		"0x1",
		"0x2",
		"0x3",
		"0x4",
		"0x5",
		"0x6",
		"0x7",
		"0x8",
		"0x9",
		"0xa",
		"0xb",
		"0xc",
		"0xd",
		"0xe",
		"0xf",
		"0x10",
		"0x11",
		"0x12",
		"0x13",
		"0x14",
		"0x15",
	}
)

func mockNewDposContext(db ethdb.Database) (*types.DposContext, error) {
	dposContext, err := types.NewDposContextFromProto(db, &types.DposContextProto{})
	if err != nil {
		return nil, err
	}

	var (
		delegator []byte
		candidate []byte
		addresses []common.Address
	)

	for i := 0; i < MaxValidatorSize; i++ {
		addresses = append(addresses, common.HexToAddress(MockEpochValidators[i]))
	}

	err = dposContext.SetValidators(addresses)
	if err != nil {
		return nil, err
	}

	for j := 0; j < len(MockEpochValidators); j++ {
		delegator = common.HexToAddress(MockEpochValidators[(j+1)%len(MockEpochValidators)]).Bytes()

		candidate = common.HexToAddress(MockEpochValidators[j]).Bytes()
		err = dposContext.DelegateTrie().TryUpdate(append(candidate, delegator...), delegator)
		if err != nil {
			return nil, err
		}

		err = dposContext.VoteTrie().TryUpdate(delegator, candidate)
		if err != nil {
			return nil, err
		}

		delegator = common.HexToAddress(MockEpochValidators[(j+2)%len(MockEpochValidators)]).Bytes()
		err = dposContext.DelegateTrie().TryUpdate(append(candidate, delegator...), delegator)
		if err != nil {
			return nil, err
		}

		err = dposContext.VoteTrie().TryUpdate(delegator, candidate)
		if err != nil {
			return nil, err
		}

		err = dposContext.CandidateTrie().TryUpdate(candidate, candidate)
		if err != nil {
			return nil, err
		}
	}

	return dposContext, nil
}

func setMintCntTrie(epochID int64, candidate common.Address, mintCntTrie *trie.Trie, count int64) error {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(cntBytes, uint64(count))
	err := mintCntTrie.TryUpdate(append(key, candidate.Bytes()...), cntBytes)
	if err != nil {
		return err
	}
	return nil
}

func getMintCnt(epochID int64, candidate common.Address, mintCntTrie *trie.Trie) int64 {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := mintCntTrie.Get(append(key, candidate.Bytes()...))
	if cntBytes == nil {
		return 0
	} else {
		return int64(binary.BigEndian.Uint64(cntBytes))
	}
}

func TestUpdateMintCnt(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposContext, err := mockNewDposContext(db)
	if err != nil {
		t.Fatalf("failed to mock dpos context,error: %v", err)
	}

	// new block still in the same epoch with current block, but newMiner is the first time to mint in the epoch
	lastTime := int64(EpochInterval)

	miner := common.HexToAddress("0xab")
	blockTime := int64(EpochInterval + BlockInterval)

	beforeUpdateCnt := getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	err = updateMintCnt(lastTime, blockTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt := getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)

	// new block still in the same epoch with current block, and newMiner has mint block before in the epoch
	err = setMintCntTrie(blockTime/EpochInterval, miner, dposContext.MintCntTrie(), int64(1))
	if err != nil {
		t.Fatalf("failed to set mint count trie,error: %v", err)
	}

	blockTime = EpochInterval + BlockInterval*4

	// currentBlock has recorded the count for the newMiner before UpdateMintCnt
	beforeUpdateCnt = getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	err = updateMintCnt(lastTime, blockTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt = getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(1), beforeUpdateCnt)
	assert.Equal(t, int64(2), afterUpdateCnt)

	// new block come to a new epoch
	blockTime = EpochInterval * 2

	beforeUpdateCnt = getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	err = updateMintCnt(lastTime, blockTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt = getMintCnt(blockTime/EpochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)
}

func TestAccumulateRewards(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposContext, err := mockNewDposContext(db)
	if err != nil {
		t.Fatalf("failed to mock dpos context,error: %v", err)
	}

	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))

	// set validator reward ratio
	validator := common.HexToAddress(MockEpochValidators[1])
	delegators := []common.Address{common.HexToAddress(MockEpochValidators[2]), common.HexToAddress(MockEpochValidators[3])}
	for i := 0; i < len(delegators); i++ {
		stateDB.SetState(delegators[i], KeyVoteDeposit, common.BigToHash(big.NewInt(113)))
	}

	var rewardRatioNumerator uint8 = 50
	stateDB.SetState(validator, KeyRewardRatioNumerator, common.BytesToHash([]byte{rewardRatioNumerator}))
	stateDbCopy := stateDB.Copy()

	// Byzantium
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1 << 10), Coinbase: validator, Validator: validator}
	expectedDelegatorReward := []*big.Int{big.NewInt(12712817820445511), big.NewInt(12712817820445511)}
	expectedValidatorReward := big.NewInt(2974574364359108978)

	// allocate the block reward among validator and its delegators
	accumulateRewards(params.MainnetChainConfig, stateDB, header, dposContext)
	header.Root = stateDB.IntermediateRoot(params.MainnetChainConfig.IsEIP158(header.Number))

	validatorBalance := stateDB.GetBalance(validator)
	if validatorBalance.Cmp(expectedValidatorReward) != 0 {
		t.Fatalf("validator reward not equal to the value assigned to address, want: %v, got: %v", expectedValidatorReward.String(), validatorBalance.String())
	}

	for i := 0; i < len(expectedDelegatorReward); i++ {
		delegatorBalance := stateDB.GetBalance(delegators[i])
		if delegatorBalance.Cmp(expectedDelegatorReward[i]) != 0 {
			t.Fatalf("delegator reward not equal to the value assigned to address, want: %v, got: %v", expectedValidatorReward.String(), validatorBalance.String())
		}
	}

	// mock block sync
	headerCopy := header
	accumulateRewards(params.MainnetChainConfig, stateDbCopy, headerCopy, dposContext)
	headerCopy.Root = stateDB.IntermediateRoot(params.MainnetChainConfig.IsEIP158(headerCopy.Number))

	if header.Root.String() != headerCopy.Root.String() {
		t.Fatalf("block sync state root not equal, one: %s, another: %s", header.Root.String(), headerCopy.Root.String())
	}
}
