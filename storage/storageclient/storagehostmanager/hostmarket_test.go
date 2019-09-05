// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"

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

// fakeHostTree is the fake implementation of StorageHostTree for testing purpose.
// Currently, only the All method is used for testing. Add more functionality as
// needed.
type fakeHostTree struct {
	infos []storage.HostInfo
}

func (t *fakeHostTree) Insert(hi storage.HostInfo, eval int64) error         { return nil }
func (t *fakeHostTree) HostInfoUpdate(hi storage.HostInfo, eval int64) error { return nil }
func (t *fakeHostTree) Remove(enodeID enode.ID) error                        { return nil }
func (t *fakeHostTree) RetrieveHostInfo(enodeID enode.ID) (storage.HostInfo, int64, bool) {
	return storage.HostInfo{}, 0, false
}
func (t *fakeHostTree) SelectRandom(needed int, blacklist, addrBlacklist []enode.ID) []storage.HostInfo {
	return []storage.HostInfo{}
}
func (t *fakeHostTree) All() []storage.HostInfo { return t.infos }

// newFakeHostTree returns a new fake host tree with the give host infos
func newFakeHostTree(infos []storage.HostInfo) *fakeHostTree {
	return &fakeHostTree{infos}
}

// newStorageHostForHostMarketTest returns a storage host manager for testing for host market
func newStorageHostForHostMarketTest(initialScanFinished bool, prices cachedPrices, tree storagehosttree.StorageHostTree) *StorageHostManager {
	shm := &StorageHostManager{
		storageHostTree: tree,
		cachedPrices:    prices,
	}
	if initialScanFinished {
		shm.finishInitialScan()
	}
	return shm
}

// TestStorageHostManager_GetMarketPrice test the functionality of StorageHostManager.GetMarketPrice
func TestStorageHostManager_GetMarketPrice(t *testing.T) {
	tests := []struct {
		initialScanFinished bool
		tree                storagehosttree.StorageHostTree
		cachedPrices        cachedPrices
		expectedPrice       storage.MarketPrice
	}{
		{
			initialScanFinished: false,
			tree:                newFakeHostTree([]storage.HostInfo{}),
			expectedPrice:       defaultMarketPrice,
		},
		{
			// Need update
			initialScanFinished: true,
			tree:                newFakeHostTree(makeHostInfos()),
			cachedPrices: cachedPrices{
				prices:         storage.MarketPrice{},
				timeLastUpdate: time.Now().AddDate(-1, 0, 0),
			},
			expectedPrice: storage.MarketPrice{
				ContractPrice: common.NewBigInt(2),
				StoragePrice:  common.NewBigInt(2),
				UploadPrice:   common.NewBigInt(2),
				DownloadPrice: common.NewBigInt(2),
				Deposit:       common.NewBigInt(2),
				MaxDeposit:    common.NewBigInt(2),
			},
		},
		{
			// No need update
			initialScanFinished: true,
			tree:                newFakeHostTree([]storage.HostInfo{}),
			cachedPrices: cachedPrices{
				prices: storage.MarketPrice{
					ContractPrice: common.NewBigInt(2),
					StoragePrice:  common.NewBigInt(2),
					UploadPrice:   common.NewBigInt(2),
					DownloadPrice: common.NewBigInt(2),
					Deposit:       common.NewBigInt(2),
					MaxDeposit:    common.NewBigInt(2),
				},
				timeLastUpdate: time.Now(),
			},
			expectedPrice: storage.MarketPrice{
				ContractPrice: common.NewBigInt(2),
				StoragePrice:  common.NewBigInt(2),
				UploadPrice:   common.NewBigInt(2),
				DownloadPrice: common.NewBigInt(2),
				Deposit:       common.NewBigInt(2),
				MaxDeposit:    common.NewBigInt(2),
			},
		},
	}
	for i, test := range tests {
		shm := newStorageHostForHostMarketTest(test.initialScanFinished, test.cachedPrices, test.tree)
		marketPrice := shm.GetMarketPrice()
		if !reflect.DeepEqual(marketPrice, test.expectedPrice) {
			t.Errorf("Test %d: \n\tGot %+v\n\tExpect %+v", i, marketPrice, test.expectedPrice)
		}
	}
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
			hostInfosByContractPrice(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByContractPrice(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByContractPrice(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByContractPrice(hostInfoListToPtrList(makeShortHostInfos(2))),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByStoragePrice(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByStoragePrice(hostInfoListToPtrList(makeShortHostInfos(2))),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByUploadPrice(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByUploadPrice(hostInfoListToPtrList(makeShortHostInfos(2))),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByDownloadPrice(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByDownloadPrice(hostInfoListToPtrList(makeShortHostInfos(2))),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByDeposit(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByDeposit(hostInfoListToPtrList(makeShortHostInfos(2))),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(hostInfoListToPtrList(makeHostInfos())),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(hostInfoListToPtrList(makeShortHostInfos(0))),
			common.NewBigInt(0),
		},
		{
			hostInfosByMaxDeposit(hostInfoListToPtrList(makeShortHostInfos(1))),
			common.NewBigInt(2),
		},
		{
			hostInfosByMaxDeposit(hostInfoListToPtrList(makeShortHostInfos(2))),
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
func makeHostInfos() []storage.HostInfo {
	return []storage.HostInfo{
		{
			HostExtConfig: storage.HostExtConfig{
				AcceptingContracts:     true,
				ContractPrice:          common.NewBigInt(100000),
				StoragePrice:           common.NewBigInt(100000),
				UploadBandwidthPrice:   common.NewBigInt(100000),
				DownloadBandwidthPrice: common.NewBigInt(100000),
				Deposit:                common.NewBigInt(100000),
				MaxDeposit:             common.NewBigInt(100000),
			},
			ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			}},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				AcceptingContracts:     true,
				ContractPrice:          common.NewBigInt(-200000),
				StoragePrice:           common.NewBigInt(-200000),
				UploadBandwidthPrice:   common.NewBigInt(-200000),
				DownloadBandwidthPrice: common.NewBigInt(-200000),
				Deposit:                common.NewBigInt(-200000),
				MaxDeposit:             common.NewBigInt(-200000),
			},
			ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			}},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				AcceptingContracts:     true,
				ContractPrice:          common.NewBigInt(1),
				StoragePrice:           common.NewBigInt(1),
				UploadBandwidthPrice:   common.NewBigInt(1),
				DownloadBandwidthPrice: common.NewBigInt(1),
				Deposit:                common.NewBigInt(1),
				MaxDeposit:             common.NewBigInt(1),
			},
			ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			}},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				AcceptingContracts:     true,
				ContractPrice:          common.NewBigInt(2),
				StoragePrice:           common.NewBigInt(2),
				UploadBandwidthPrice:   common.NewBigInt(2),
				DownloadBandwidthPrice: common.NewBigInt(2),
				Deposit:                common.NewBigInt(2),
				MaxDeposit:             common.NewBigInt(2),
			},
			ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			}},
		},
		{
			HostExtConfig: storage.HostExtConfig{
				AcceptingContracts:     true,
				ContractPrice:          common.NewBigInt(3),
				StoragePrice:           common.NewBigInt(3),
				UploadBandwidthPrice:   common.NewBigInt(3),
				DownloadBandwidthPrice: common.NewBigInt(3),
				Deposit:                common.NewBigInt(3),
				MaxDeposit:             common.NewBigInt(3),
			},
			ScanRecords: storage.HostPoolScans{storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			}},
		},
	}
}

// makeShortHostInfos makes a list of HostInfo. Note the returned value all points to the same
// HostInfo
func makeShortHostInfos(size int) []storage.HostInfo {
	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			ContractPrice:          common.NewBigInt(2),
			StoragePrice:           common.NewBigInt(2),
			UploadBandwidthPrice:   common.NewBigInt(2),
			DownloadBandwidthPrice: common.NewBigInt(2),
			Deposit:                common.NewBigInt(2),
			MaxDeposit:             common.NewBigInt(2),
		},
	}
	res := make([]storage.HostInfo, size)
	for i := 0; i != size; i++ {
		res[i] = info
	}
	return res
}
