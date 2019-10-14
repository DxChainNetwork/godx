// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

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

func calcBandwidthRevenueForProof(nd *uploadNegotiationData, subTreeHashesLen, leafHashesLen int, downloadBandwidthPrice common.BigInt) common.BigInt {
	// calculate the merkle proof size
	merkleProofSize := storage.HashSize * (subTreeHashesLen + leafHashesLen + 1)
	nd.bandwidthRevenue = nd.bandwidthRevenue.Add(downloadBandwidthPrice.Mult(common.NewBigInt(int64(merkleProofSize))))
	return nd.bandwidthRevenue
}
