// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
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
		ContractPrice: common.PtrBigInt(new(big.Int).Mul(math.BigPow(10, 15), big.NewInt(50))),
		StoragePrice:  common.PtrBigInt(math.BigPow(10, 3)),
		UploadPrice:   common.PtrBigInt(math.BigPow(10, 7)),
		DownloadPrice: common.PtrBigInt(math.BigPow(10, 8)),
		Deposit:       common.PtrBigInt(math.BigPow(10, 3)),
		MaxDeposit:    common.PtrBigInt(math.BigPow(10, 20)),
	}
}
