// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// HostMarket provides methods to evaluate the storage price, upload price, download
// price, and deposit price. Note the HostMarket should have a caching method.
type HostMarket interface {
	GetMarketPrice() storage.MarketPrice
	GetBlockNumber() uint64
}

// GetMarketPrice return the market price for evaluation based on host entries
// TODO: implement this
func (shm *StorageHostManager) GetMarketPrice() storage.MarketPrice {
	return storage.MarketPrice{
		ContractPrice: common.NewBigInt(1000),
		StoragePrice:  common.NewBigInt(1000),
		UploadPrice:   common.NewBigInt(1000),
		DownloadPrice: common.NewBigInt(1000),
		Deposit:       common.NewBigInt(1000),
		MaxDeposit:    common.NewBigInt(10000000),
	}
}

// GetBlockNumber get the current block number from storage host manager
func (shm *StorageHostManager) GetBlockNumber() uint64 {
	return shm.blockHeight
}
