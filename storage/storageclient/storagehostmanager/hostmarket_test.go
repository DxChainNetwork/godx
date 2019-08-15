// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
)

// fakeHostMarket is a fake host market that implement hostMarket
type fakeHostMarket struct {
	ContractPrice common.BigInt
	StoragePrice  common.BigInt
	UploadPrice   common.BigInt
	DownloadPrice common.BigInt
	Deposit       common.BigInt
	MaxDeposit    common.BigInt
}

// GetMarketPrice return the price for the fake host manager
func (hm *fakeHostMarket) GetMarketPrice() MarketPrice {
	return MarketPrice{
		ContractPrice: common.BigInt0,
		StoragePrice:  common.BigInt0,
		UploadPrice:   common.BigInt0,
		DownloadPrice: common.BigInt0,
		Deposit:       common.BigInt0,
		MaxDeposit:    common.BigInt0,
	}
}

// getFakeMarketPrice return a prototype of a fakeHostMarket for testing.
// The maxDeposit is capped at 1 sector to be stored in one day based on the deposit.
func getFakeMarketPrice() *fakeHostMarket {
	return &fakeHostMarket{
		ContractPrice: common.NewBigIntUint64(100),
		StoragePrice:  common.NewBigIntUint64(100),
		UploadPrice:   common.NewBigIntUint64(100),
		DownloadPrice: common.NewBigIntUint64(100),
		Deposit:       common.NewBigIntUint64(100),
		MaxDeposit:    common.NewBigIntUint64(storage.SectorSize * 3 * unit.BlocksPerDay * 100),
	}
}
