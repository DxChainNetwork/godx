// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

const (
	blocksPerEpoch = EpochInterval / BlockInterval
	timeFirstBlock = 1000000
	firstElect     = (EpochInterval - (timeFirstBlock-BlockInterval)%EpochInterval) / BlockInterval
)

func TestApiHelper_lastElectBlockHeader(t *testing.T) {
	testData := []struct {
		blockNum uint64
		expect   uint64
	}{
		{0, 0},
		{1, 0},
		{100, 0},
		{uint64(firstElect+blocksPerEpoch) - 100, uint64(firstElect)},
		{uint64(firstElect+blocksPerEpoch) - 1, uint64(firstElect)},
		{uint64(firstElect + blocksPerEpoch), uint64(firstElect)},
		{uint64(firstElect + blocksPerEpoch*3/2), uint64(firstElect + blocksPerEpoch)},
		{uint64(firstElect + blocksPerEpoch*5), uint64(firstElect + blocksPerEpoch*4)},
		{uint64(firstElect + blocksPerEpoch*11/2), uint64(firstElect + blocksPerEpoch*5)},
	}
	bc, err := NewFakeBlockChain(int(blocksPerEpoch*6+firstElect), nil)
	if err != nil {
		t.Fatal(err)
	}
	helper := ApiHelper{bc}
	for _, test := range testData {
		header := helper.bc.GetHeaderByNumber(test.blockNum)
		if header == nil {
			t.Fatalf("unknown header %v", test.blockNum)
		}
		got, err := helper.lastElectBlockHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Errorf("input %v, got nil", test.blockNum)
		}
		if got.Number.Uint64() != test.expect {
			t.Errorf("input %v, got %v, expect %v", test.blockNum, got.Number.Uint64(), test.expect)
		}
	}
}

type fakeBlockChain struct {
	blocks   []*types.Header
	states   []*state.StateDB
	dposCtxs []*types.DposContext
	db       ethdb.Database
}

func NewFakeBlockChain(num int, f func(i int, header *types.Header, statedb *state.StateDB, dposCtx *types.DposContext)) (*fakeBlockChain, error) {
	db := ethdb.NewMemDatabase()
	bc := &fakeBlockChain{
		blocks:   make([]*types.Header, 0, num),
		states:   make([]*state.StateDB, 0, num),
		dposCtxs: make([]*types.DposContext, 0, num),
		db:       db,
	}
	var parentHash common.Hash
	curTime := int64(0)
	statedb, err := state.New(common.Hash{}, state.NewDatabase(db))
	if err != nil {
		return nil, err
	}
	dposCtx, err := types.NewDposContext(db)
	if err != nil {
		return nil, err
	}
	for i := 0; i != num; i++ {
		curHash := makeBlockHash(i)
		h := &types.Header{
			ParentHash:  parentHash,
			Root:        curHash,
			DposContext: &types.DposContextRoot{EpochRoot: curHash},
			Number:      big.NewInt(int64(i)),
			Time:        big.NewInt(curTime),
		}
		if f != nil {
			f(i, h, statedb, dposCtx)
		}
		bc.blocks = append(bc.blocks, h)
		bc.states = append(bc.states, statedb.Copy())
		bc.dposCtxs = append(bc.dposCtxs, dposCtx.Copy())
		if curTime == 0 {
			curTime = timeFirstBlock
		} else {
			curTime += BlockInterval
		}
		parentHash = curHash
	}
	return bc, nil
}

func (bc *fakeBlockChain) GetHeaderByNumber(bn uint64) *types.Header {
	if int(bn) >= len(bc.blocks) {
		return nil
	}
	return bc.blocks[int(bn)]
}

func (bc *fakeBlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	bn := hashToNumber(hash)
	return bc.GetHeaderByNumber(bn)
}

func (bc *fakeBlockChain) GetHeader(hash common.Hash, bn uint64) *types.Header {
	expectBN := hashToNumber(hash)
	if expectBN != bn {
		return nil
	}
	return bc.GetHeaderByNumber(bn)
}

func (bc *fakeBlockChain) StateAt(hash common.Hash) (*state.StateDB, error) {
	bn := hashToNumber(hash)
	if int(bn) >= len(bc.states) {
		return nil, fmt.Errorf("unknown state %x", hash)
	}
	return bc.states[int(bn)], nil
}

func (bc *fakeBlockChain) DposCtxAt(roots *types.DposContextRoot) (*types.DposContext, error) {
	bn := hashToNumber(roots.EpochRoot)
	if int(bn) >= len(bc.dposCtxs) {
		return nil, fmt.Errorf("unkwown dpos context %x", roots.EpochRoot)
	}
	return bc.dposCtxs[int(bn)], nil
}

func makeBlockHash(i int) common.Hash {
	var hash common.Hash
	binary.LittleEndian.PutUint64(hash[:], uint64(i))
	return hash
}

func hashToNumber(hash common.Hash) uint64 {
	bn := binary.LittleEndian.Uint64(hash[:])
	return bn
}
