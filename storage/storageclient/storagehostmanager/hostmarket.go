// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// hostMarket provides methods to evaluate the storage price, upload price, download
// price, and deposit price. Note the hostMarket should have a caching method.
type hostMarket interface {
	GetMarketPrice() storage.MarketPrice
	getBlockHeight() uint64
}

// GetMarketPrice will return the market price. It will first try to get the value from
// cached prices. If cached prices need to be updated, prices are calculated and returned.
func (shm *StorageHostManager) GetMarketPrice() storage.MarketPrice {
	if shm.cachedPrices.isUpdateNeeded() {

		// prices need to be calculated and applied to cachedPrices
		prices := shm.calculateMarketPrice()
		shm.cachedPrices.updatePrices(prices)
	}
	return shm.cachedPrices.getPrices()
}

// calculateMarketPrice calculate and return the market price
func (shm *StorageHostManager) calculateMarketPrice() storage.MarketPrice {
	// get all host infos
	infos := shm.ActiveStorageHosts()
	// change the list of hostInfo to the list of hostInfo pointers
	ptrInfos := hostInfoListToPtrList(infos)
	// calculate the market price and return
	return storage.MarketPrice{
		ContractPrice: getAverageContractPrice(ptrInfos),
		StoragePrice:  getAverageStoragePrice(ptrInfos),
		UploadPrice:   getAverageUploadPrice(ptrInfos),
		DownloadPrice: getAverageDownloadPrice(ptrInfos),
		Deposit:       getAverageDeposit(ptrInfos),
		MaxDeposit:    getAverageMaxDeposit(ptrInfos),
	}
}

// hostInfoListToPtrList change a list of hostInfo to a list of hostInfo pointers
func hostInfoListToPtrList(infos []storage.HostInfo) []*storage.HostInfo {
	var ptrs []*storage.HostInfo
	for _, info := range infos {
		ptrs = append(ptrs, &info)
	}
	return ptrs
}

// cachedPrices is the cache for pricing. The field is registered in storage host manager
// and not saved to persistence
type cachedPrices struct {
	prices         storage.MarketPrice
	timeLastUpdate time.Time
	lock           sync.RWMutex
}

// isUpdateNeeded returns whether a cachedPrices update is needed.
// The prices should be updated when
// 1. prices are zero
// 2. last update is before priceUpdateInterval
func (cp *cachedPrices) isUpdateNeeded() bool {
	cp.lock.RLock()
	defer cp.lock.RUnlock()

	if reflect.DeepEqual(cp.prices, storage.MarketPrice{}) {
		return true
	}
	if time.Since(cp.timeLastUpdate) > priceUpdateInterval {
		return true
	}
	return false
}

// updatePrices update the prices in cachedPrices
func (cp *cachedPrices) updatePrices(prices storage.MarketPrice) {
	cp.lock.Lock()
	defer cp.lock.Unlock()

	cp.prices = prices
	cp.timeLastUpdate = time.Now()
}

// getPrices return the prices stored in cachedPrices
func (cp *cachedPrices) getPrices() storage.MarketPrice {
	cp.lock.RLock()
	defer cp.lock.RUnlock()

	return cp.prices
}

// priceGetterSorter is the interface that could get a specified price in the indexed item
// and implement sort.Interface to enable sorting
type priceGetterSorter interface {
	sort.Interface
	getPrice(index int) common.BigInt
}

// getAverage get the average price for hostInfos. The strategy is to neglect the lowest prices and
// highest prices, and aiming to get the average price for the mid range.
func getAverage(hostInfos priceGetterSorter) common.BigInt {
	sort.Sort(hostInfos)
	length := hostInfos.Len()
	lowIndex := int(math.Floor(float64(length) * floorRatio))
	highIndex := length - int(math.Floor(float64(hostInfos.Len())*ceilRatio))

	sum := common.BigInt0
	for i := lowIndex; i != highIndex; i++ {
		sum = sum.Add(hostInfos.getPrice(i))
	}

	average := sum.DivUint64(uint64(highIndex - lowIndex))
	return average
}

// hostInfosByContractPrice is the list of hostInfo which implements priceGetterSorter for
// contractPrice
type hostInfosByContractPrice []*storage.HostInfo

func (infos hostInfosByContractPrice) getPrice(i int) common.BigInt { return infos[i].ContractPrice }
func (infos hostInfosByContractPrice) Len() int                     { return len(infos) }
func (infos hostInfosByContractPrice) Swap(i, j int)                { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByContractPrice) Less(i, j int) bool {
	return infos[i].ContractPrice.Cmp(infos[j].ContractPrice) < 0
}

// getAverageContractPrice get the contract price from the host infos
func getAverageContractPrice(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByContractPrice(infos)
	return getAverage(infosForSort)
}

// hostInfosByStoragePrice is the list of hostInfo which implements priceGetterSorter for
// storagePrice
type hostInfosByStoragePrice []*storage.HostInfo

func (infos hostInfosByStoragePrice) getPrice(i int) common.BigInt { return infos[i].StoragePrice }
func (infos hostInfosByStoragePrice) Len() int                     { return len(infos) }
func (infos hostInfosByStoragePrice) Swap(i, j int)                { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByStoragePrice) Less(i, j int) bool {
	return infos[i].StoragePrice.Cmp(infos[j].StoragePrice) < 0
}

// getAverageStoragePrice get the average storage price from the host infos
func getAverageStoragePrice(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByStoragePrice(infos)
	return getAverage(infosForSort)
}

// hostInfosByUploadPrice is the list of hostInfo which implements priceGetterSorter for
// uploadBandwidthPrice
type hostInfosByUploadPrice []*storage.HostInfo

func (infos hostInfosByUploadPrice) getPrice(i int) common.BigInt {
	return infos[i].UploadBandwidthPrice
}
func (infos hostInfosByUploadPrice) Len() int      { return len(infos) }
func (infos hostInfosByUploadPrice) Swap(i, j int) { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByUploadPrice) Less(i, j int) bool {
	return infos[i].UploadBandwidthPrice.Cmp(infos[j].UploadBandwidthPrice) < 0
}

// getAverageUploadPrice return the average upload price from the host infos
func getAverageUploadPrice(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByUploadPrice(infos)
	return getAverage(infosForSort)
}

// hostInfosByDownloadPrice is the list of hostInfo which implements priceGetterSorter for
// downloadBandwidthPrice
type hostInfosByDownloadPrice []*storage.HostInfo

func (infos hostInfosByDownloadPrice) getPrice(i int) common.BigInt {
	return infos[i].DownloadBandwidthPrice
}
func (infos hostInfosByDownloadPrice) Len() int      { return len(infos) }
func (infos hostInfosByDownloadPrice) Swap(i, j int) { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByDownloadPrice) Less(i, j int) bool {
	return infos[i].DownloadBandwidthPrice.Cmp(infos[j].DownloadBandwidthPrice) < 0
}

// getAverageDownloadPrice return the average download price from the host infos
func getAverageDownloadPrice(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByDownloadPrice(infos)
	return getAverage(infosForSort)
}

// hostInfosByDeposit is the list of hostInfo which implements priceGetterSorter for
// Deposit
type hostInfosByDeposit []*storage.HostInfo

func (infos hostInfosByDeposit) getPrice(i int) common.BigInt { return infos[i].Deposit }
func (infos hostInfosByDeposit) Len() int                     { return len(infos) }
func (infos hostInfosByDeposit) Swap(i, j int)                { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByDeposit) Less(i, j int) bool {
	return infos[i].Deposit.Cmp(infos[j].Deposit) < 0
}

// getAverageDeposit return the average deposit from the host infos
func getAverageDeposit(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByDeposit(infos)
	return getAverage(infosForSort)
}

// hostInfosByMaxDeposit is the list of hostInfo which implements priceGetterSorter for
// MaxDeposit
type hostInfosByMaxDeposit []*storage.HostInfo

func (infos hostInfosByMaxDeposit) getPrice(i int) common.BigInt { return infos[i].MaxDeposit }
func (infos hostInfosByMaxDeposit) Len() int                     { return len(infos) }
func (infos hostInfosByMaxDeposit) Swap(i, j int)                { infos[i], infos[j] = infos[j], infos[i] }
func (infos hostInfosByMaxDeposit) Less(i, j int) bool {
	return infos[i].StoragePrice.Cmp(infos[j].MaxDeposit) < 0
}

func getAverageMaxDeposit(infos []*storage.HostInfo) common.BigInt {
	infosForSort := hostInfosByMaxDeposit(infos)
	return getAverage(infosForSort)
}
