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

func TestPresenceFactorCalc(t *testing.T) {
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
		shm := &StorageHostManager{
			blockHeight: test.presence + firstSeen,
		}
		info := storage.HostInfo{
			FirstSeen: firstSeen,
		}
		factor := shm.presenceScoreCalc(info)
		if test.presence <= lowTimeLimit {
			if factor != lowValueLimit {
				t.Errorf("low limit test failed")
			}
		} else if test.presence >= highTimeLimit {
			if factor != highValueLimit {
				t.Errorf("high limit test failed")
			}
		} else {
			// Calculate the value before and after the limit, and check whether the
			// factor is incrementing
			if test.presence == 0 || test.presence == math.MaxUint64 {
				continue
			}
			factorSmaller := shm.presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen + 1})
			factorLarger := shm.presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen - 1})
			if factorSmaller >= factor || factor >= factorLarger {
				t.Errorf("Near range %d the factor not incrementing", test.presence)
			}
		}
	}
}

// TestIllegalPresenceFactorCalc test the illegal case of host manager's blockHeight is smaller
// than the first seen, which should yield a factor of 0
func TestIllegalPresenceFactorCalc(t *testing.T) {
	firstSeen := uint64(30)
	blockHeight := uint64(10)
	shm := &StorageHostManager{
		blockHeight: blockHeight,
	}
	info := storage.HostInfo{
		FirstSeen: firstSeen,
	}
	factor := shm.presenceScoreCalc(info)
	if firstSeen > blockHeight && factor != 0 {
		t.Errorf("Illegal input for presence factor calculation does not give 0 factor")
	}
}

// TestDepositFactorCalc test the functionality of StorageHostManagerdepositFactorCalc
func TestDepositFactorCalc(t *testing.T) {
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
		shm := &StorageHostManager{}
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
		res := shm.depositScoreCalc(info, rent, market)
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
			maxDoposit:      common.NewBigIntUint64(30000), // maxDeposit is enough
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, multiple hosts
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(45000),
			numHosts:        3,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(30000), // maxDeposit is enough
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
			expectedDeposit: common.NewBigIntUint64(15000), // result capped at max dpeposit
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
		{
			// negative and zero settings
			contractPrice:          common.BigInt0.Sub(common.NewBigIntUint64(100)),
			fund:                   common.BigInt0.Sub(common.NewBigIntUint64(100)),
			numHosts:               0,
			storagePrice:           common.BigInt0.Sub(common.NewBigIntUint64(100)),
			depositPrice:           common.BigInt0.Sub(common.NewBigIntUint64(100)),
			maxDoposit:             common.BigInt0.Sub(common.NewBigIntUint64(100)),
			uploadBandwithPrice:    common.BigInt0.Sub(common.NewBigIntUint64(100)),
			downloadBandwidthPrice: common.BigInt0.Sub(common.NewBigIntUint64(100)),
			expectedDeposit:        common.NewBigIntUint64(0),
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
		shm := &StorageHostManager{}
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: test.remainingStorage,
			},
		}
		settings := storage.RentPayment{
			ExpectedStorage: expectedStorage,
			StorageHosts:    test.numHosts,
		}
		res := shm.storageRemainingScoreCalc(info, settings)
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
