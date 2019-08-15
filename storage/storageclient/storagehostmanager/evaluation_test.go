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
		factor := shm.presenceFactorCalc(info)
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
			factorSmaller := shm.presenceFactorCalc(storage.HostInfo{FirstSeen: firstSeen + 1})
			factorLarger := shm.presenceFactorCalc(storage.HostInfo{FirstSeen: firstSeen - 1})
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
	factor := shm.presenceFactorCalc(info)
	if firstSeen > blockHeight && factor != 0 {
		t.Errorf("Illegal input for presence factor calculation does not give 0 factor")
	}
}

// TestDepositFactorCalc test the functionality of StorageHostManagerdepositFactorCalc
func TestDepositFactorCalc(t *testing.T) {

}

// TestEvalHostMarketDeposit test the normal functionality of TestEvalHostMarketDeposit
func TestEvalHostMarketDeposit(t *testing.T) {

}

// TestEvalHostDeposit test evalHostDeposit
func TestEvalHostDeposit(t *testing.T) {
	tests := []struct {
		contractPrice          common.BigInt
		fund                   common.BigInt
		numHosts               uint64
		storagePrice           common.BigInt
		depositPrice           common.BigInt
		MaxDoposit             common.BigInt
		UploadBandwithPrice    common.BigInt
		DownloadBandwidthPrice common.BigInt
		expectedDeposit        common.BigInt
	}{
		{
			// normal case, the maxDeposit is larger than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			MaxDoposit:      common.NewBigIntUint64(30000), // MaxDeposit is enough
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, the maxDeposit is smaller than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			MaxDoposit:      common.NewBigIntUint64(15000), // MaxDeposit is not enough
			expectedDeposit: common.NewBigIntUint64(15000), // result capped at max dpeposit
		},
		{
			// client fund is not enough to pay the contract price
			contractPrice:   common.NewBigIntUint64(20000),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			MaxDoposit:      common.NewBigIntUint64(15000),
			expectedDeposit: common.BigInt0,
		},
		{
			// numHost set to 0, the settings should be regulated to 1
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        0,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			MaxDoposit:      common.NewBigIntUint64(30000), // MaxDeposit is enough
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// negative settings
			contractPrice:          common.BigInt0.Sub(common.NewBigIntUint64(100)),
			fund:                   common.BigInt0.Sub(common.NewBigIntUint64(100)),
			numHosts:               0,
			storagePrice:           common.BigInt0.Sub(common.NewBigIntUint64(100)),
			depositPrice:           common.BigInt0.Sub(common.NewBigIntUint64(100)),
			MaxDoposit:             common.BigInt0.Sub(common.NewBigIntUint64(100)),
			UploadBandwithPrice:    common.BigInt0.Sub(common.NewBigIntUint64(100)),
			DownloadBandwidthPrice: common.BigInt0.Sub(common.NewBigIntUint64(100)),
			expectedDeposit:        common.NewBigIntUint64(0),
		},
	}
	for index, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          test.contractPrice,
				StoragePrice:           test.storagePrice,
				Deposit:                test.depositPrice,
				MaxDeposit:             test.MaxDoposit,
				UploadBandwidthPrice:   test.UploadBandwithPrice,
				DownloadBandwidthPrice: test.DownloadBandwidthPrice,
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
