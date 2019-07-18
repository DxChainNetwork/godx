package storagehost

import (
	"errors"
	"fmt"
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

// handleDownload is the function to be called for download negotiation
func (h *StorageHost) DownloadHandler(sp storage.Peer, downloadReqMsg p2p.Msg) {
	var downloadErr error

	defer func() {
		if downloadErr != nil {
			h.log.Error("data download failed", "err", downloadErr.Error())
			sp.TriggerError(downloadErr)
		}
	}()

	// read the download request.
	var req storage.DownloadRequest
	err := downloadReqMsg.Decode(&req)
	if err != nil {
		downloadErr = fmt.Errorf("error decoding the download request message: %s", err.Error())
		return
	}

	// as soon as client complete downloading, will send stop msg.
	stopSignal := make(chan error, 1)
	go func() {
		msg, err := sp.HostWaitContractResp()
		if err != nil {
			stopSignal <- err
		} else if msg.Code != storage.NegotiationStopMsg {
			stopSignal <- errors.New("expected 'stop' from client, got " + string(msg.Code))
		} else {
			stopSignal <- nil
		}
	}()

	// get storage responsibility
	h.lock.RLock()
	so, err := getStorageResponsibility(h.db, req.StorageContractID)
	h.lock.RUnlock()

	// it is totally fine not getting the storage responsibility
	if err != nil {
		return
	}

	// check whether the contract is empty
	if reflect.DeepEqual(so.OriginStorageContract, types.StorageContract{}) {
		downloadErr = errors.New("no contract locked")
		<-stopSignal
		return
	}

	settings := h.externalConfig()

	currentRevision := so.StorageContractRevisions[len(so.StorageContractRevisions)-1]

	// Validate the request.
	for _, sec := range req.Sections {
		var err error
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
			downloadErr = fmt.Errorf("download request validation failed: %s", err.Error())
			return
		}
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
	for _, sec := range req.Sections {
		// use the worst-case proof size of 2*tree depth (this occurs when
		// proving across the two leaves in the center of the tree)
		estHashesPerProof := 2 * bits.Len64(storage.SectorSize/merkle.LeafSize)
		estBandwidth += uint64(sec.Length) + uint64(estHashesPerProof*storage.HashSize)
		sectorAccesses[sec.MerkleRoot] = struct{}{}
	}

	// calculate total cost
	bandwidthCost := settings.DownloadBandwidthPrice.MultUint64(estBandwidth)
	sectorAccessCost := settings.SectorAccessPrice.MultUint64(uint64(len(sectorAccesses)))
	totalCost := settings.BaseRPCPrice.Add(bandwidthCost).Add(sectorAccessCost)
	err = verifyPaymentRevision(currentRevision, newRevision, h.blockHeight, totalCost.BigIntPtr())
	if err != nil {
		downloadErr = fmt.Errorf("failed to verify the payment revision: %s", err.Error())
		return
	}

	// Sign the new revision.
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[1].Address}
	wallet, err := h.am.Find(account)
	if err != nil {
		downloadErr = fmt.Errorf("failed to find the account address: %s", err.Error())
		return
	}

	hostSig, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		downloadErr = fmt.Errorf("host failed to sign the revision: %s", err.Error())
		return
	}

	newRevision.Signatures = [][]byte{req.Signature, hostSig}

	// update the storage responsibility.
	paymentTransfer := common.NewBigInt(currentRevision.NewValidProofOutputs[0].Value.Int64()).Sub(common.NewBigInt(newRevision.NewValidProofOutputs[0].Value.Int64()))
	so.PotentialDownloadRevenue = so.PotentialDownloadRevenue.Add(paymentTransfer)
	so.StorageContractRevisions = append(so.StorageContractRevisions, newRevision)
	h.lock.Lock()
	err = h.modifyStorageResponsibility(so, nil, nil, nil)
	h.lock.Unlock()
	if err != nil {
		downloadErr = fmt.Errorf("host failed to modify the storage responsibility: %s", err.Error())
		return
	}

	// enter response loop
	for i, sec := range req.Sections {

		// fetch the requested data from host local storage
		sectorData, err := h.ReadSector(sec.MerkleRoot)
		if err != nil {
			downloadErr = fmt.Errorf("host failed read sector: %s", err.Error())
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
				downloadErr = fmt.Errorf("host failed to generate the merkle proof: %s", err.Error())
				return
			}
		}

		// send the response.
		// if host received a stop signal, or this is the final response, include host's signature in the response.
		resp := storage.DownloadResponse{
			Signature:   nil,
			Data:        data,
			MerkleProof: proof,
		}

		select {
		case err := <-stopSignal:
			if err != nil {
				downloadErr = fmt.Errorf("stop signal received: %s", err.Error())
				return
			}

			resp.Signature = hostSig
			if err := sp.SendContractDownloadData(resp); err != nil {
				downloadErr = fmt.Errorf("failed to send the contract download data message: %s", err.Error())
				return
			}
		default:
		}

		if i == len(req.Sections)-1 {
			resp.Signature = hostSig
		}

		if err := sp.SendContractDownloadData(resp); err != nil {
			downloadErr = fmt.Errorf("failed to send the contract download data message: %s", err.Error())
			return
		}
	}

	// the stop signal must arrive before RPC is complete.
	downloadErr = <-stopSignal
	return
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
