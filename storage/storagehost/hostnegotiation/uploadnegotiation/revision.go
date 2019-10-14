// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// updateRevisionFileSize will update the new contract revision's file size based on the
// type of the upload action
func updateRevisionFileSize(newRev *types.StorageContractRevision, uploadReq storage.UploadRequest) {
	for _, action := range uploadReq.Actions {
		switch action.Type {
		case storage.UploadActionAppend:
			newRev.NewFileSize += storage.SectorSize
		}
	}
}

// calcAndUpdateRevisionMerkleRoot will calculate the new file merkle root for storage contract revision
// and update both new revision and uploadNegotiationData
func calcAndUpdateRevisionMerkleRoot(session *hostnegotiation.UploadSession, newRev *types.StorageContractRevision) {
	// calculate the new merkle roots and update the new revision
	session.NewMerkleRoot = merkle.Sha256CachedTreeRoot2(session.SectorRoots)
	newRev.NewFileMerkleRoot = session.NewMerkleRoot
}

// updateRevisionMissedAndValidPayback will update the new contract revision missed and valid
// proof payback
func updateRevisionMissedAndValidPayback(newRev *types.StorageContractRevision, oldRev types.StorageContractRevision, uploadReq storage.UploadRequest) {
	// update the revision valid proof outputs
	for i := range oldRev.NewValidProofOutputs {
		validProofOutput := types.DxcoinCharge{
			Value:   uploadReq.NewValidProofValues[i],
			Address: oldRev.NewValidProofOutputs[i].Address,
		}
		newRev.NewValidProofOutputs = append(newRev.NewValidProofOutputs, validProofOutput)
	}

	// update the revision missed proof outputs
	for i := range oldRev.NewValidProofOutputs {
		missedProofOutput := types.DxcoinCharge{
			Value:   uploadReq.NewMissedProofValues[i],
			Address: oldRev.NewMissedProofOutputs[i].Address,
		}
		newRev.NewMissedProofOutputs = append(newRev.NewMissedProofOutputs, missedProofOutput)
	}
}
