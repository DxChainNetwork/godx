// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// handleUploadAppendType will handle the upload action with the type UploadAppendAction
// by handing it, a bunch of data will be calculated and recorded in the uploadNegotiationData
func handleUploadAppendType(action storage.UploadAction, session *hostnegotiation.UploadSession, uploadBandwidthPrice common.BigInt) {
	// update upload negotiation data
	newRoot := merkle.Sha256MerkleTreeRoot(action.Data)
	session.NewRoots = append(session.NewRoots, newRoot)
	session.SectorGained = append(session.SectorGained, newRoot)
	session.GainedSectorData = append(session.GainedSectorData, action.Data)
	session.SectorsChanged[uint64(len(session.NewRoots)-1)] = struct{}{}
	session.BandwidthRevenue = session.BandwidthRevenue.Add(uploadBandwidthPrice.MultUint64(storage.SectorSize))
}
