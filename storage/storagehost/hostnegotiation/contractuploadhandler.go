// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"fmt"
	"sort"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func ContractUploadHandler(np NegotiationProtocol, sp storage.Peer, uploadReqMsg p2p.Msg) {
	var negotiateErr error
	var nd uploadNegotiationData
	defer handleNegotiationErr(&negotiateErr, sp, np)

	// decode the upload request and get the storage responsibility
	uploadReq, sr, err := decodeUploadReqAndGetSr(np, &nd, uploadReqMsg)
	if err != nil {
		negotiateErr = err
		return
	}

	// get the storage host configuration and start to parse and handle the upload actions
	hostConfig := np.GetHostConfig()
	if err := parseAndHandleUploadActions(uploadReq, &nd, sr, hostConfig.UploadBandwidthPrice); err != nil {
		negotiateErr = err
		return
	}

	// construct and verify new contract revision
	newRevision, err = constructAndVerifyNewRevision(np, &nd, sr, uploadReq, hostConfig)
	if err != nil {
		negotiateErr = err
		return
	}

	// construct upload merkle proof
	merkleProof, err := constructUploadMerkleProof(nd, sr)
	if err != nil {
		negotiateErr = err
		return
	}

}

// decodeUploadReqAndGetSr will decode the upload request and get the storage responsibility
// based on the storage id. In the end, the storage responsibility will be snapshot and stored
// in the upload negotiation data
func decodeUploadReqAndGetSr(np NegotiationProtocol, nd *uploadNegotiationData, uploadReqMsg p2p.Msg) (storage.UploadRequest, storagehost.StorageResponsibility, error) {
	var uploadReq storage.UploadRequest
	// decode upload request
	if err := uploadReqMsg.Decode(&uploadReq); err != nil {
		err = fmt.Errorf("failed to decode the upload request: %s", err.Error())
		return storage.UploadRequest{}, storagehost.StorageResponsibility{}, err
	}

	// get the storage responsibility
	sr, err := np.GetStorageResponsibility(uploadReq.StorageContractID)
	if err != nil {
		err = fmt.Errorf("failed to retrieve the storage responsibility: %s", err.Error())
		return storage.UploadRequest{}, storagehost.StorageResponsibility{}, err
	}

	// snapshot the storage responsibility, and return
	nd.srSnapshot = sr
	return uploadReq, sr, nil
}

// parseAndHandleUploadActions will parse the upload actions based on the type of the action
// currently, there is only one type which is append. During the action handling process, the
// following data will be calculated and recorded in the uploadNegotiationData
// 	1. newRoots
//  2. sectorGained
//  3. gainedSectorData
//  4. sectorsChanged
//  5. bandwidthRevenue
func parseAndHandleUploadActions(uploadReq storage.UploadRequest, nd *uploadNegotiationData, sr storagehost.StorageResponsibility, uploadBandwidthPrice common.BigInt) error {
	// data preparation
	nd.newRoots = append(nd.newRoots, sr.SectorRoots...)
	nd.sectorsChanged = make(map[uint64]struct{})

	// loop and handle each action
	for _, action := range uploadReq.Actions {
		switch action.Type {
		case storage.UploadActionAppend:
			handleUploadAppendType(action, nd, uploadBandwidthPrice)
		default:
			return fmt.Errorf("failed to parse the upload action, unknown upload action type: %s", action.Type)
		}
	}

	return nil
}

// constructAndVerifyNewRevision will construct a new storage contract revision
// and verify the new revision
func constructAndVerifyNewRevision(np NegotiationProtocol, nd *uploadNegotiationData, sr storagehost.StorageResponsibility, uploadReq storage.UploadRequest, hostConfig storage.HostIntConfig) (types.StorageContractRevision, error) {
	// get the latest revision and update the revision
	currentRev := sr.StorageContractRevisions[len(sr.StorageContractRevisions)-1]
	newRev := currentRev
	newRev.NewRevisionNumber = uploadReq.NewRevisionNumber

	// update contract revision
	updateRevisionFileSize(&newRev, uploadReq)
	calcAndUpdateRevisionMerkleRoot(nd, &newRev)
	updateRevisionMissedAndValidPayback(nd, &newRev, currentRev, uploadReq)

	// contract revision validation
	blockHeight := np.GetBlockHeight()
	hostRevenue := calcHostRevenue(nd, sr, blockHeight, hostConfig)
	sr.SectorRoots, nd.newRoots = nd.newRoots, sr.SectorRoots
	if err := uploadRevisionValidation(sr, newRev, blockHeight, hostRevenue); err != nil {
		return types.StorageContractRevision{}, err
	}
	sr.SectorRoots, nd.newRoots = nd.newRoots, sr.SectorRoots

	// after validation, return the new revision
	return newRev, nil
}

func constructUploadMerkleProof(nd uploadNegotiationData, sr storagehost.StorageResponsibility) (storage.UploadMerkleProof, error) {
	proofRanges := calcAndSortProofRanges(sr, nd)
	leafHashes := calcLeafHashes(proofRanges, sr)
	oldHashSet, err := calcOldHashSet(sr, proofRanges)
	if err != nil {
		err = fmt.Errorf("failed to construct upload merkle proof: %s", err.Error())
		return storage.UploadMerkleProof{}, err
	}

	// construct the merkle proof
	return storage.UploadMerkleProof{
		OldSubtreeHashes: oldHashSet,
		OldLeafHashes:    leafHashes,
		NewMerkleRoot:    nd.newMerkleRoot,
	}, nil

}

func calcAndSortProofRanges(sr storagehost.StorageResponsibility, nd uploadNegotiationData) []merkle.SubTreeLimit {
	// calculate proof ranges
	oldNumSectors := uint64(len(sr.SectorRoots))
	var proofRanges []merkle.SubTreeLimit
	for i := range nd.sectorsChanged {
		if i < oldNumSectors {
			proofRange := merkle.SubTreeLimit{
				Left:  i,
				Right: i + 1,
			}

			proofRanges = append(proofRanges, proofRange)
		}
	}

	// sort proof ranges
	sort.Slice(proofRanges, func(i, j int) bool {
		return proofRanges[i].Left < proofRanges[j].Left
	})

	return proofRanges
}

func calcLeafHashes(proofRanges []merkle.SubTreeLimit, sr storagehost.StorageResponsibility) []common.Hash {
	var leafHashes []common.Hash
	for _, proofRange := range proofRanges {
		leafHashes = append(leafHashes, sr.SectorRoots[proofRange.Left])
	}

	return leafHashes
}

func calcOldHashSet(sr storagehost.StorageResponsibility, proofRanges []merkle.SubTreeLimit) ([]common.Hash, error) {
	return merkle.Sha256DiffProof(sr.SectorRoots, proofRanges, uint64(len(sr.SectorRoots)))
}

// updateRevisionFileSize will update the new contract revision's file size based on the
// type of the upload action
// 	 1. UploadActionAppend -> based on the number of append action, increase the file size by sectorSize
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
func calcAndUpdateRevisionMerkleRoot(nd *uploadNegotiationData, newRev *types.StorageContractRevision) {
	// calculate the new merkle roots and update the new revision
	nd.newMerkleRoot = merkle.Sha256CachedTreeRoot2(nd.newRoots)
	newRev.NewFileMerkleRoot = nd.newMerkleRoot
}

// updateRevisionMissedAndValidPayback will update the new contract revision missed and valid
// proof payback
func updateRevisionMissedAndValidPayback(nd *uploadNegotiationData, newRev *types.StorageContractRevision, currentRev types.StorageContractRevision, uploadReq storage.UploadRequest) {
	// update the revision valid proof outputs
	for i := range currentRev.NewValidProofOutputs {
		validProofOutput := types.DxcoinCharge{
			Value:   uploadReq.NewValidProofValues[i],
			Address: currentRev.NewValidProofOutputs[i].Address,
		}
		newRev.NewValidProofOutputs = append(newRev.NewValidProofOutputs, validProofOutput)
	}

	// update the revision missed proof outputs
	for i := range currentRev.NewValidProofOutputs {
		missedProofOutput := types.DxcoinCharge{
			Value:   uploadReq.NewMissedProofValues[i],
			Address: currentRev.NewMissedProofOutputs[i].Address,
		}
		newRev.NewMissedProofOutputs = append(newRev.NewMissedProofOutputs, missedProofOutput)
	}
}

// handleUploadAppendType will handle the upload action with the type UploadAppendAction
// by handing it, a bunch of data will be calculated and recorded in the uploadNegotiationData
func handleUploadAppendType(action storage.UploadAction, nd *uploadNegotiationData, uploadBandwidthPrice common.BigInt) {
	// update upload negotiation data
	newRoot := merkle.Sha256MerkleTreeRoot(action.Data)
	nd.newRoots = append(nd.newRoots, newRoot)
	nd.sectorGained = append(nd.sectorGained, newRoot)
	nd.gainedSectorData = append(nd.gainedSectorData, action.Data)
	nd.sectorsChanged[uint64(len(nd.newRoots)-1)] = struct{}{}
	nd.bandwidthRevenue = nd.bandwidthRevenue.Add(uploadBandwidthPrice.MultUint64(storage.SectorSize))
}
