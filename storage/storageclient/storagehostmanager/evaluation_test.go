// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/storage"
)

func init() {
	// initialize the erasure coding related params
	defaultNumSectors = 2
	defaultMinSectors = 1
}

// TestDefaultEvaluator_calFinalScore test defaultEvaluator.calcFinalScore
func TestDefaultEvaluator_calFinalScore(t *testing.T) {
	tests := []struct {
		scs    defaultEvaluationScores
		expect int64
	}{
		{scs: defaultEvaluationScores{1, 1, 1, 1, 1, 1}, expect: scoreDefaultBase},
		{scs: defaultEvaluationScores{0, 0, 0, 0, 0, 0}, expect: minScore},
	}
	for i, test := range tests {
		de := &defaultEvaluator{}
		sc := de.calcFinalScore(&test.scs)
		if sc != test.expect {
			t.Errorf("Test %v: unexpected score. Got %v, Expect %v", i, sc, test.expect)
		}
	}
}

func TestPresenceScoreCalc(t *testing.T) {
	tests := []struct {
		presence uint64
	}{
		{lowTimeLimit},
		{lowTimeLimit + 1},
		{10},
		{100},
		{1000},
		{10000},
		{100000},
		{highTimeLimit - 1},
		{highTimeLimit},
		{highTimeLimit + 1},
	}
	for _, test := range tests {
		// first Seen must not be 0
		firstSeen := uint64(10)
		hm := &fakeHostMarket{
			blockNumber: test.presence + firstSeen,
		}
		info := storage.HostInfo{
			FirstSeen: firstSeen,
		}
		score := presenceScoreCalc(info, hm)
		if test.presence <= lowTimeLimit {
			if score != lowValueLimit {
				t.Errorf("low limit test failed")
			}
		} else if test.presence >= highTimeLimit {
			if score != highValueLimit {
				t.Errorf("high limit test failed")
			}
		} else {
			// Calculate the value before and after the limit, and check whether the
			// score is incrementing
			if test.presence == 0 || test.presence == math.MaxUint64 {
				continue
			}
			factorSmaller := presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen + 1}, hm)
			factorLarger := presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen - 1}, hm)
			if factorSmaller >= score || score >= factorLarger {
				t.Errorf("Near range %d the score not incrementing", test.presence)
			}
		}
	}
}

// TestIllegalPresenceFactorCalc test the illegal case of host manager's blockHeight is smaller
// than the first seen, which should yield a factor of 0
func TestIllegalPresenceScoreCalc(t *testing.T) {
	firstSeen := uint64(30)
	blockHeight := uint64(10)
	hm := &fakeHostMarket{
		blockNumber: blockHeight,
	}
	info := storage.HostInfo{
		FirstSeen: firstSeen,
	}
	score := presenceScoreCalc(info, hm)
	if firstSeen > blockHeight && score != 0 {
		t.Errorf("Illegal input for presence factor calculation does not give 0 factor")
	}
}

// TestDepositFactorCalc test the functionality of StorageHostManagerdepositFactorCalc
func TestDepositScoreCalc(t *testing.T) {
	tests := []struct {
		ratio float64
	}{
		{0},
		{0.001},
		{0.01},
		{0.1},
		{1},
		{10},
		{100},
		{1000},
	}
	var lastResult float64
	for index, test := range tests {
		rent := storage.RentPayment{
			Fund:         common.NewBigIntUint64(10000000),
			StorageHosts: 1,
		}
		marketDeposit := common.NewBigIntUint64(10000)
		storagePrice := common.NewBigIntUint64(5000)
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				StoragePrice: storagePrice,
				Deposit:      marketDeposit.MultFloat64(test.ratio),
				MaxDeposit:   common.NewBigIntUint64(math.MaxUint64),
			},
		}
		market := &fakeHostMarket{
			storagePrice: storagePrice,
			deposit:      marketDeposit,
			maxDeposit:   common.NewBigIntUint64(math.MaxUint64),
		}
		res := depositScoreCalc(info, rent, market)
		// Check the result is within range [0, 1)
		if res < 0 || res >= 1 {
			t.Errorf("Test %d illegal factor. Got %v", index, res)
		}
		// Check whether the result is larger than last result
		if index == 0 {
			lastResult = 0
			continue
		}
		if res <= lastResult {
			t.Errorf("Test %d not incrementing. Got %v, Last %v", index, res, lastResult)
		}
		lastResult = res
	}
}

// TestEvalHostMarketDeposit test the normal functionality of TestEvalHostMarketDeposit
func TestEvalHostMarketDeposit(t *testing.T) {
	tests := []struct {
		contractPrice common.BigInt
		storagePrice  common.BigInt
		deposit       common.BigInt
		maxDeposit    common.BigInt

		fund     common.BigInt
		numHosts uint64

		expectedDeposit common.BigInt
	}{
		{
			contractPrice:   common.NewBigIntUint64(0),
			storagePrice:    common.NewBigIntUint64(100),
			deposit:         common.NewBigIntUint64(200),
			maxDeposit:      common.NewBigIntUint64(30000),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			expectedDeposit: common.NewBigIntUint64(20000),
		},
	}
	for index, test := range tests {
		market := &fakeHostMarket{
			contractPrice: test.contractPrice,
			storagePrice:  test.storagePrice,
			deposit:       test.deposit,
			maxDeposit:    test.maxDeposit,
		}
		settings := storage.RentPayment{
			Fund:         test.fund,
			StorageHosts: test.numHosts,
		}
		res := evalHostMarketDeposit(settings, market)
		if res.Cmp(test.expectedDeposit) != 0 {
			t.Errorf("test %d deposit not expected. Got %v, Expect %v", index, res, test.expectedDeposit)
		}
	}
}

// TestEvalHostDeposit test evalHostDeposit
func TestEvalHostDeposit(t *testing.T) {
	tests := []struct {
		contractPrice          common.BigInt
		fund                   common.BigInt
		numHosts               uint64
		storagePrice           common.BigInt
		depositPrice           common.BigInt
		maxDoposit             common.BigInt
		uploadBandwithPrice    common.BigInt
		downloadBandwidthPrice common.BigInt
		expectedDeposit        common.BigInt
	}{
		{
			// normal case, the maxDeposit is larger than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(30000),
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, multiple hosts
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(45000),
			numHosts:        3,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(30000),
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, the maxDeposit is smaller than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(15000), // maxDeposit is not enough
			expectedDeposit: common.NewBigIntUint64(15000), // result capped at max deposit
		},
		{
			// client fund is not enough to pay the contract price
			contractPrice:   common.NewBigIntUint64(20000),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(15000),
			expectedDeposit: common.BigInt0,
		},
	}
	for index, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          test.contractPrice,
				StoragePrice:           test.storagePrice,
				Deposit:                test.depositPrice,
				MaxDeposit:             test.maxDoposit,
				UploadBandwidthPrice:   test.uploadBandwithPrice,
				DownloadBandwidthPrice: test.downloadBandwidthPrice,
			},
		}
		settings := storage.RentPayment{
			Fund:         test.fund,
			StorageHosts: test.numHosts,
		}
		res := evalHostDeposit(info, settings)
		if res.Cmp(test.expectedDeposit) != 0 {
			t.Errorf("Test %d deposit not expected. Got %v, Expect %v", index, res, test.expectedDeposit)
		}
	}
}

// TestStorageRemainingFactorCalc test the functionality of storageRemainingScoreCalc
func TestStorageRemainingFactorCalc(t *testing.T) {
	expectedStorage := uint64(1e6)
	tests := []struct {
		remainingStorage uint64
		numHosts         uint64
	}{
		{1e3, 1},
		{3.3e3, 3},
		{3e4, 3},
		{3e5, 3},
		{3e6, 3},
		{3e7, 3},
		{3e8, 3},
		{3e9, 3},
	}
	var lastResult float64
	for index, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: test.remainingStorage,
			},
		}
		settings := storage.RentPayment{
			ExpectedStorage: expectedStorage,
			StorageHosts:    test.numHosts,
		}
		res := storageRemainingScoreCalc(info, settings)
		if res < 0 || res >= 1 {
			t.Errorf("invalid result: %v", res)
		}
		if index == 0 {
			lastResult = res
			continue
		}
		// the res should be incrementing
		if res < lastResult {
			t.Errorf("test %d, the factor not incrementing. Got %v, previous %v", index, res, lastResult)
		}
		lastResult = res
	}
}

// TestStorageRemainingScoreCalc test the functionality of storageRemainingScoreCalc.
// The returned score should be within range [0, 1), and increment as remaining storage increases
func TestStorageRemainingScoreCalc(t *testing.T) {
	baseStorage := uint64(10000000000)
	tests := []struct {
		remainingStorage uint64
		expectedStorage  uint64
		numHosts         uint64
	}{
		{0, baseStorage, 1},
		{uint64(0.1 * float64(baseStorage)), baseStorage, 1},
		{uint64(0.5 * float64(baseStorage)), baseStorage, 1},
		{baseStorage, baseStorage * 30, 30},
		{baseStorage * 3, baseStorage, 1},
		{10 * baseStorage, baseStorage, 1},
		{30 * baseStorage, baseStorage, 1},
		{100 * baseStorage, baseStorage, 1},
	}
	type scoreRecord struct{ ratio, sc float64 }
	var results []scoreRecord
	for _, test := range tests {
		ratio := float64(test.remainingStorage) * float64(test.numHosts) * float64(defaultMinSectors) /
			float64(defaultNumSectors) / float64(test.expectedStorage)
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: test.remainingStorage,
			},
		}
		settings := storage.RentPayment{
			ExpectedStorage: test.expectedStorage,
			StorageHosts:    test.numHosts,
		}
		sc := storageRemainingScoreCalc(info, settings)
		// Check whether the score is within range 0 to 1
		if sc < 0 || sc >= 1 {
			t.Fatalf("unexpected score %v. Not within range [0, 1)", sc)
		}
		// Add the result to results
		results = append(results, scoreRecord{ratio, sc})

		if test.remainingStorage == 0 {
			continue
		}
		// Check whether the score in incrementing with the remaining storage
		const decrementRatio = 0.99
		const incrementRatio = 1.01
		lowInfo := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: uint64(float64(test.remainingStorage) * decrementRatio),
			},
		}
		highInfo := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: uint64(float64(test.remainingStorage) * incrementRatio),
			},
		}
		lowSc := storageRemainingScoreCalc(lowInfo, settings)
		highSc := storageRemainingScoreCalc(highInfo, settings)

		if sc <= lowSc || sc >= highSc {
			t.Fatalf("In ratio %v, score not incrementing: %v -> %v -> %v", ratio, lowSc, sc, highSc)
		}
	}
	// Display the result
	for _, result := range results {
		t.Logf("At remainingStorage / expectedStorage ratio %7.4f, got score %7.4f", result.ratio, result.sc)
	}
}

// TestEvalContractCost test the functionality of evalContractCost
func TestEvalContractCost(t *testing.T) {
	tests := []struct {
		contractPrice    common.BigInt
		storagePrice     common.BigInt
		uploadPrice      common.BigInt
		downloadPrice    common.BigInt
		numHosts         uint64
		period           uint64
		expectedUpload   uint64
		expectedDownload uint64
		expectedStorage  uint64
		expectedCost     common.BigInt
	}{
		{
			contractPrice: common.NewBigInt(1),
			numHosts:      1,
			expectedCost:  common.NewBigInt(2),
		},
		{
			storagePrice:    common.NewBigInt(10),
			numHosts:        1,
			period:          10,
			expectedStorage: 5,
			expectedCost:    common.NewBigInt(1000),
		},
		{
			uploadPrice:    common.NewBigInt(100),
			numHosts:       1,
			expectedUpload: 10,
			expectedCost:   common.NewBigInt(1000),
		},
		{
			downloadPrice:    common.NewBigInt(100),
			numHosts:         1,
			expectedDownload: 10,
			expectedCost:     common.NewBigInt(1000),
		},
		{
			contractPrice:    common.NewBigInt(1),
			storagePrice:     common.NewBigInt(10),
			uploadPrice:      common.NewBigInt(100),
			downloadPrice:    common.NewBigInt(100),
			numHosts:         1,
			period:           10,
			expectedUpload:   10,
			expectedDownload: 10,
			expectedStorage:  5,
			expectedCost:     common.NewBigInt(3002),
		},
	}
	for i, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          test.contractPrice,
				StoragePrice:           test.storagePrice,
				UploadBandwidthPrice:   test.uploadPrice,
				DownloadBandwidthPrice: test.downloadPrice,
			},
		}
		rent := storage.RentPayment{
			StorageHosts:     test.numHosts,
			Period:           test.period,
			ExpectedStorage:  test.expectedStorage,
			ExpectedDownload: test.expectedDownload,
			ExpectedUpload:   test.expectedUpload,
		}
		cost := evalContractCost(info, rent)
		if cost.Cmp(test.expectedCost) != 0 {
			t.Errorf("Test %v: cost not expected. Got %v, Expect %v", i, cost, test.expectedCost)
		}
	}

}
