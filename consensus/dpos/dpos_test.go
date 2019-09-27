// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
	"github.com/stretchr/testify/assert"
)

func TestUpdateMinedCnt(t *testing.T) {
	var (
		delegator = common.HexToAddress("0xaaa")
	)

	db := ethdb.NewMemDatabase()
	dposContext, _, err := mockDposContext(db, time.Now().Unix(), delegator)
	if err != nil {
		t.Fatalf("failed to mock dpos context,error: %v", err)
	}

	// new block still in the same epoch with current block, but newMiner is the first time to mined in the epoch
	lastTime := int64(EpochInterval)

	miner := common.HexToAddress("0xab")
	blockTime := int64(EpochInterval + BlockInterval)

	beforeUpdateCnt, err := getMinedCnt(dposContext.MinedCntTrie(), blockTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	err = updateMinedCnt(lastTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt, err := getMinedCnt(dposContext.MinedCntTrie(), blockTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(0), beforeUpdateCnt)
	assert.Equal(t, uint64(1), afterUpdateCnt)

	// new block still in the same epoch with current block, and newMiner has mined block before in the epoch
	err = setMinedCntTrie(blockTime/EpochInterval, miner, dposContext.MinedCntTrie(), int64(1))
	if err != nil {
		t.Fatalf("failed to set mined count trie,error: %v", err)
	}

	blockTime = EpochInterval + BlockInterval*4

	// currentBlock has recorded the count for the newMiner before updateMinedCnt
	beforeUpdateCnt, err = getMinedCnt(dposContext.MinedCntTrie(), blockTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	err = updateMinedCnt(lastTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt, err = getMinedCnt(dposContext.MinedCntTrie(), blockTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(1), beforeUpdateCnt)
	assert.Equal(t, uint64(2), afterUpdateCnt)

	// new block come to a new epoch
	blockTime = EpochInterval * 2

	beforeUpdateCnt, err = getMinedCnt(dposContext.MinedCntTrie(), lastTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	err = updateMinedCnt(lastTime, miner, dposContext)
	assert.Nil(t, err)

	afterUpdateCnt, err = getMinedCnt(dposContext.MinedCntTrie(), lastTime/EpochInterval, miner)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(2), beforeUpdateCnt)
	assert.Equal(t, uint64(3), afterUpdateCnt)
}

func TestAccumulateRewards(t *testing.T) {
	delegator := common.HexToAddress("0xaaa")
	db := ethdb.NewMemDatabase()
	dposCtx, candidates, err := mockDposContext(db, time.Now().Unix(), delegator)
	if err != nil {
		t.Fatalf("failed to mock dpos context,error: %v", err)
	}

	_, err = dposCtx.Commit()
	assert.Nil(t, err)

	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))

	// set vote deposit and weight ratio for delegator
	validator := candidates[1]
	SetVoteLastEpoch(stateDB, delegator, common.PtrBigInt(big.NewInt(100000)))

	// set validator reward ratio
	var rewardRatioNumerator uint64 = 50
	SetRewardRatioNumeratorLastEpoch(stateDB, validator, rewardRatioNumerator)

	// set the total vote weight for validator
	SetTotalVote(stateDB, validator, common.PtrBigInt(big.NewInt(100000)))

	stateDbCopy := stateDB.Copy()

	dposEng := &Dpos{
		db: db,
	}

	testChain := testChainReader{
		headers: make(map[uint64]*testHeader, 0),
	}

	for i := uint64(0); i < ConsensusSize; i++ {
		hash := common.BigToHash(new(big.Int).SetUint64(i))
		if i == 0 {
			testChain.insertGenesis(hash, uint64(10*i+1000), dposCtx)
			continue
		}
		testChain.insert(hash, i, uint64(10*i+1000), dposEng, dposCtx)
	}

	// Byzantium
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1 << 10), Coinbase: validator, Validator: validator}
	expectedDelegatorReward := big.NewInt(1.5e+18)
	expectedValidatorReward := big.NewInt(1.5e+18)

	// allocate the block reward among validator and its delegators
	accumulateRewards(params.MainnetChainConfig, stateDB, header, trie.NewDatabase(db), testChain.GetHeaderByNumber(0))
	header.Root = stateDB.IntermediateRoot(params.MainnetChainConfig.IsEIP158(header.Number))

	validatorBalance := stateDB.GetBalance(validator)
	if validatorBalance.Cmp(expectedValidatorReward) != 0 {
		t.Errorf("validator reward not equal to the value assigned to address, want: %v, got: %v", expectedValidatorReward.String(), validatorBalance.String())
	}

	delegatorBalance := stateDB.GetBalance(delegator)
	if delegatorBalance.Cmp(expectedDelegatorReward) != 0 {
		t.Errorf("delegator reward not equal to the value assigned to address, want: %v, got: %v", expectedValidatorReward.String(), validatorBalance.String())
	}

	// mock block sync
	headerCopy := header
	accumulateRewards(params.MainnetChainConfig, stateDbCopy, headerCopy, trie.NewDatabase(db), testChain.GetHeaderByNumber(0))
	headerCopy.Root = stateDB.IntermediateRoot(params.MainnetChainConfig.IsEIP158(headerCopy.Number))

	if header.Root != headerCopy.Root {
		t.Errorf("block sync state root not equal, one: %s, another: %s", header.Root.String(), headerCopy.Root.String())
	}
}

func TestDpos_CheckValidator(t *testing.T) {
	var (
		delegator = common.HexToAddress("0xaaa")

		tests = []struct {
			name    string
			fn      func(db ethdb.Database, dposRoot *types.DposContextRoot, validator common.Address) error
			wantErr error
		}{
			{
				name: "mined the future block",
				fn: func(db ethdb.Database, dposRoot *types.DposContextRoot, validator common.Address) error {
					lastBlockTime := int64(86430)
					lastBlockHeader := &types.Header{
						Time:        new(big.Int).SetInt64(lastBlockTime),
						DposContext: dposRoot,
					}

					lastBlock := types.NewBlockWithHeader(lastBlockHeader)
					dposEng := New(nil, db)
					dposEng.signer = validator
					return dposEng.CheckValidator(lastBlock, int64(86420))
				},
				wantErr: ErrMinedFutureBlock,
			},
			{
				name: "wait for last block arrived",
				fn: func(db ethdb.Database, dposRoot *types.DposContextRoot, validator common.Address) error {
					lastBlockTime := int64(86410)
					lastBlockHeader := &types.Header{
						Time:        new(big.Int).SetInt64(lastBlockTime),
						DposContext: dposRoot,
					}

					lastBlock := types.NewBlockWithHeader(lastBlockHeader)
					dposEng := New(nil, db)
					dposEng.signer = validator
					return dposEng.CheckValidator(lastBlock, int64(86443))
				},
				wantErr: ErrWaitForPrevBlock,
			},
			{
				name: "invalid block validator",
				fn: func(db ethdb.Database, dposRoot *types.DposContextRoot, validator common.Address) error {
					lastBlockTime := int64(86410)
					lastBlockHeader := &types.Header{
						Time:        new(big.Int).SetInt64(lastBlockTime),
						DposContext: dposRoot,
					}

					lastBlock := types.NewBlockWithHeader(lastBlockHeader)
					dposEng := New(nil, db)
					dposEng.signer = common.HexToAddress("0x234")
					return dposEng.CheckValidator(lastBlock, int64(86440))
				},
				wantErr: ErrInvalidBlockValidator,
			},
			{
				name: "success to check validator",
				fn: func(db ethdb.Database, dposRoot *types.DposContextRoot, validator common.Address) error {
					lastBlockTime := int64(86400)
					lastBlockHeader := &types.Header{
						Time:        new(big.Int).SetInt64(lastBlockTime),
						DposContext: dposRoot,
					}

					lastBlock := types.NewBlockWithHeader(lastBlockHeader)
					dposEng := New(nil, db)
					dposEng.signer = validator
					return dposEng.CheckValidator(lastBlock, int64(86410))
				},
				wantErr: nil,
			},
		}
	)

	db := ethdb.NewMemDatabase()
	dposContext, candidates, err := mockDposContext(db, int64(86400), delegator)
	if err != nil {
		t.Fatalf("failed to mock dpos context,error: %v", err)
	}

	dposRoot, err := dposContext.Commit()
	if err != nil {
		t.Fatalf("failed to commit dpos context,error: %v", err)
	}

	// set the new block is candidates[1]'s turn to produce block
	for _, test := range tests {
		err := test.fn(db, dposRoot, candidates[1])
		if err != test.wantErr {
			t.Errorf("wanted %v, got %v", test.wantErr, err)
		}
	}
}

func TestUpdateConfirmedBlockHeader(t *testing.T) {
	var (
		tests = []struct {
			name                  string
			fn                    func() (uint64, error)
			wantConfirmedBlockNum uint64
		}{
			{
				name: "the number of current block chain less than ConsensusSize",
				fn: func() (uint64, error) {
					chainDB := ethdb.NewMemDatabase()
					dposEng := &Dpos{
						db: chainDB,
					}

					testChain := testChainReader{
						headers: make(map[uint64]*testHeader, 0),
					}

					for i := uint64(0); i < ConsensusSize; i++ {
						hash := common.BigToHash(new(big.Int).SetUint64(i))
						if i == 0 {
							testChain.insertGenesis(hash, uint64(10*i+1000), nil)
							continue
						}
						err := testChain.insert(hash, i, uint64(10*i+1000), dposEng, nil)
						if err != nil {
							return 0, err
						}
					}

					return dposEng.confirmedBlockHeader.Number.Uint64(), nil
				},
				wantConfirmedBlockNum: 0,
			},
			{
				name: "the number of current block chain more than ConsensusSize",
				fn: func() (uint64, error) {
					chainDB := ethdb.NewMemDatabase()
					dposEng := &Dpos{
						db: chainDB,
					}

					testChain := testChainReader{
						headers: make(map[uint64]*testHeader, 0),
					}

					for i := uint64(0); i < MaxValidatorSize+5; i++ {
						hash := common.BigToHash(new(big.Int).SetUint64(i))
						if i == 0 {
							testChain.insertGenesis(hash, uint64(10*i+1000), nil)
							continue
						}
						err := testChain.insert(hash, i, uint64(10*i+1000), dposEng, nil)
						if err != nil {
							return 0, err
						}
					}

					return dposEng.confirmedBlockHeader.Number.Uint64(), nil
				},
				wantConfirmedBlockNum: (MaxValidatorSize + 5) - ConsensusSize,
			},
		}
	)

	for _, test := range tests {
		num, err := test.fn()
		if err != nil {
			t.Fatalf("%s : %v", test.name, err)
		}

		if num != test.wantConfirmedBlockNum {
			t.Errorf("%s want number: %d,got: %d", test.name, test.wantConfirmedBlockNum, num)
		}
	}

}

type testHeader struct {
	hash        common.Hash
	number      uint64
	parent      *testHeader
	time        uint64
	validator   common.Address
	DposContext *types.DposContextRoot
}

type testChainReader struct {
	headers     map[uint64]*testHeader
	currentHash common.Hash
	currentNum  uint64
}

func (test testChainReader) Config() *params.ChainConfig {
	return nil
}

func (test testChainReader) CurrentHeader() *types.Header {
	if len(test.headers) == 0 {
		return nil
	}

	var parentHash common.Hash
	var timeStamp uint64
	var validator common.Address
	if test.currentNum == 0 {
		parentHash = common.Hash{}
	} else {
		parentHash = test.headers[test.currentNum].parent.hash
		timeStamp = test.headers[test.currentNum].time
		validator = test.headers[test.currentNum].validator
	}

	return &types.Header{
		ParentHash: parentHash,
		Number:     new(big.Int).SetUint64(test.currentNum),
		Time:       new(big.Int).SetUint64(timeStamp),
		Validator:  validator,
	}
}

func (test testChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}

func (test testChainReader) GetHeaderByNumber(number uint64) *types.Header {
	if len(test.headers) == 0 {
		return nil
	}

	var parentHash common.Hash
	var timeStamp uint64
	var validator common.Address
	if number == 0 {
		parentHash = common.Hash{}
	} else {
		parentHash = test.headers[number].parent.hash
		timeStamp = test.headers[number].time
		validator = test.headers[test.currentNum].validator
	}

	return &types.Header{
		ParentHash:  parentHash,
		Number:      new(big.Int).SetUint64(number),
		Time:        new(big.Int).SetUint64(timeStamp),
		Validator:   validator,
		DposContext: test.headers[number].DposContext,
	}
}

func (test testChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	if len(test.headers) == 0 {
		return nil
	}

	parentHash := common.Hash{}
	var number uint64
	var timeStamp uint64
	var validator common.Address
	for _, header := range test.headers {
		if header.hash == hash {
			if header.number != 0 {
				parentHash = header.parent.hash
			}
			number = header.number
			timeStamp = header.time
			validator = header.validator
			break
		}
	}

	return &types.Header{
		ParentHash: parentHash,
		Number:     new(big.Int).SetUint64(number),
		Time:       new(big.Int).SetUint64(timeStamp),
		Validator:  validator,
	}
}

func (test testChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}

func (test testChainReader) insertGenesis(hash common.Hash, time uint64, dposCtx *types.DposContext) {
	var dposCtxRoot *types.DposContextRoot
	if dposCtx != nil {
		dposCtxRoot = dposCtx.ToRoot()
	}
	header := &testHeader{
		hash:        hash,
		number:      0,
		parent:      nil,
		time:        time,
		validator:   common.BigToAddress(new(big.Int).SetUint64(0)),
		DposContext: dposCtxRoot,
	}
	test.headers[0] = header
	test.currentHash = hash
	test.currentNum = 0
}

func (test testChainReader) insert(hash common.Hash, number uint64, time uint64, dposEng *Dpos, dposCtx *types.DposContext) error {
	parent := test.headers[number-1]
	var dposCtxRoot *types.DposContextRoot
	if dposCtx != nil {
		dposCtxRoot = dposCtx.ToRoot()
	}
	header := &testHeader{
		hash:        hash,
		number:      number,
		parent:      parent,
		time:        time,
		validator:   common.BigToAddress(new(big.Int).SetUint64(number)),
		DposContext: dposCtxRoot,
	}

	test.headers[number] = header
	test.currentHash = hash
	test.currentNum = number
	err := dposEng.updateConfirmedBlockHeader(test)
	if err != nil {
		return err
	}

	return nil
}

func mockDposContext(db ethdb.Database, now int64, delegator common.Address) (*types.DposContext, []common.Address, error) {
	dposContext, err := types.NewDposContextFromProto(db, &types.DposContextRoot{})
	if err != nil {
		return nil, nil, err
	}

	// mock MaxValidatorSize+5 candidates, and the prev MaxValidatorSize candidates as validator
	var candidates []common.Address
	for i := 0; i < MaxValidatorSize+5; i++ {
		str := fmt.Sprintf("%d", i+1)
		addr := common.HexToAddress("0x" + str)
		candidates = append(candidates, addr)
	}

	// update candidates trie and delegate trie
	for _, can := range candidates {
		err = dposContext.CandidateTrie().TryUpdate(can.Bytes(), can.Bytes())
		if err != nil {
			return nil, nil, err
		}

		err = dposContext.DelegateTrie().TryUpdate(append(can.Bytes(), delegator.Bytes()...), delegator.Bytes())
		if err != nil {
			return nil, nil, err
		}
	}

	// update epoch trie, set the prev MaxValidatorSize candidates as validators
	err = dposContext.SetValidators(candidates[:MaxValidatorSize])
	if err != nil {
		return nil, nil, err
	}

	// update mined count trie
	cnt := int64(0)
	epochID := CalculateEpochID(now)
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(epochID))
	for i := 0; i < MaxValidatorSize; i++ {

		// the prev 1/3 validators will be set not qualified
		if i < MaxValidatorSize/3 {
			cnt = EpochInterval/BlockInterval/MaxValidatorSize/2 - int64(i+1)
		} else {
			cnt = EpochInterval/BlockInterval/MaxValidatorSize/2 + int64(i+1)
		}

		cntBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(cntBytes, uint64(cnt))
		err = dposContext.MinedCntTrie().TryUpdate(append(epochBytes, candidates[i].Bytes()...), cntBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	// update vote trie
	canListBytes, err := rlp.EncodeToBytes(candidates)
	if err != nil {
		return nil, nil, err
	}

	err = dposContext.VoteTrie().TryUpdate(delegator.Bytes(), canListBytes)
	if err != nil {
		return nil, nil, err
	}

	return dposContext, candidates, nil
}

func setMinedCntTrie(epochID int64, candidate common.Address, minedCntTrie *trie.Trie, count int64) error {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(cntBytes, uint64(count))
	err := minedCntTrie.TryUpdate(append(key, candidate.Bytes()...), cntBytes)
	if err != nil {
		return err
	}
	return nil
}
