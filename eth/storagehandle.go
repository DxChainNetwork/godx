// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"github.com/DxChainNetwork/godx/p2p"
)

func (pm *ProtocolManager) uploadHandler(p *peer, uploadReqMsg p2p.Msg) {
	//// decode the upload request message
	//var uploadRequest storage.UploadRequest
	//if err := uploadReqMsg.Decode(&uploadRequest); err != nil {
	//	err = fmt.Errorf("error decoding the upload request message: %s", err.Error())
	//	log.Error("upload failed, disconnecting ...", "err", err.Error())
	//	p.TriggerError(err)
	//	return
	//}
	//
	//// get the storage contract revision from the storage responsibility
	//sr, err := pm.eth.storageHost.GetStorageResponsibility(uploadRequest.StorageContractID)
	//if err != nil {
	//	// no error return, it is normal if the storage responsibility cannot be retrieved
	//	// return directly but there is no need to disconnect
	//	return
	//}
	//
	//// gather all the information needed
	//hostConfig := pm.eth.storageHost.RetrieveExternalConfig()
	//currentBlockHeight := pm.eth.storageHost.GetCurrentBlockHeight()
	//latestRevision := sr.StorageContractRevisions[len(sr.StorageContractRevisions)-1]
	//
	//// calculate roots, revenue, and etc. for each action
	//newRoots := append([]common.Hash(nil), sr.SectorRoots...)
	//sectorsChanged := make(map[uint64]struct{})
	//
	//var bandwidthRevenue common.BigInt
	//var sectorsRemoved []common.Hash
	//var sectorsGained []common.Hash
	//var gainedSectorData [][]byte
	//
	//for _, action := range uploadRequest.Actions {
	//	switch action.Type {
	//	case storage.UploadActionAppend:
	//		// Update sector roots.
	//		newRoot := merkle.Sha256MerkleTreeRoot(action.Data)
	//		newRoots = append(newRoots, newRoot)
	//		sectorsGained = append(sectorsGained, newRoot)
	//		gainedSectorData = append(gainedSectorData, action.Data)
	//
	//		sectorsChanged[uint64(len(newRoots))-1] = struct{}{}
	//
	//		// Update finances
	//		bandwidthRevenue = bandwidthRevenue.Add(hostConfig.UploadBandwidthPrice.MultUint64(storage.SectorSize))
	//
	//	default:
	//		err := fmt.Errorf("unknown action type: %s", action.Type)
	//		log.Error("upload failed, disconnecting ...", "err", err.Error())
	//		p.TriggerError(err)
	//		return
	//	}
	//}
	//
	//// calculate storage revenue and new deposit
	////var storageRevenue, newDeposit *big.Int
	//var storageRevenue, newDeposit common.BigInt
	//
	//if len(newRoots) > len(sr.SectorRoots) {
	//	bytesAdded := storage.SectorSize * uint64(len(newRoots)-len(sr.SectorRoots))
	//	blocksRemaining := sr.ProofDeadline() - currentBlockHeight
	//	blockBytesCurrency := common.NewBigIntUint64(blocksRemaining).Mult(common.NewBigIntUint64(bytesAdded))
	//	storageRevenue = blockBytesCurrency.Mult(hostConfig.StoragePrice)
	//	newDeposit = newDeposit.Add(blockBytesCurrency.Mult(hostConfig.Deposit))
	//}
	//
	//// If a Merkle proof was requested, construct it
	//newMerkleRoot := merkle.Sha256CachedTreeRoot2(newRoots)
	//
	//// Construct the new revision
	//newRevision := latestRevision
	//newRevision.NewRevisionNumber = uploadRequest.NewRevisionNumber
	//for _, action := range uploadRequest.Actions {
	//	if action.Type == storage.UploadActionAppend {
	//		newRevision.NewFileSize += storage.SectorSize
	//	}
	//}
	//newRevision.NewFileMerkleRoot = newMerkleRoot
	//newRevision.NewValidProofOutputs = make([]types.DxcoinCharge, len(latestRevision.NewValidProofOutputs))
	//for i := range newRevision.NewValidProofOutputs {
	//	newRevision.NewValidProofOutputs[i] = types.DxcoinCharge{
	//		Value:   uploadRequest.NewValidProofValues[i],
	//		Address: latestRevision.NewValidProofOutputs[i].Address,
	//	}
	//}
	//newRevision.NewMissedProofOutputs = make([]types.DxcoinCharge, len(latestRevision.NewMissedProofOutputs))
	//for i := range newRevision.NewMissedProofOutputs {
	//	newRevision.NewMissedProofOutputs[i] = types.DxcoinCharge{
	//		Value:   uploadRequest.NewMissedProofValues[i],
	//		Address: latestRevision.NewMissedProofOutputs[i].Address,
	//	}
	//}
	//
	//// new revision verification
	//newRevenue := storageRevenue.Add(bandwidthRevenue).Add(hostConfig.BaseRPCPrice)
	//
	//sr.SectorRoots, newRoots = newRoots, sr.SectorRoots
	//if err := storagehost.VerifyRevision(&sr, &newRevision, currentBlockHeight, newRevenue, newDeposit); err != nil {
	//	log.Error("VerifyRevision Failed", "contractID", newRevision.ParentID.String(), "err", err)
	//	p.TriggerError(err)
	//	return
	//}
	//sr.SectorRoots, newRoots = newRoots, sr.SectorRoots
	//
	//// Calculate which sectors changed
	//oldNumSectors := uint64(len(sr.SectorRoots))
	//proofRanges := make([]merkle.SubTreeLimit, 0, len(sectorsChanged))
	//for index := range sectorsChanged {
	//	if index < oldNumSectors {
	//		proofRanges = append(proofRanges, merkle.SubTreeLimit{
	//			Left:  index,
	//			Right: index + 1,
	//		})
	//	}
	//}
	//sort.Slice(proofRanges, func(i, j int) bool {
	//	return proofRanges[i].Left < proofRanges[j].Left
	//})
	//
	//// Record old leaf hashes for all changed sectors
	//leafHashes := make([]common.Hash, len(proofRanges))
	//for i, r := range proofRanges {
	//	leafHashes[i] = sr.SectorRoots[r.Left]
	//}
	//
	//// Construct the merkle proof
	//oldHashSet, err := merkle.Sha256DiffProof(sr.SectorRoots, proofRanges, oldNumSectors)
	//if err != nil {
	//	err = fmt.Errorf("failed to contract merkle proof: %s", err.Error())
	//	log.Error("upload failed, disconnecting ...", "err", err.Error())
	//	p.TriggerError(err)
	//	return
	//}
	//
	//merkleResp := storage.UploadMerkleProof{
	//	OldSubtreeHashes: oldHashSet,
	//	OldLeafHashes:    leafHashes,
	//	NewMerkleRoot:    newMerkleRoot,
	//}
	//
	//// Calculate bandwidth cost of proof
	//proofSize := storage.HashSize * (len(merkleResp.OldSubtreeHashes) + len(leafHashes) + 1)
	//bandwidthRevenue = bandwidthRevenue.Add(hostConfig.DownloadBandwidthPrice.Mult(common.NewBigInt(int64(proofSize))))
	//
	//// Send upload merkle proof
	//if err := p.SendUploadMerkleProof(merkleResp); err != nil {
	//	err = fmt.Errorf("failed to send upload merkle proof: %s", err.Error())
	//	log.Error("upload failed, disconnecting ...", "err", err.Error())
	//	p.TriggerError(err)
	//	return
	//}
	//
	//// wait until getting response from the storage client
	//msg, err := pm.eth.storageHost.WaitReceived()
	//if err != nil {
	//	err = fmt.Errorf("after mekle proof was sent, failed to get response from the client: %s", err.Error())
	//	log.Error("upload failed, disconnecting ...", "err", err.Error())
	//	p.TriggerError(err)
	//	return
	//}

}
