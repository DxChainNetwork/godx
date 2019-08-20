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
	blockNumber   uint64
	contractPrice common.BigInt
	storagePrice  common.BigInt
	uploadPrice   common.BigInt
	downloadPrice common.BigInt
	deposit       common.BigInt
	maxDeposit    common.BigInt
}

// GetMarketPrice return the price for the fake host manager
func (hm *fakeHostMarket) GetMarketPrice() MarketPrice {
	return MarketPrice{
		ContractPrice: hm.contractPrice,
		StoragePrice:  hm.storagePrice,
		UploadPrice:   hm.uploadPrice,
		DownloadPrice: hm.downloadPrice,
		Deposit:       hm.deposit,
		MaxDeposit:    hm.maxDeposit,
	}
}

// getFakeMarketPrice return a prototype of a fakeHostMarket for testing.
// The maxDeposit is capped at 1 sector to be stored in one day based on the deposit.
func getFakeHostMarket() *fakeHostMarket {
	return &fakeHostMarket{
		contractPrice: common.NewBigIntUint64(100),
		storagePrice:  common.NewBigIntUint64(100),
		uploadPrice:   common.NewBigIntUint64(100),
		downloadPrice: common.NewBigIntUint64(100),
		deposit:       common.NewBigIntUint64(100),
		maxDeposit:    common.NewBigIntUint64(storage.SectorSize * 3 * unit.BlocksPerDay * 100),
	}
}

// GetBlockNumber return the block number of the fake host market
func (hm *fakeHostMarket) GetBlockNumber() uint64 {
	return hm.blockNumber
}
