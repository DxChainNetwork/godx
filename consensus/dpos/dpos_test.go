// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
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

func mockNewDposContext(db ethdb.Database) *types.DposContext {
	dposContext, err := types.NewDposContextFromProto(db, &types.DposContextProto{})
	if err != nil {
		return nil
	}

	var (
		delegator = []byte{}
		candidate = []byte{}
		addresses = []common.Address{}
	)

	for i := 0; i < maxValidatorSize; i++ {
		addresses = append(addresses, common.HexToAddress(MockEpochValidators[i]))
	}

	dposContext.SetValidators(addresses)

	for j := 0; j < len(MockEpochValidators); j++ {
		delegator = common.HexToAddress(MockEpochValidators[j]).Bytes()
		candidate = common.HexToAddress(MockEpochValidators[j]).Bytes()
		dposContext.DelegateTrie().TryUpdate(append(candidate, delegator...), candidate)
		dposContext.CandidateTrie().TryUpdate(candidate, candidate)
		dposContext.VoteTrie().TryUpdate(candidate, candidate)
	}

	return dposContext
}

func setMintCntTrie(epochID int64, candidate common.Address, mintCntTrie *trie.Trie, count int64) {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(cntBytes, uint64(count))
	mintCntTrie.TryUpdate(append(key, candidate.Bytes()...), cntBytes)
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
	dposContext := mockNewDposContext(db)

	// new block still in the same epoch with current block, but newMiner is the first time to mint in the epoch
	lastTime := int64(epochInterval)

	miner := common.HexToAddress("0xab")
	blockTime := int64(epochInterval + blockInterval)

	beforeUpdateCnt := getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt := getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)

	// new block still in the same epoch with current block, and newMiner has mint block before in the epoch
	setMintCntTrie(blockTime/epochInterval, miner, dposContext.MintCntTrie(), int64(1))

	blockTime = epochInterval + blockInterval*4

	// currentBlock has recorded the count for the newMiner before UpdateMintCnt
	beforeUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(1), beforeUpdateCnt)
	assert.Equal(t, int64(2), afterUpdateCnt)

	// new block come to a new epoch
	blockTime = epochInterval * 2

	beforeUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)
}
