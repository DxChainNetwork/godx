// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethash

import (
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core/types"
)

// This test checks that cache lru logic doesn't crash under load.
// It reproduces https://github.com/ethereum/go-ethereum/issues/14943
func TestCacheFileEvict(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "ethash-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	e := New(Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: tmpdir, PowMode: ModeTest}, nil, false)
	defer e.Close()

	workers := 8
	epochs := 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go verifyTest(&wg, e, i, epochs)
	}
	wg.Wait()
}

func verifyTest(wg *sync.WaitGroup, e *Ethash, workerIndex, epochs int) {
	defer wg.Done()

	const wiggle = 4 * epochLength
	r := rand.New(rand.NewSource(int64(workerIndex)))
	for epoch := 0; epoch < epochs; epoch++ {
		block := int64(epoch)*epochLength - wiggle/2 + r.Int63n(wiggle)
		if block < 0 {
			block = 0
		}
		header := &types.Header{Number: big.NewInt(block), Difficulty: big.NewInt(100)}
		e.VerifySeal(nil, header)
	}
}

func TestRemoteSealer(t *testing.T) {
	ethash := NewTester(nil, false)
	defer ethash.Close()

	api := &API{ethash}
	if _, err := api.GetWork(); err != errNoMiningWork {
		t.Error("expect to return an error indicate there is no mining work")
	}
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(100)}
	block := types.NewBlockWithHeader(header)
	sealhash := ethash.SealHash(header)

	// Push new work.
	results := make(chan *types.Block)
	ethash.Seal(nil, block, results, nil)

	var (
		work [4]string
		err  error
	)
	if work, err = api.GetWork(); err != nil || work[0] != sealhash.Hex() {
		t.Error("expect to return a mining work has same hash")
	}

	if res := api.SubmitWork(types.BlockNonce{}, sealhash, common.Hash{}); res {
		t.Error("expect to return false when submit a fake solution")
	}
	// Push new block with same block number to replace the original one.
	header = &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1000)}
	block = types.NewBlockWithHeader(header)
	sealhash = ethash.SealHash(header)
	ethash.Seal(nil, block, results, nil)

	if work, err = api.GetWork(); err != nil || work[0] != sealhash.Hex() {
		t.Error("expect to return the latest pushed work")
	}
}

func TestClosedRemoteSealer(t *testing.T) {
	ethash := NewTester(nil, false)
	time.Sleep(1 * time.Second) // ensure exit channel is listening
	ethash.Close()

	api := &API{ethash}
	if _, err := api.GetWork(); err != errEthashStopped {
		t.Error("expect to return an error to indicate ethash is stopped")
	}

	if res := api.SubmitHashRate(hexutil.Uint64(100), common.HexToHash("a")); res {
		t.Error("expect to return false when submit hashrate to a stopped ethash")
	}
}

// my test cases
func TestHashRate(t *testing.T) {
	var (
		hashrate = []hexutil.Uint64{100, 200, 300}
		expect   uint64
		ids      = []common.Hash{common.HexToHash("a"), common.HexToHash("b"), common.HexToHash("c")}
	)
	ethash := NewTester(nil, false)
	defer ethash.Close()

	if tot := ethash.Hashrate(); tot != 0 {
		t.Error("expect the result should be zero")
	}

	api := &API{ethash}
	for i := 0; i < len(hashrate); i += 1 {
		if res := api.SubmitHashRate(hashrate[i], ids[i]); !res {
			t.Error("remote miner submit hashrate failed")
		}
		expect += uint64(hashrate[i])
	}
	if tot := ethash.Hashrate(); tot != float64(expect) {
		t.Error("expect total hashrate should be same")
	}

	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1 << 16)}
	if _, err := mineBlock(ethash, header, 100*time.Second); err != nil {
		t.Error("mine timeout error")
	}

	// We don't have to update hash rate on every nonce, so update after after 2^X nonces
	tot := ethash.Hashrate()
	t.Logf("After mining, hashrate : %f\n", tot)

}

func TestEthash_Threads(t *testing.T) {
	ethash := NewTester(nil, false)
	defer ethash.Close()

	if ethash.Threads() < 0 {
		t.Error("mine threads less than zero")
	}

	var threads = 10
	ethash.SetThreads(threads)
	if threads != ethash.Threads() {
		t.Error("mine threads inequal set value")
	}
}

func TestEthash_Close(t *testing.T) {
	ethash := NewTester(nil, false)
	if err := ethash.Close(); err != nil {
		t.Error(err)
	}
}

func TestEthash_APIs(t *testing.T) {
	ethash := NewTester(nil, false)
	defer ethash.Close()

	apis := ethash.APIs(nil)
	if apis == nil || len(apis) != 2 {
		t.Error("fetch ethash apis error")
	}

	if apis[0].Namespace != "eth" || apis[1].Namespace != "ethash" {
		t.Error("api info mismatch")
	}
}

func BenchmarkSeedHash(b *testing.B) {
	b.ResetTimer()
	for block := 0; block < epochLength*b.N; block += epochLength {
		SeedHash(uint64(block))
	}
}
