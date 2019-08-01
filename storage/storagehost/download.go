// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/log"
	"math/big"
	"math/bits"
	"reflect"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/storage"
)

// DownloadHandler handles the download negotiation
func DownloadHandler(h *StorageHost, sp storage.Peer, downloadReqMsg p2p.Msg) {
	var hostNegotiateErr, clientNegotiateErr, clientCommitErr error

	defer func() {
		if clientNegotiateErr != nil {
			_ = sp.SendHostAckMsg()
		}

		if hostNegotiateErr != nil {
			if err := sp.SendHostNegotiateErrorMsg(); err == nil {
				msg, err := sp.HostWaitContractResp()
				if err == nil && msg.Code == storage.ClientAckMsg {
					_ = sp.SendHostAckMsg()
				}
			}
		}

		if clientNegotiateErr != nil || clientCommitErr != nil {
			h.ethBackend.CheckAndUpdateConnection(sp.PeerNode())
		}
	}()

	// read the download request.
	var req storage.DownloadRequest
	err := downloadReqMsg.Decode(&req)
	if err != nil {
		clientNegotiateErr = fmt.Errorf("error decoding the download request message: %s", err.Error())
		return
	}

	// get storage responsibility
	h.lock.RLock()
	so, err := getStorageResponsibility(h.db, req.StorageContractID)
	snapshotSo := so
	h.lock.RUnlock()

	// it is totally fine not getting the storage responsibility
	if err != nil {
		hostNegotiateErr = err
		return
	}

	// check whether the contract is empty
	if reflect.DeepEqual(so.OriginStorageContract, types.StorageContract{}) {
		hostNegotiateErr = errors.New("no contract locked")
		return
	}

	settings := h.externalConfig()
	currentRevision := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

	// Validate the request.
	sec := req.Sector
	switch {
	case uint64(sec.Offset)+uint64(sec.Length) > storage.SectorSize:
		err = errors.New("download out boundary of sector")
	case sec.Length == 0:
		err = errors.New("length cannot be 0")
	case req.MerkleProof && (sec.Offset%storage.SegmentSize != 0 || sec.Length%storage.SegmentSize != 0):
		err = errors.New("offset and length must be multiples of SegmentSize when requesting a Merkle proof")
	case len(req.NewValidProofValues) != len(currentRevision.NewValidProofOutputs):
		err = errors.New("the number of valid proof values not match the old")
	case len(req.NewMissedProofValues) != len(currentRevision.NewMissedProofOutputs):
		err = errors.New("the number of missed proof values not match the old")
	}
	if err != nil {
		hostNegotiateErr = fmt.Errorf("download request validation failed: %s", err.Error())
		return
	}

	// construct the new revision
	newRevision := currentRevision
	newRevision.NewRevisionNumber = req.NewRevisionNumber
	newRevision.NewValidProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewValidProofOutputs))
	for i := range newRevision.NewValidProofOutputs {
		newRevision.NewValidProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewValidProofValues[i],
			Address: currentRevision.NewValidProofOutputs[i].Address,
		}
	}
	newRevision.NewMissedProofOutputs = make([]types.DxcoinCharge, len(currentRevision.NewMissedProofOutputs))
	for i := range newRevision.NewMissedProofOutputs {
		newRevision.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Value:   req.NewMissedProofValues[i],
			Address: currentRevision.NewMissedProofOutputs[i].Address,
		}
	}

	// calculate expected cost and verify against client's revision
	var estBandwidth uint64
	sectorAccesses := make(map[common.Hash]struct{})
	// use the worst-case proof size of 2*tree depth (this occurs when
	// proving across the two leaves in the center of the tree)
	estHashesPerProof := 2 * bits.Len64(storage.SectorSize/merkle.LeafSize)
	estBandwidth += uint64(sec.Length) + uint64(estHashesPerProof*storage.HashSize)
	sectorAccesses[sec.MerkleRoot] = struct{}{}

	// calculate total cost
	bandwidthCost := settings.DownloadBandwidthPrice.MultUint64(estBandwidth)
	sectorAccessCost := settings.SectorAccessPrice.MultUint64(uint64(len(sectorAccesses)))
	totalCost := settings.BaseRPCPrice.Add(bandwidthCost).Add(sectorAccessCost)
	err = verifyPaymentRevision(currentRevision, newRevision, h.blockHeight, totalCost.BigIntPtr())
	if err != nil {
		hostNegotiateErr = fmt.Errorf("failed to verify the payment revision: %s", err.Error())
		return
	}

	// Sign the new revision.
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[1].Address}
	wallet, err := h.am.Find(account)
	if err != nil {
		hostNegotiateErr = fmt.Errorf("failed to find the account address: %s", err.Error())
		return
	}

	hostSig, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		hostNegotiateErr = fmt.Errorf("host failed to sign the revision: %s", err.Error())
		return
	}

	newRevision.Signatures = [][]byte{req.Signature, hostSig}

	// update the storage responsibility.
	paymentTransfer := common.NewBigInt(currentRevision.NewValidProofOutputs[0].Value.Int64()).Sub(common.NewBigInt(newRevision.NewValidProofOutputs[0].Value.Int64()))
	so.PotentialDownloadRevenue = so.PotentialDownloadRevenue.Add(paymentTransfer)
	so.StorageContractRevisions = append(so.StorageContractRevisions, newRevision)

	// fetch the requested data from host local storage
	sectorData, err := h.ReadSector(sec.MerkleRoot)
	if err != nil {
		hostNegotiateErr = fmt.Errorf("host failed read sector: %s", err.Error())
		return
	}
	data := sectorData[sec.Offset : sec.Offset+sec.Length]

	// construct the Merkle proof, if requested.
	var proof []common.Hash
	if req.MerkleProof {
		proofStart := int(sec.Offset) / merkle.LeafSize
		proofEnd := int(sec.Offset+sec.Length) / merkle.LeafSize
		proof, err = merkle.Sha256RangeProof(sectorData, proofStart, proofEnd)
		if err != nil {
			hostNegotiateErr = fmt.Errorf("host failed to generate the merkle proof: %s", err.Error())
			return
		}
	}

	// send the response
	resp := storage.DownloadResponse{
		Signature:   nil,
		Data:        data,
		MerkleProof: proof,
	}

	resp.Signature = hostSig
	if err := sp.SendContractDownloadData(resp); err != nil {
		log.Error("failed to send the contract download data message", "err", err)
		return
	}

	// wait for client commit success msg
	msg, err := sp.HostWaitContractResp()
	if err != nil {
		log.Error("storage host failed to get client commit success msg", "err", err)
		return
	}

	var isHostCommitSuccess = true
	if msg.Code == storage.ClientCommitSuccessMsg {
		h.lock.Lock()
		err = h.modifyStorageResponsibility(so, nil, nil, nil)
		h.lock.Unlock()
		if err != nil {
			isHostCommitSuccess = false
			if err := sp.SendHostCommitFailedMsg(); err != nil {
				log.Error("storage host failed to send commit failed msg", "err", err)
				return
			}

			// wait for client ack msg
			msg, err = sp.HostWaitContractResp()
			if err != nil {
				log.Error("storage host failed to get client ack msg", "err", err)
				return
			}
		}
	} else if msg.Code == storage.ClientCommitFailedMsg {
		clientCommitErr = storage.ClientCommitErr
	} else if msg.Code == storage.ClientNegotiateErrorMsg {
		clientNegotiateErr = storage.ClientNegotiateErr
		return
	}

	// In case the user restart the program, the contract create handler will not be
	// reached. Therefore, needs another check in the download handler
	if !sp.IsStaticConn() {
		node := sp.PeerNode()
		if node == nil {
			return
		}
		h.ethBackend.SetStatic(node)
	}

	// send host 'ACK' msg to client
	if err := sp.SendHostAckMsg(); err != nil {
		log.Error("storage host failed to send host ack msg", "err", err)

		if isHostCommitSuccess {
			_ = h.rollbackStorageResponsibility(snapshotSo, nil, nil, nil)
		}
	}
}

// verifyPaymentRevision verifies that the revision being provided to pay for
// the data has transferred the expected amount of money from the client to the
// host.
func verifyPaymentRevision(existingRevision, paymentRevision types.StorageContractRevision, blockHeight uint64, expectedTransfer *big.Int) error {
	// Check that the revision is well-formed.
	if len(paymentRevision.NewValidProofOutputs) != 2 || len(paymentRevision.NewMissedProofOutputs) != 2 {
		return errBadContractOutputCounts
	}

	// Check that the time to finalize and submit the file contract revision
	// has not already passed.
	if existingRevision.NewWindowStart-postponedExecutionBuffer <= blockHeight {
		return errLateRevision
	}

	// Host payout addresses shouldn't change
	if paymentRevision.NewValidProofOutputs[1].Address != existingRevision.NewValidProofOutputs[1].Address {
		return errors.New("host payout address changed during downloading")
	}
	if paymentRevision.NewMissedProofOutputs[1].Address != existingRevision.NewMissedProofOutputs[1].Address {
		return errors.New("host payout address changed during downloading")
	}

	// Host missed proof payout should not change
	if paymentRevision.NewMissedProofOutputs[1].Value.Cmp(existingRevision.NewMissedProofOutputs[1].Value) != 0 {
		return errors.New("host missed proof payout changed during downloading")
	}

	// Determine the amount that was transferred from the client.
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(existingRevision.NewValidProofOutputs[0].Value) > 0 {
		return ExtendErr("client increased its valid proof output during downloading: ", errHighRenterValidOutput)
	}

	// Verify that enough money was transferred.
	fromClient := common.NewBigInt(existingRevision.NewValidProofOutputs[0].Value.Int64()).Sub(common.NewBigInt(paymentRevision.NewValidProofOutputs[0].Value.Int64()))
	if fromClient.BigIntPtr().Cmp(expectedTransfer) < 0 {
		s := fmt.Sprintf("expected at least %v to be exchanged, but %v was exchanged during downloading: ", expectedTransfer, fromClient)
		return ExtendErr(s, errHighRenterValidOutput)
	}

	// Determine the amount of money that was transferred to the host.
	if existingRevision.NewValidProofOutputs[1].Value.Cmp(paymentRevision.NewValidProofOutputs[1].Value) > 0 {
		return ExtendErr("host valid proof output was decreased during downloading: ", errLowHostValidOutput)
	}

	// Verify that enough money was transferred.
	toHost := common.NewBigInt(paymentRevision.NewValidProofOutputs[1].Value.Int64()).Sub(common.NewBigInt(existingRevision.NewValidProofOutputs[1].Value.Int64()))
	if toHost.Cmp(fromClient) != 0 {
		s := fmt.Sprintf("expected exactly %v to be transferred to the host, but %v was transferred during downloading: ", fromClient, toHost)
		return ExtendErr(s, errLowHostValidOutput)
	}

	// Avoid that client has incentive to see the host fail. in that case, client maybe purposely set larger missed output
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(paymentRevision.NewMissedProofOutputs[0].Value) > 0 {
		return ExtendErr("client has incentive to see host fail during downloading: ", errHighRenterMissedOutput)
	}

	// Check that the revision count has increased.
	if paymentRevision.NewRevisionNumber <= existingRevision.NewRevisionNumber {
		return errBadRevisionNumber
	}

	// Check that all of the non-volatile fields are the same.
	if paymentRevision.ParentID != existingRevision.ParentID {
		return errBadParentID
	}
	if paymentRevision.UnlockConditions.UnlockHash() != existingRevision.UnlockConditions.UnlockHash() {
		return errBadUnlockConditions
	}
	if paymentRevision.NewFileSize != existingRevision.NewFileSize {
		return errBadFileSize
	}
	if paymentRevision.NewFileMerkleRoot != existingRevision.NewFileMerkleRoot {
		return errBadFileMerkleRoot
	}
	if paymentRevision.NewWindowStart != existingRevision.NewWindowStart {
		return errBadWindowStart
	}
	if paymentRevision.NewWindowEnd != existingRevision.NewWindowEnd {
		return errBadWindowEnd
	}
	if paymentRevision.NewUnlockHash != existingRevision.NewUnlockHash {
		return errBadUnlockHash
	}

	return nil
}
