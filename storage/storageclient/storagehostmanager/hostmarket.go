// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
)

// hostMarket provides methods to evaluate the storage price, upload price, download
// price, and deposit price. Note the hostMarket should have a caching method.
type hostMarket interface {
	GetMarketPrice() MarketPrice
	GetBlockNumber() uint64
}

// MarketPrice is the market price metrics from hostMarket
type MarketPrice struct {
	ContractPrice common.BigInt
	StoragePrice  common.BigInt
	UploadPrice   common.BigInt
	DownloadPrice common.BigInt
	Deposit       common.BigInt
	MaxDeposit    common.BigInt
}

// GetMarketPrice return the market price for evaluation based on host entries
// TODO: implement this
func (shm *StorageHostManager) GetMarketPrice() MarketPrice {
	return MarketPrice{
		ContractPrice: common.BigInt0,
		StoragePrice:  common.BigInt0,
		UploadPrice:   common.BigInt0,
		DownloadPrice: common.BigInt0,
		Deposit:       common.BigInt0,
		MaxDeposit:    common.BigInt0,
	}
}

// GetBlockNumber get the current block number from storage host manager
func (shm *StorageHostManager) GetBlockNumber() uint64 {
	return shm.blockHeight
}
