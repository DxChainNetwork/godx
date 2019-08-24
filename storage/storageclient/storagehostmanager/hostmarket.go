// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
)

// hostMarket provides methods to evaluate the storage price, upload price, download
// price, and deposit price. Note the hostMarket should have a caching method.
type hostMarket interface {
	getMarketPrice() MarketPrice
	getBlockHeight() uint64
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

// getMarketPrice return the market price for evaluation based on host entries.
// Currently, the returned market price is hard coded host default settings.
// Will be updated later.
// TODO: implement this
func (shm *StorageHostManager) getMarketPrice() MarketPrice {
	return MarketPrice{
		ContractPrice: common.PtrBigInt(new(big.Int).Mul(math.BigPow(10, 15), big.NewInt(50))),
		StoragePrice:  common.PtrBigInt(math.BigPow(10, 3)),
		UploadPrice:   common.PtrBigInt(math.BigPow(10, 7)),
		DownloadPrice: common.PtrBigInt(math.BigPow(10, 8)),
		Deposit:       common.PtrBigInt(math.BigPow(10, 3)),
		MaxDeposit:    common.PtrBigInt(math.BigPow(10, 20)),
	}
}
