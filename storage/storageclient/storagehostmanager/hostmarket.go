// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	// fields denotes the corresponding field in node info
	fieldContractPrice = iota
	fieldStoragePrice
	fieldUploadPrice
	fieldDownloadPrice
	fieldDeposit
	fieldMaxDeposit
)

// GetMarketPrice will return the market price. It will first try to get the value from
// cached prices. If cached prices need to be updated, prices are calculated and returned.
// Note that the function need to be protected by shm.lock for shm.initialScanFinished field.
func (shm *StorageHostManager) GetMarketPrice() storage.MarketPrice {
	// If the initial scan has not finished, return the default host market price
	// Since the shm has been locked when evaluating the host score, no lock is needed
	// here.
	if !shm.isInitialScanFinished() {
		return defaultMarketPrice
	}
	return shm.cachedPrices.getPrices()
}

// UpdateMarketPriceLoop is a infinite loop to update the market price. The input mutex is locked in
// the inital status. After the first market price is updated, the lock will be unlocked to allow
// scan to continue.
func (shm *StorageHostManager) updateMarketPriceLoop(mutex *sync.RWMutex) {
	// Add to thread manager. If error happens directly return and no error reported.
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	var once sync.Once

	// Forever loop to update the prices
	for {
		// calculate the prices and update
		prices := shm.calculateMarketPrice()
		shm.cachedPrices.updatePrices(prices)
		once.Do(mutex.Unlock)
		select {
		// Return when stopped, continue when interval passed
		case <-shm.tm.StopChan():
			return
		case <-time.After(priceUpdateInterval):
			continue
		}
	}
}

// calculateMarketPrice calculate and return the market price
func (shm *StorageHostManager) calculateMarketPrice() storage.MarketPrice {
	// get all host infos
	infos := shm.ActiveStorageHosts()
	// If there is no active hosts, return the default market price
	if len(infos) == 0 {
		return defaultMarketPrice
	}
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
	ptrs := make([]*storage.HostInfo, len(infos))
	// copy the pointer to the pointer list. The pointer value to be appended need
	// to be declared within the loop. More details please visit this blog:
	// https://medium.com/codezillas/uh-ohs-in-go-slice-of-pointers-c0a30669feee
	for i, info := range infos {
		infoCopy := info
		ptrs[i] = &infoCopy
	}
	return ptrs
}

// cachedPrices is the cache for pricing. The field is registered in storage host manager
// and not saved to persistence
type cachedPrices struct {
	prices storage.MarketPrice
	lock   sync.RWMutex
}

// updatePrices update the prices in cachedPrices
func (cp *cachedPrices) updatePrices(prices storage.MarketPrice) {
	cp.lock.Lock()
	defer cp.lock.Unlock()

	cp.prices = prices
}

// getPrices return the prices stored in cachedPrices
func (cp *cachedPrices) getPrices() storage.MarketPrice {
	cp.lock.RLock()
	defer cp.lock.RUnlock()

	return cp.prices
}

// hostInfoPriceSorter is the structure for sorting for host infos. The structure implements sort interface
type hostInfoPriceSorter struct {
	infos []*storage.HostInfo
	field int
}

// newInfoPriceSorter returns a new hostInfoPriceSorter with the given infos and field
func newInfoPriceSorter(infos []*storage.HostInfo, field int) *hostInfoPriceSorter {
	return &hostInfoPriceSorter{
		infos: infos,
		field: field,
	}
}

// getAverage get the average price for infoSorter. The strategy is to neglect the lowest prices and
// highest prices, and aiming to get the average price for the mid range.
func getAverage(infoSorter *hostInfoPriceSorter) common.BigInt {
	// Sort the info sorter
	sort.Sort(infoSorter)
	length := infoSorter.Len()
	// Calculate the range we want to calculate for average
	lowIndex := int(math.Floor(float64(length) * floorRatio))
	highIndex := length - int(math.Floor(float64(length)*ceilRatio))
	if highIndex == lowIndex {
		return getMarketPriceByField(defaultMarketPrice, infoSorter.field)
	}
	// Calculate average
	sum := common.BigInt0
	for i := lowIndex; i != highIndex; i++ {
		sum = sum.Add(infoSorter.getPrice(i))
	}
	average := sum.DivUint64(uint64(highIndex - lowIndex))
	return average
}

// Len return the size of the list
func (infoSorter *hostInfoPriceSorter) Len() int {
	return len(infoSorter.infos)
}

// Swap swap two elements specified by index
func (infoSorter *hostInfoPriceSorter) Swap(i, j int) {
	infoSorter.infos[i], infoSorter.infos[j] = infoSorter.infos[j], infoSorter.infos[i]
}

// Less compare the two items of index i and j to determine whose related price is smaller
func (infoSorter *hostInfoPriceSorter) Less(i, j int) bool {
	p1 := infoSorter.getPrice(i)
	p2 := infoSorter.getPrice(j)
	return p1.Cmp(p2) < 0
}

// getPrice get the price of specified field of specified indexed item. Notice the index is assumed to be within
// the range of [0, len(infoSorter.infos] - 1]
func (infoSorter *hostInfoPriceSorter) getPrice(index int) common.BigInt {
	return getInfoPriceByField(infoSorter.infos[index], infoSorter.field)
}

// getAverageContractPrice get the average contract price from host infos
func getAverageContractPrice(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldContractPrice)
	return getAverage(sorter)
}

// getAverageStoragePrice get the average storage price from host infos
func getAverageStoragePrice(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldStoragePrice)
	return getAverage(sorter)
}

// getAverageUploadPrice get the average upload price from host infos
func getAverageUploadPrice(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldUploadPrice)
	return getAverage(sorter)
}

// getAverageDownloadPrice get the average download price from host infos
func getAverageDownloadPrice(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldDownloadPrice)
	return getAverage(sorter)
}

// getAverageDeposit get the average deposit from host infos
func getAverageDeposit(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldDeposit)
	return getAverage(sorter)
}

// getAverageMaxDeposit get the average max deposit from host infos
func getAverageMaxDeposit(infos []*storage.HostInfo) common.BigInt {
	sorter := newInfoPriceSorter(infos, fieldMaxDeposit)
	return getAverage(sorter)
}

// getInfoPriceByField get the price specified by field of a host info
func getInfoPriceByField(info *storage.HostInfo, field int) common.BigInt {
	switch field {
	case fieldContractPrice:
		return info.ContractPrice
	case fieldStoragePrice:
		return info.StoragePrice
	case fieldUploadPrice:
		return info.UploadBandwidthPrice
	case fieldDownloadPrice:
		return info.DownloadBandwidthPrice
	case fieldDeposit:
		return info.Deposit
	case fieldMaxDeposit:
		return info.MaxDeposit
	default:
	}
	return common.BigInt0
}

// getMarketPriceByField get the price specified by field of a storage market prices
func getMarketPriceByField(prices storage.MarketPrice, field int) common.BigInt {
	switch field {
	case fieldContractPrice:
		return prices.ContractPrice
	case fieldStoragePrice:
		return prices.StoragePrice
	case fieldUploadPrice:
		return prices.UploadPrice
	case fieldDownloadPrice:
		return prices.DownloadPrice
	case fieldDeposit:
		return prices.Deposit
	case fieldMaxDeposit:
		return prices.MaxDeposit
	default:
	}
	return common.BigInt0
}
