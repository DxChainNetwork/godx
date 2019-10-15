// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

func Handler(np hostnegotiation.Protocol, sp storage.Peer, uploadReqMsg p2p.Msg) {
	var negotiateErr error
	var session hostnegotiation.UploadSession
	defer handleNegotiationErr(np, sp, &negotiateErr)

	// decode the upload request and get the storage responsibility
	uploadReq, sr, err := getUploadReqAndStorageResponsibility(np, &session, uploadReqMsg)
	if err != nil {
		negotiateErr = err
		return
	}

	// get the storage host configuration and start to parse and handle the upload actions
	hostConfig := np.GetHostConfig()
	if err := parseAndHandleUploadActions(&session, uploadReq, sr, hostConfig.UploadBandwidthPrice); err != nil {
		negotiateErr = err
		return
	}

	// construct and verify new contract revision
	newRevision, err := constructAndVerifyNewRevision(np, &session, sr, uploadReq, hostConfig)
	if err != nil {
		negotiateErr = err
		return
	}

	// merkleProof Negotiation
	clientRevisionSign, err := merkleProofNegotiation(sp, &session, sr)
	if err != nil {
		negotiateErr = err
		return
	}

	// host sign and update revision
	if err := signAndUpdateRevision(np, &newRevision, clientRevisionSign); err != nil {
		negotiateErr = err
		return
	}

	// host revision sign negotiation
	if err := hostRevisionSignNegotiation(sp, np, hostConfig, &session, sr, newRevision); err != nil {
		negotiateErr = err
		return
	}
}

// getUploadReqAndStorageResponsibility will decode the upload request and get the storage responsibility
// based on the storage id. In the end, the storage responsibility will be snapshot and stored
// in the upload negotiation data
func getUploadReqAndStorageResponsibility(np hostnegotiation.Protocol, session *hostnegotiation.UploadSession, uploadReqMsg p2p.Msg) (storage.UploadRequest, storagehost.StorageResponsibility, error) {
	var uploadReq storage.UploadRequest
	// decode upload request
	if err := uploadReqMsg.Decode(&uploadReq); err != nil {
		err = uploadNegotiationError(err.Error())
		return storage.UploadRequest{}, storagehost.StorageResponsibility{}, err
	}

	// get the storage responsibility
	sr, err := np.GetStorageResponsibility(uploadReq.StorageContractID)
	if err != nil {
		err = uploadNegotiationError(err.Error())
		return storage.UploadRequest{}, storagehost.StorageResponsibility{}, err
	}

	// snapshot the storage responsibility and return
	session.SrSnapshot = sr
	return uploadReq, sr, nil
}

// parseAndHandleUploadActions will parse the upload actions and handle each of them
// based on its type
func parseAndHandleUploadActions(session *hostnegotiation.UploadSession, uploadReq storage.UploadRequest, sr storagehost.StorageResponsibility,
	uploadBandwidthPrice common.BigInt) error {
	// loop through upload request actions and handle each action based on the action type
	for _, action := range uploadReq.Actions {
		switch action.Type {
		case storage.UploadActionAppend:
			handleAppendAction(action, session, sr, uploadBandwidthPrice)
		default:
			return errParseAction
		}
	}

	return nil
}

// constructAndVerifyNewRevision will construct a new storage contract revision
// and verify the new revision
func constructAndVerifyNewRevision(np hostnegotiation.Protocol, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility,
	uploadReq storage.UploadRequest, hostConfig storage.HostIntConfig) (types.StorageContractRevision, error) {
	// get the latest revision and update the revision
	oldRev := sr.StorageContractRevisions[len(sr.StorageContractRevisions)-1]
	newRev := oldRev
	newRev.NewRevisionNumber = uploadReq.NewRevisionNumber

	// update contract revision
	updateRevisionFileSize(&newRev, uploadReq)
	calcAndUpdateRevisionMerkleRoot(session, &newRev)
	updateRevisionMissedAndValidPayback(&newRev, oldRev, uploadReq)

	// contract revision validation
	blockHeight := np.GetBlockHeight()
	hostRevenue := calcHostRevenue(session, sr, blockHeight, hostConfig)
	sr.SectorRoots, session.SectorRoots = session.SectorRoots, sr.SectorRoots
	if err := uploadRevisionValidation(sr, newRev, blockHeight, hostRevenue); err != nil {
		return types.StorageContractRevision{}, err
	}
	sr.SectorRoots, session.SectorRoots = session.SectorRoots, sr.SectorRoots

	// after validation, return the new revision
	return newRev, nil
}

func merkleProofNegotiation(sp storage.Peer, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility) ([]byte, error) {
	// construct merkle proof
	merkleProof, err := constructUploadMerkleProof(session, sr)
	if err != nil {
		return []byte{}, err
	}

	// send the merkleProof to storage host
	if err := sp.SendUploadMerkleProof(merkleProof); err != nil {
		err := uploadNegotiationError(err.Error())
		return []byte{}, err
	}

	// wait for client revision sign
	return waitAndHandleClientRevSignResp(sp)
}

func signAndUpdateRevision(np hostnegotiation.Protocol, newRev *types.StorageContractRevision, clientRevisionSign []byte) error {
	// get the wallet
	account := accounts.Account{Address: newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Address}
	wallet, err := np.FindWallet(account)
	if err != nil {
		return uploadNegotiationError(err.Error())
	}

	// sign the revision
	hostRevisionSign, err := wallet.SignHash(account, newRev.RLPHash().Bytes())
	if err != nil {
		return uploadNegotiationError(err.Error())
	}

	// update the revision
	newRev.Signatures = [][]byte{clientRevisionSign, hostRevisionSign}
	return nil
}

func hostRevisionSignNegotiation(sp storage.Peer, np hostnegotiation.Protocol, hostConfig storage.HostIntConfig, session *hostnegotiation.UploadSession, sr storagehost.StorageResponsibility, newRev types.StorageContractRevision) error {
	// get the host revision sign from the new revision, and send the upload host revision sign
	hostRevSign := newRev.Signatures[hostSignIndex]
	if err := sp.SendUploadHostRevisionSign(hostRevSign); err != nil {
		return uploadNegotiationError(fmt.Sprintf("hostRevisionSignNeogtiation failed, failed to send the host rev sign: %s", err.Error()))
	}

	// storage host wait and handle the client's response
	return waitAndHandleClientCommitRespUpload(sp, np, session, sr, hostConfig, newRev)
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

	log.Error("Host Upload Negotiation Error", "err", np)

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
