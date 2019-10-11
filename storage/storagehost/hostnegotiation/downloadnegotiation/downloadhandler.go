// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiation

import (
	"fmt"
	"reflect"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// ContractHandler handles the download negotiation process
func ContractHandler(np hostnegotiation.Protocol, sp storage.Peer, downloadReqMsg p2p.Msg) {
	var negotiateErr error
	var session hostnegotiation.DownloadSession
	defer handleNegotiationErr(np, sp, &negotiateErr)

	// 1. decode the download request and get the storage responsibility
	req, sr, err := getDownloadRequestAndStorageResponsibility(np, &session, downloadReqMsg)
	if err != nil {
		negotiateErr = err
		return
	}

	// 2. validate the download request
	oldRev := sr.StorageContractRevisions[len(sr.StorageContractRevisions)-1]
	if err := downloadRequestValidation(req, oldRev); err != nil {
		negotiateErr = err
		return
	}

	// 3. construct new contract revision and validate the new revision
	newRevision := constructNewRevision(oldRev, req)
	if err := downloadRevisionValidation(np, oldRev, newRevision, req.Sector); err != nil {
		negotiateErr = err
		return
	}

	// 4. send the download request data
	if err := sendDownloadResp(np, sp, &session, req, newRevision); err != nil {
		negotiateErr = err
		return
	}

	// 5. handle storage client's response
	if err := clientDownloadRespHandle(np, sp, session, sr, oldRev); err != nil {
		negotiateErr = err
		return
	}

	// 6. update connection and send ack
	if err := updateConnAndSendACK(np, sp, session.SrSnapshot); err != nil {
		negotiateErr = err
		return
	}
}

// getDownloadRequestAndStorageResponsibility will decode the downloadReqMsg and based on the information
// acquired, get the corresponded storage responsibility
func getDownloadRequestAndStorageResponsibility(np hostnegotiation.Protocol, session *hostnegotiation.DownloadSession, downloadReqMsg p2p.Msg) (storage.DownloadRequest, storagehost.StorageResponsibility, error) {
	// decode the download request
	var req storage.DownloadRequest
	if err := downloadReqMsg.Decode(&req); err != nil {
		return storage.DownloadRequest{}, storagehost.StorageResponsibility{}, err
	}

	// get the storage responsibility
	sr, err := np.GetStorageResponsibility(req.StorageContractID)
	if err != nil {
		return storage.DownloadRequest{}, storagehost.StorageResponsibility{}, err
	}
	session.SrSnapshot = sr

	// validate the storage responsibility, and make a snapshot
	if reflect.DeepEqual(sr.OriginStorageContract, types.StorageContract{}) {
		return storage.DownloadRequest{}, storagehost.StorageResponsibility{}, fmt.Errorf("contract saved in the storage responsibility is empty")
	}
	session.SrSnapshot = sr
	return req, sr, nil
}

// downloadRequestValidation validates the download request
func downloadRequestValidation(downloadReq storage.DownloadRequest, latestRevision types.StorageContractRevision) error {
	// validate the download data sector
	if err := downloadSectorValidation(downloadReq.Sector, downloadReq.MerkleProof); err != nil {
		return err
	}

	// validate the download request payback for both valid and missed proof
	if err := downloadRequestPaybackValidation(downloadReq.NewValidProofValues, downloadReq.NewMissedProofValues, latestRevision); err != nil {
		return err
	}

	return nil
}

// constructNewRevision will construct the new contract revision based on the latest revision information and
// the information included in the download request
func constructNewRevision(latestRev types.StorageContractRevision, req storage.DownloadRequest) types.StorageContractRevision {
	// assign the latest revision to the new revision, and update the information accordingly
	newRevision := latestRev
	newRevision.NewRevisionNumber = req.NewRevisionNumber

	// clear the new revision's new valid proof payback, then update them
	newRevision.NewValidProofOutputs = make([]types.DxcoinCharge, len(latestRev.NewValidProofOutputs))
	for i := range newRevision.NewValidProofOutputs {
		newRevision.NewValidProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewValidProofValues[i],
			Address: latestRev.NewValidProofOutputs[i].Address,
		}
	}

	// clear the new revision's new missed proof payback, then update them
	newRevision.NewMissedProofOutputs = make([]types.DxcoinCharge, len(latestRev.NewValidProofOutputs))
	for i := range newRevision.NewMissedProofOutputs {
		newRevision.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewMissedProofValues[i],
			Address: latestRev.NewMissedProofOutputs[i].Address,
		}
	}

	// return the new revision
	return newRevision
}

// downloadRevisionValidation validates the revision;s payback, compares the new revision with the old
// revision, and etc.
func downloadRevisionValidation(np hostnegotiation.Protocol, oldRev, newRev types.StorageContractRevision, dataSector storage.DownloadRequestSector) error {
	// new download revision payback validation
	if err := downloadRevPaybackValidation(oldRev, newRev); err != nil {
		return err
	}

	// compares new revision and old revision no-changeable fields
	if err := downloadNewRevAndOldRevValidation(oldRev, newRev); err != nil {
		return err
	}

	// validates the revision number and if the revision is submitted late by the client
	if err := revNumberAndWindowValidation(np, oldRev, newRev); err != nil {
		return err
	}

	// validates the amount of tokens transferred by the storage client
	if err := clientTokenTransferValidation(np, dataSector, oldRev, newRev); err != nil {
		return err
	}

	return nil
}

// sendDownloadResp send the requested data sector, update the storage revision, and etc. back to the
// storage client
func sendDownloadResp(np hostnegotiation.Protocol, sp storage.Peer, session *hostnegotiation.DownloadSession, req storage.DownloadRequest, newRev types.StorageContractRevision) error {
	// sign and update the download revision
	err := signAndUpdateDownloadRevision(np, session, newRev, req.Signature)
	if err != nil {
		return err
	}

	// construct download response
	downloadResp, err := constructDownloadResp(np, req, newRev.Signatures[hostSignIndex])
	if err != nil {
		return err
	}

	// send the response to the storage client
	if err := sp.SendContractDownloadData(downloadResp); err != nil {
		return err
	}

	return nil
}

// clientDownloadRespHandle handles the response from the storage client after
// sent the data requested by the storage client
func clientDownloadRespHandle(np hostnegotiation.Protocol, sp storage.Peer, session hostnegotiation.DownloadSession, sr storagehost.StorageResponsibility, oldRev types.StorageContractRevision) error {
	// wait for the client response message
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		return downloadNegotiationError(err.Error())
	}

	// based on the message code, handle it differently
	if msg.Code == storage.ClientCommitSuccessMsg {
		return updateHostResponsibility(np, sp, session, sr, oldRev)
	} else if msg.Code == storage.ClientCommitFailedMsg {
		return storage.ErrClientCommit
	} else if msg.Code == storage.ClientNegotiateErrorMsg {
		return storage.ErrClientNegotiate
	}

	return nil
}

// updateConnAndSendACK will update the connection and send the host acknowledgement to
// storage client
func updateConnAndSendACK(np hostnegotiation.Protocol, sp storage.Peer, srSnapShot storagehost.StorageResponsibility) error {
	// check if the connection is static, if not, update the connection
	if !sp.IsStaticConn() {
		node := sp.PeerNode()
		if node == nil {
			return nil
		}
		np.SetStatic(node)
	}

	// send the host acknowledgement message
	if err := sp.SendHostAckMsg(); err != nil {
		_ = np.RollbackUploadStorageResponsibility(srSnapShot, nil, nil, nil)
		np.CheckAndUpdateConnection(sp.PeerNode())
		return err
	}

	return nil
}

// handleNegotiationErr will handle the following error cases
//  1. no error          -> return directly
//  2. ErrHostCommit     -> send host commit, after getting response from client, send host ack message
//  3. other error types -> send host negotiation error
func handleNegotiationErr(np hostnegotiation.Protocol, sp storage.Peer, negotiateErr *error) {
	// return directly if there are no errors
	if negotiateErr == nil {
		return
	}

	// check and handle other negotiation errors
	if common.ErrContains(*negotiateErr, storage.ErrHostCommit) {
		_ = sp.SendHostCommitFailedMsg()
		// wait for client ack message
		if _, err := sp.HostWaitContractResp(); err != nil {
			return
		}
		// send back host ack message
		_ = sp.SendHostAckMsg()
	} else {
		// for other negotiation error message, send back the host negotiation error message
		// directly
		_ = sp.SendHostNegotiateErrorMsg()
		np.CheckAndUpdateConnection(sp.PeerNode())
	}
}

// updateHostResponsibility will update the host's storage responsibility and commit
// the information locally
func updateHostResponsibility(np hostnegotiation.Protocol, sp storage.Peer, session hostnegotiation.DownloadSession, sr storagehost.StorageResponsibility, oldRev types.StorageContractRevision) error {
	// update the storage responsibility
	downloadRevenue := common.PtrBigInt(session.NewRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value).Sub(common.PtrBigInt(oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value))
	sr.PotentialDownloadRevenue = sr.PotentialDownloadRevenue.Add(downloadRevenue)
	sr.StorageContractRevisions = append(sr.StorageContractRevisions, session.NewRev)
	if err := np.ModifyStorageResponsibility(sr, nil, nil, nil); err == nil {
		return nil
	}

	// if failed to modify the storage responsibility, send the failed commit message
	// back to the client, and wait for its response
	_ = sp.SendHostCommitFailedMsg()
	_, err := sp.HostWaitContractResp()
	if err != nil {
		return err
	}

	// send storage host acknowledgement
	_ = sp.SendHostAckMsg()
	return errCommitFailed
}
