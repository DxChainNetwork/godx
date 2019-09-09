package miner

import (
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/params"
)

var (
	coinbaseAddress common.Address
	consensusEngine *dpos.Dpos
	backend         *testWorkerBackend
	miner           *Miner
)

func init() {
	coinbaseAddress = common.HexToAddress("0xD36722ADeC3EdCB29c8e7b5a47f352D701393462")
	consensusEngine = dpos.NewDposFaker()
	backend = newTestWorkerBackend(new(testing.T), params.DposChainConfig, consensusEngine, 0)
	miner = New(backend, params.TestChainConfig, new(event.TypeMux), consensusEngine, time.Second, params.GenesisGasLimit, params.GenesisGasLimit, nil)
}

func TestMiner_StartAndStop(t *testing.T) {
	t.Log("1-yes  0-no")
	t.Logf("[Before start] canstart: %d | shouldstart: %d | worker running: %d\n", miner.canStart, miner.shouldStart, miner.worker.running)

	miner.Start(coinbaseAddress)

	t.Logf("[After start] canstart: %d | shouldstart: %d | worker running: %d\n", miner.canStart, miner.shouldStart, miner.worker.running)

	miner.Stop()
	t.Logf("[After stop] canstart: %d | shouldstart: %d | worker running: %d\n", miner.canStart, miner.shouldStart, miner.worker.running)
}

func TestMiner_Pending(t *testing.T) {
	block, state := miner.Pending()
	if block == nil || state == nil {
		t.Error("retrieve pending block error")
	}

	t.Logf("block hash: %s | number: %d | coinbase: %s\n", block.Hash().Hex(), block.NumberU64(), block.Coinbase().Hex())
	t.Logf("coinbase balance: %d\n", state.GetBalance(block.Coinbase()).Uint64())
}

func TestMiner_Mining(t *testing.T) {
	miner.Start(coinbaseAddress)
	if !miner.Mining() {
		t.Error("start failed")
	}
}

func TestMiner_HashRate(t *testing.T) {
	miner.Start(coinbaseAddress)

	hashRate := miner.HashRate()
	if hashRate < 0 {
		t.Error("HashRate is lower than zero")
	}
	t.Logf("hashrate: %d\n", miner.HashRate())
}
