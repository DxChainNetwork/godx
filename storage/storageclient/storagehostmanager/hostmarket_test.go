// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// fakeHostMarket is a fake host market that implement HostMarket
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
