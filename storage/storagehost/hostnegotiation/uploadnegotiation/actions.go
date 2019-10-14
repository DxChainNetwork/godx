// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// handleAppendAction will handle the upload action with the type UploadAppendAction.
func handleAppendAction(action storage.UploadAction, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, uploadBandwidthPrice common.BigInt) {
	// data initialization
	session.SectorRoots = append(session.SectorRoots, sr.SectorRoots...)
	session.SectorsCount = make(map[uint64]struct{})

	// update the sector information and the bandwidthRevenue
	newRoot := merkle.Sha256MerkleTreeRoot(action.Data)
	session.SectorRoots = append(session.SectorRoots, newRoot)
	session.SectorRootsGained = append(session.SectorRootsGained, newRoot)
	session.SectorDataGained = append(session.SectorDataGained, action.Data)
	session.SectorsCount[uint64(len(session.SectorRoots)-1)] = struct{}{}
	session.BandwidthRevenue = session.BandwidthRevenue.Add(uploadBandwidthPrice.MultUint64(storage.SectorSize))
}
