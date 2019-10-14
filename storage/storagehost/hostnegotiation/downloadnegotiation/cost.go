// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiation

import (
	"math/bits"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// calcExpectedDownloadCost calculates the expected download cost, which is the sum of
// downloadBandwidthCost, downloadSectorAccessCost, and baseRPCPrice
func calcExpectedDownloadCost(np hostnegotiation.Protocol, dataSector storage.DownloadRequestSector) common.BigInt {
	// 1. get storage host config
	hostConfig := np.GetHostConfig()

	// 2. calculate download bandwidth cost and sector access cost
	bandwidthCost := calcDownloadBandwidthCost(hostConfig.DownloadBandwidthPrice, uint64(dataSector.Length))
	sectorAccessCost := calcDownloadSectorAccessCost(hostConfig.SectorAccessPrice, dataSector.MerkleRoot)

	// 3. calculate the expected download cost
	downloadCost := hostConfig.BaseRPCPrice.Add(bandwidthCost).Add(sectorAccessCost)
	return downloadCost
}

// calcDownloadBandwidthCost calculates the download bandwidth cost
func calcDownloadBandwidthCost(downloadBandwidthPrice common.BigInt, sectorLength uint64) common.BigInt {
	hashesPerProof := 2 * bits.Len64(storage.SectorSize/merkle.LeafSize)
	bandwidth := sectorLength + uint64(hashesPerProof*storage.HashSize)
	bandwidthCost := downloadBandwidthPrice.MultUint64(bandwidth)
	return bandwidthCost
}

// calcDownloadSectorAccessCost calculates the download sector access cost
func calcDownloadSectorAccessCost(sectorAccessPrice common.BigInt, sectorRoot [32]byte) common.BigInt {
	sectorAccesses := make(map[common.Hash]struct{})
	sectorAccesses[sectorRoot] = struct{}{}
	sectorAccessCost := sectorAccessPrice.MultUint64(uint64(len(sectorAccesses)))
	return sectorAccessCost
}
