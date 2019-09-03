// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// fakeHostMarket is a fake host market that implement hostMarket
type fakeHostMarket struct {
	blockHeight   uint64
	contractPrice common.BigInt
	storagePrice  common.BigInt
	uploadPrice   common.BigInt
	downloadPrice common.BigInt
	deposit       common.BigInt
	maxDeposit    common.BigInt
}

// GetMarketPrice return the price for the fake host manager
func (hm *fakeHostMarket) GetMarketPrice() storage.MarketPrice {
	return storage.MarketPrice{
		ContractPrice: hm.contractPrice,
		StoragePrice:  hm.storagePrice,
		UploadPrice:   hm.uploadPrice,
		DownloadPrice: hm.downloadPrice,
		Deposit:       hm.deposit,
		MaxDeposit:    hm.maxDeposit,
	}
}

// getBlockHeight return the block number of the fake host market
func (hm *fakeHostMarket) getBlockHeight() uint64 {
	return hm.blockHeight
}

// TestEmptyCalculateMarketPrice test the functionality of calculateMarketPrice when the active
// storage host have length 0
func TestEmptyCalculateMarketPrice(t *testing.T) {
	shm := StorageHostManager{
		storageHostTree: storagehosttree.New(),
	}
	marketPrice := shm.calculateMarketPrice()
	if !reflect.DeepEqual(marketPrice, defaultMarketPrice) {
		t.Errorf("Empty host tree not return default market price\n\tGot %+v\n\tExpect %+v", marketPrice,
			defaultMarketPrice)
	}
}

// TestCachedPrices_isUpdateNeeded test cachedPrices.isUpdateNeeded
func TestCachedPrices_isUpdateNeeded(t *testing.T) {
	tests := []struct {
		marketPrice storage.MarketPrice
		timeUpdate  time.Time
		expect      bool
	}{
		{
			storage.MarketPrice{},
			time.Now(),
			true,
		},
		{
			storage.MarketPrice{ContractPrice: common.BigInt1},
			time.Now(),
			false,
		},
		{
			storage.MarketPrice{ContractPrice: common.BigInt1},
			time.Now().AddDate(-1, 0, 0),
			true,
		},
	}
	for i, test := range tests {
		cp := &cachedPrices{
			prices:         test.marketPrice,
			timeLastUpdate: test.timeUpdate,
		}
		res := cp.isUpdateNeeded()
		if res != test.expect {
			t.Errorf("Test %v result not expected. Got %v, Expect %v", i, res, test.expect)
		}
	}
}

// TestGetAverage test the functionality of getAverage
func TestGetAverage(t *testing.T) {
	tests := []struct {
		infos  priceGetterSorter
		expect common.BigInt
	}{
		{
			hostInfosByContractPrice(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByContractPrice(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByContractPrice(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByContractPrice(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByStoragePrice(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByUploadPrice(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByDownloadPrice(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByDeposit(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(makeHostInfos()),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(makeShortHostInfos(0)),
			common.NewBigInt(0),
		},
		{
			hostInfosByMaxDeposit(makeShortHostInfos(1)),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(makeShortHostInfos(2)),
			common.NewBigInt(2),
		},
	}
	for i, test := range tests {
		res := getAverage(test.infos)
		if res.Cmp(test.expect) != 0 {
			t.Errorf("Test %d: got %v, expect %v", i, res, test.expect)
		}
	}
}

// makeHostInfos return a list of hostInfo. The list have 5 elements and one abnormally high
// value, one abnormally low value. The expected result of the getAveragePrice is 2.
// If the value of floorRatio and ceilRatio are to be changed, the values here might need to
// be changed
func makeHostInfos() []*storage.HostInfo {
	return []*storage.HostInfo{
		{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          common.NewBigInt(100000),
				StoragePrice:           common.NewBigInt(100000),
				UploadBandwidthPrice:   common.NewBigInt(100000),
				DownloadBandwidthPrice: common.NewBigInt(100000),
				Deposit:                common.NewBigInt(100000),
				MaxDeposit:             common.NewBigInt(100000),
			},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          common.NewBigInt(-200000),
				StoragePrice:           common.NewBigInt(-200000),
				UploadBandwidthPrice:   common.NewBigInt(-200000),
				DownloadBandwidthPrice: common.NewBigInt(-200000),
				Deposit:                common.NewBigInt(-200000),
				MaxDeposit:             common.NewBigInt(-200000),
			},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          common.NewBigInt(1),
				StoragePrice:           common.NewBigInt(1),
				UploadBandwidthPrice:   common.NewBigInt(1),
				DownloadBandwidthPrice: common.NewBigInt(1),
				Deposit:                common.NewBigInt(1),
				MaxDeposit:             common.NewBigInt(1),
			},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          common.NewBigInt(2),
				StoragePrice:           common.NewBigInt(2),
				UploadBandwidthPrice:   common.NewBigInt(2),
				DownloadBandwidthPrice: common.NewBigInt(2),
				Deposit:                common.NewBigInt(2),
				MaxDeposit:             common.NewBigInt(2),
			},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          common.NewBigInt(3),
				StoragePrice:           common.NewBigInt(3),
				UploadBandwidthPrice:   common.NewBigInt(3),
				DownloadBandwidthPrice: common.NewBigInt(3),
				Deposit:                common.NewBigInt(3),
				MaxDeposit:             common.NewBigInt(3),
			},
		},
	}
}

// makeShortHostInfos makes a list of HostInfo. Note the returned value all points to the same
// HostInfo
func makeShortHostInfos(size int) []*storage.HostInfo {
	info := &storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			ContractPrice:          common.NewBigInt(2),
			StoragePrice:           common.NewBigInt(2),
			UploadBandwidthPrice:   common.NewBigInt(2),
			DownloadBandwidthPrice: common.NewBigInt(2),
			Deposit:                common.NewBigInt(2),
			MaxDeposit:             common.NewBigInt(2),
		},
	}
	res := make([]*storage.HostInfo, size)
	for i := 0; i != size; i++ {
		res[i] = info
	}
	return res
}
