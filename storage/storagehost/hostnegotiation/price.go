// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func renewBasePrice(sr storagehost.StorageResponsibility, storagePrice common.BigInt, windowEnd, fileSize uint64) common.BigInt {
	// calculate the timeExtension and the base cost for contract renew
	timeExtension := windowEnd - sr.ProofDeadline()
	if timeExtension <= 0 {
		return common.BigInt0
	}

	// calculate the renew base price
	return storagePrice.MultUint64(fileSize).MultUint64(timeExtension)
}

func renewBaseDeposit(sr storagehost.StorageResponsibility, windowEnd, fileSize uint64, deposit common.BigInt) common.BigInt {
	// calculate the timeExtension and the base deposit cost for contract renew
	timeExtension := windowEnd - sr.ProofDeadline()
	if timeExtension <= 0 {
		return common.BigInt0
	}

	// calculate the contract renew deposit
	return deposit.MultUint64(fileSize).MultUint64(timeExtension)
}

func calcStorageRevenueAndNewDeposit(nd *uploadNegotiationData, sr storagehost.StorageResponsibility, blockHeight uint64, storagePrice, deposit common.BigInt) {
	if len(nd.newRoots) > len(sr.SectorRoots) {
		dataAdded := storage.SectorSize * uint64(len(nd.newRoots)-len(sr.SectorRoots))
		blocksRemaining := sr.ProofDeadline() - blockHeight
		dataBlocks := common.NewBigIntUint64(blocksRemaining).Mult(common.NewBigIntUint64(dataAdded))
		nd.storageRevenue = dataBlocks.Mult(storagePrice)
		nd.newDeposit = dataBlocks.Mult(deposit)
	}
}

func calcHostRevenue(nd *uploadNegotiationData, sr storagehost.StorageResponsibility, blockHeight uint64, hostConfig storage.HostIntConfig) common.BigInt {
	calcStorageRevenueAndNewDeposit(nd, sr, blockHeight, hostConfig.StoragePrice, hostConfig.Deposit)
	return nd.storageRevenue.Add(nd.bandwidthRevenue).Add(hostConfig.BaseRPCPrice)
}
