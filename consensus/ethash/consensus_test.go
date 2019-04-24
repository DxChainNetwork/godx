package ethash

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/ethdb"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/params"
)

var (
	ethash       = NewTester(nil, false)
	parentHeader = &types.Header{Number: big.NewInt(1150000), Difficulty: big.NewInt(1000), Coinbase: common.HexToAddress("0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba"), Time: big.NewInt(time.Now().Unix() - 100000000), GasLimit: params.MinGasLimit + 100}
)

type diffTest struct {
	ParentTimestamp    uint64
	ParentDifficulty   *big.Int
	CurrentTimestamp   uint64
	CurrentBlocknumber *big.Int
	CurrentDifficulty  *big.Int
}

func (d *diffTest) UnmarshalJSON(b []byte) (err error) {
	var ext struct {
		ParentTimestamp    string
		ParentDifficulty   string
		CurrentTimestamp   string
		CurrentBlocknumber string
		CurrentDifficulty  string
	}
	if err := json.Unmarshal(b, &ext); err != nil {
		return err
	}

	d.ParentTimestamp = math.MustParseUint64(ext.ParentTimestamp)
	d.ParentDifficulty = math.MustParseBig256(ext.ParentDifficulty)
	d.CurrentTimestamp = math.MustParseUint64(ext.CurrentTimestamp)
	d.CurrentBlocknumber = math.MustParseBig256(ext.CurrentBlocknumber)
	d.CurrentDifficulty = math.MustParseBig256(ext.CurrentDifficulty)

	return nil
}

func TestCalcDifficulty(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "testdata", "difficulty.json"))
	if err != nil {
		t.Skip(err)
	}
	defer file.Close()

	tests := make(map[string]diffTest)
	err = json.NewDecoder(file).Decode(&tests)
	if err != nil {
		t.Fatal(err)
	}

	config := &params.ChainConfig{HomesteadBlock: big.NewInt(1150000), ByzantiumBlock: big.NewInt(4370000), ConstantinopleBlock: big.NewInt(7280000)}

	for name, test := range tests {
		number := new(big.Int).Sub(test.CurrentBlocknumber, big.NewInt(1))
		diff := CalcDifficulty(config, test.CurrentTimestamp, &types.Header{
			Number:     number,
			Time:       new(big.Int).SetUint64(test.ParentTimestamp),
			Difficulty: test.ParentDifficulty,
		})

		// test difficulty inequal
		if name == "ByzantiumExpDiffIncrease" {
			if diff.Cmp(test.CurrentDifficulty) == 0 {
				t.Error(name, "failed. we expect diff is not different")
			}
			continue
		}

		if diff.Cmp(test.CurrentDifficulty) != 0 {
			t.Error(name, "failed. Expected", test.CurrentDifficulty, "and calculated", diff)
		}
	}
}

/*
    _____      _            _         ______                _   _               _______        _
   |  __ \    (_)          | |       |  ____|              | | (_)             |__   __|      | |
   | |__) | __ ___   ____ _| |_ ___  | |__ _   _ _ __   ___| |_ _  ___  _ __      | | ___  ___| |_
   |  ___/ '__| \ \ / / _` | __/ _ \ |  __| | | | '_ \ / __| __| |/ _ \| '_ \     | |/ _ \/ __| __|
   | |   | |  | |\ V / (_| | ||  __/ | |  | |_| | | | | (__| |_| | (_) | | | |    | |  __/\__ \ |_
   |_|   |_|  |_| \_/ \__,_|\__\___| |_|   \__,_|_| |_|\___|\__|_|\___/|_| |_|    |_|\___||___/\__|

*/

func TestEthash_Prepare(t *testing.T) {
	// TODO
	// with chainReader

}

func TestEthash_Finalize(t *testing.T) {
	// TODO
	// with chainReader and stateDB
}

func TestEthash_VerifyHeaders(t *testing.T) {
	// TODO
	// need real block chain
}

func TestEthash_VerifyUncles(t *testing.T) {
	// TODO
	// need real block chain
}

func TestEthash_Seal(t *testing.T) {
	results := make(chan *types.Block)
	defer close(results)

	// for 10 times, the sealed block in every loop is different, despite the same header property
	for i := 0; i < 5; i++ {
		header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1000)}
		if err := ethash.Seal(nil, types.NewBlockWithHeader(header), results, nil); err != nil {
			t.Error("Seal is Failed!")
		}

		select {
		case block := <-results:
			// cmp Difficulty
			if err := ethash.VerifySeal(nil, block.Header()); err != nil {
				t.Fatalf("unexpected block verification error: %v", err)
			}
			t.Logf("%d: Block Hash: %v | Block Nouce: %d \n", i, block.Hash().Hex(), block.Nonce())
		case <-time.NewTimer(100 * time.Second).C:
			t.Error("sealing result timeout")
		}
	}

	// compute different cost time under difficulty with 8 step
	two16 := new(big.Int).Exp(big.NewInt(2), big.NewInt(16), big.NewInt(0))
	difficulty := big.NewInt(1)
	for {
		if difficulty.Cmp(two16) > 0 {
			break
		}

		header := &types.Header{Number: big.NewInt(1), Difficulty: difficulty}

		start := time.Now()
		if err := ethash.Seal(nil, types.NewBlockWithHeader(header), results, nil); err != nil {
			t.Error("Seal is Failed!")
		}
		select {
		case block := <-results:
			if err := ethash.VerifySeal(nil, block.Header()); err != nil {
				t.Fatalf("unexpected block verification error: %v", err)
			}
		case <-time.NewTimer(100 * time.Second).C:
		}

		cost := time.Since(start)
		t.Logf("Difficulty: %v, CostTime = [%s]\n", difficulty, cost)

		difficulty = difficulty.Mul(difficulty, big.NewInt(8))
	}

}

func TestEthash_SealHash(t *testing.T) {
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1000)}
	sealHash := ethash.SealHash(header)
	blockHash := types.NewBlockWithHeader(header).Hash()

	// block hash different from seal hash
	if strings.Compare(sealHash.Hex(), blockHash.Hex()) == 0 {
		t.Error("sealHash and blockHash is equal")
	}

	// the seal hash with nonce field is equal to the former without nonce
	headerWithNonce := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1000), Nonce: types.EncodeNonce(0xff00000000000000)}
	sealHashWithNonce := ethash.SealHash(headerWithNonce)
	if strings.Compare(sealHash.Hex(), sealHashWithNonce.Hex()) != 0 {
		t.Error("sealHash and with nonce is not equal")
	}

}

func TestEthash_VerifyHeader(t *testing.T) {
	header := &types.Header{ParentHash: common.HexToHash("0xa8b4f02ce7cac2047999dda3f507d46c68188dce95ae4e74c490a2d57c5f795e"), Number: big.NewInt(1150001), Time: big.NewInt(time.Now().Unix()), GasLimit: params.MinGasLimit + 101}

	config := &params.ChainConfig{HomesteadBlock: big.NewInt(1150000)}
	diff := CalcDifficulty(config, header.Time.Uint64(), parentHeader)
	header.Difficulty = diff

	if block, err := mineBlock(ethash, header, 100*time.Second); err != nil {
		t.Errorf("failed to seal block: %v", err)
	} else {
		header := block.Header()

		err := ethash.verifyHeader(&fakeChainReader{}, header, parentHeader, false, false)
		if err != nil {
			t.Errorf("failed verify header")
			fmt.Println(err)
		}

		// we modify header field in order to fail verify
		originalTime := header.Time
		header.Time = new(big.Int).Sub(parentHeader.Time, big.NewInt(10000))
		err = ethash.verifyHeader(&fakeChainReader{}, header, parentHeader, false, false)
		if err != errZeroBlockTime {
			t.Errorf("not expect errZeroBlockTime error")
		}

		header.Time = originalTime
		header.GasLimit = params.MinGasLimit - 100
		err = ethash.verifyHeader(&fakeChainReader{}, header, parentHeader, false, false)
		if err == nil {
			t.Errorf("not expect gaslimit error")
		}
	}
}

func TestEthash_VerifySeal(t *testing.T) {
	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(1 << 10)}
	if block, err := mineBlock(ethash, header, 100*time.Second); err != nil {
		t.Error("mine block failed!")
	} else {
		if err := ethash.VerifySeal(nil, block.Header()); err != nil {
			t.Fatalf("unexpected block verification error: %v", err)
		}
	}
}

func TestUnclesReward(t *testing.T) {
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1),
		HomesteadBlock:      big.NewInt(1150000),
		ByzantiumBlock:      big.NewInt(4370000),
		ConstantinopleBlock: big.NewInt(7280000),
	}

	stateDb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))

	coinbaseHead := common.HexToAddress("0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba")
	coinbaseSide1 := common.HexToAddress("0x2adc25665018aa1fe0e6bc666dac8fc2697ff900")
	coinbaseSide2 := common.HexToAddress("0x2adc25665018aa1fe0e6bc666dac8fc2697ff901")

	blockNumbers := [3]*big.Int{big.NewInt(100), big.NewInt(4370000), big.NewInt(7300000)}
	blockRewards := [3]*big.Int{FrontierBlockReward, ByzantiumBlockReward, ConstantinopleBlockReward}

	header := &types.Header{ParentHash: parentHeader.Hash(), Coinbase: coinbaseHead, Time: big.NewInt(60)}
	uncleHeader1 := &types.Header{ParentHash: parentHeader.Hash(), Coinbase: coinbaseSide1, Time: big.NewInt(40)}
	uncleHeader2 := &types.Header{ParentHash: parentHeader.Hash(), Coinbase: coinbaseSide2, Time: big.NewInt(35)}

	for i, v := range blockNumbers {
		header.Number = v

		// 1 uncle
		for j := 1; j < 7; j++ {
			blockReward := blockRewards[i]
			uncleHeader1.Number = new(big.Int).Sub(v, big.NewInt(int64(j)))
			accumulateRewards(config, stateDb, header, []*types.Header{uncleHeader1})
			headBlockReward := stateDb.GetBalance(coinbaseHead)
			expectedBlcokReward := new(big.Int).Add(blockReward, new(big.Int).Div(blockReward, big32))

			if headBlockReward.Cmp(expectedBlcokReward) != 0 {
				t.Errorf("failed, head expectedReward: %d, return: %d", expectedBlcokReward, headBlockReward)
			}

			sideBlockReward1 := stateDb.GetBalance(coinbaseSide1)
			expectedSideBlockReward := new(big.Int).Div(new(big.Int).Mul(big.NewInt(int64(8-j)), blockReward), big8)

			if sideBlockReward1.Cmp(expectedSideBlockReward) != 0 {
				t.Errorf("failed, side expectedReward: %d, return: %d", expectedSideBlockReward, sideBlockReward1)
			}

			stateDb.Reset(common.Hash{})
		}

		// 2 uncles
		for j := 1; j < 7; j++ {
			blockReward := blockRewards[i]
			uncleHeader1.Number = new(big.Int).Sub(v, big.NewInt(int64(j)))
			uncleHeader2.Number = new(big.Int).Sub(v, big.NewInt(int64(j)))
			accumulateRewards(config, stateDb, header, []*types.Header{uncleHeader1, uncleHeader2})
			headBlockReward := stateDb.GetBalance(coinbaseHead)
			expectedBlcokReward := new(big.Int).Add(blockReward, new(big.Int).Div(blockReward, big.NewInt(16)))

			if headBlockReward.Cmp(expectedBlcokReward) != 0 {
				t.Errorf("failed, head expectedReward: %d, return: %d", expectedBlcokReward, headBlockReward)
			}

			sideBlockReward1 := stateDb.GetBalance(coinbaseSide1)
			expectedSideBlockReward1 := new(big.Int).Div(new(big.Int).Mul(big.NewInt(int64(8-j)), blockReward), big8)

			if sideBlockReward1.Cmp(expectedSideBlockReward1) != 0 {
				t.Errorf("failed, side expectedReward: %d, return: %d", expectedSideBlockReward1, sideBlockReward1)
			}

			sideBlockReward2 := stateDb.GetBalance(coinbaseSide1)
			expectedSideBlockReward2 := new(big.Int).Div(new(big.Int).Mul(big.NewInt(int64(8-j)), blockReward), big8)

			if sideBlockReward2.Cmp(expectedSideBlockReward2) != 0 {
				t.Errorf("failed, side expectedReward: %d, return: %d", expectedSideBlockReward2, sideBlockReward1)
			}

			stateDb.Reset(common.Hash{})
		}
	}

}

func ExampleCalcDifficulty() {
	// Tip: A simple example of mining block and verify. But entire process must be associated with console, miner, transaction and blockchain modules.

	// 1. prepare header
	diff := CalcDifficulty(params.MainnetChainConfig, uint64(1000), &types.Header{
		Number:     big.NewInt(5000000),
		Time:       new(big.Int).SetUint64(900),
		Difficulty: big.NewInt(1 << 12),
	})
	header := &types.Header{Number: big.NewInt(1), Difficulty: diff}

	// 2. mine, call Seal method
	block, err := mineBlock(ethash, header, 100*time.Second)
	if err != nil || block == nil {
		fmt.Printf("mine block failed\n")
		return
	}

	// 3. verify
	if err := ethash.VerifySeal(nil, block.Header()); err != nil {
		fmt.Printf("unexpected block verification error: %v\n", err)
		return
	} else {
		fmt.Printf("Successful Mined")
	}

	// Output:
	// Successful Mined
}

func mineBlock(ethash *Ethash, header *types.Header, timeout time.Duration) (*types.Block, error) {
	results := make(chan *types.Block)
	defer close(results)

	if err := ethash.Seal(nil, types.NewBlockWithHeader(header), results, nil); err != nil {
		return nil, err
	}

	select {
	case block := <-results:
		return block, nil
	case <-time.NewTimer(timeout).C:
		return nil, errors.New("timeout")
	}
}

type fakeChainReader struct{}

func (fakeChainReader *fakeChainReader) Config() *params.ChainConfig {
	config := &params.ChainConfig{HomesteadBlock: big.NewInt(1150000)}
	return config
}

func (fakeChainReader *fakeChainReader) CurrentHeader() *types.Header {
	return nil
}

func (fakeChainReader *fakeChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}

func (fakeChainReader *fakeChainReader) GetHeaderByNumber(number uint64) *types.Header {
	return nil
}

func (fakeChainReader *fakeChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	return nil
}

func (fakeChainReader *fakeChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}
