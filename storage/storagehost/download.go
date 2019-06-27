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
func handleDownload(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetBusy()
	defer s.ResetBusy()

	s.SetDeadLine(storage.DownloadTime)

	// read the download request.
	var req storage.DownloadRequest
	err := beginMsg.Decode(req)
	if err != nil {
		return err
	}

	// as soon as client complete downloading, will send stop msg.
	stopSignal := make(chan error, 1)
	go func() {
		msg, err := s.ReadMsg()
		if err != nil {
			stopSignal <- err
		} else if msg.Code != storage.NegotiationStopMsg {
			stopSignal <- errors.New("expected 'stop' from client, got " + string(msg.Code))
		} else {
			stopSignal <- nil
		}
	}()

	// get storage responsibility
	so, err := getStorageResponsibility(h.db, req.StorageContractID)
	if err != nil {
		return fmt.Errorf("[Error Get Storage Responsibility] Error: %v", err)
	}

	// check whether the contract is empty
	if reflect.DeepEqual(so.OriginStorageContract, types.StorageContract{}) {
		err := errors.New("no contract locked")
		<-stopSignal
		return err
	}

	h.lock.RLock()
	settings := h.externalConfig()
	h.lock.RUnlock()

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
			return err
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
		return err
	}

	// Sign the new revision.
	account := accounts.Account{Address: newRevision.NewValidProofOutputs[1].Address}
	wallet, err := h.am.Find(account)
	if err != nil {
		return err
	}

	hostSig, err := wallet.SignHash(account, newRevision.RLPHash().Bytes())
	if err != nil {
		return err
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
		return err
	}

	// enter response loop
	for i, sec := range req.Sections {

		// fetch the requested data from host local storage
		sectorData, err := h.ReadSector(sec.MerkleRoot)
		if err != nil {
			return err
		}
		data := sectorData[sec.Offset : sec.Offset+sec.Length]

		// construct the Merkle proof, if requested.
		var proof []common.Hash
		if req.MerkleProof {
			proofStart := int(sec.Offset) / merkle.LeafSize
			proofEnd := int(sec.Offset+sec.Length) / merkle.LeafSize
			proof, err = merkle.RangeProof(sectorData, proofStart, proofEnd)
			if err != nil {
				return err
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
				return err
			}

			resp.Signature = hostSig
			return s.SendStorageContractDownloadData(resp)
		default:
		}

		if i == len(req.Sections)-1 {
			resp.Signature = hostSig
		}

		if err := s.SendStorageContractDownloadData(resp); err != nil {
			return err
		}
	}

	// the stop signal must arrive before RPC is complete.
	return <-stopSignal
}

// verifyPaymentRevision verifies that the revision being provided to pay for
// the data has transferred the expected amount of money from the renter to the
// host.
func verifyPaymentRevision(existingRevision, paymentRevision types.StorageContractRevision, blockHeight uint64, expectedTransfer *big.Int) error {
	// Check that the revision is well-formed.
	if len(paymentRevision.NewValidProofOutputs) != 2 || len(paymentRevision.NewMissedProofOutputs) != 3 {
		return errBadContractOutputCounts
	}

	// Check that the time to finalize and submit the file contract revision
	// has not already passed.
	if existingRevision.NewWindowStart-postponedExecutionBuffer <= blockHeight {
		return errLateRevision
	}

	// Host payout addresses shouldn't change
	if paymentRevision.NewValidProofOutputs[1].Address != existingRevision.NewValidProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	if paymentRevision.NewMissedProofOutputs[1].Address != existingRevision.NewMissedProofOutputs[1].Address {
		return errors.New("host payout address changed")
	}
	// Make sure the lost collateral still goes to the void
	if paymentRevision.NewMissedProofOutputs[2].Address != existingRevision.NewMissedProofOutputs[2].Address {
		return errors.New("lost collateral address was changed")
	}

	// Determine the amount that was transferred from the renter.
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(existingRevision.NewValidProofOutputs[0].Value) > 0 {
		return ExtendErr("renter increased its valid proof output: ", errHighRenterValidOutput)
	}
	fromRenter := existingRevision.NewValidProofOutputs[0].Value.Sub(existingRevision.NewValidProofOutputs[0].Value, paymentRevision.NewValidProofOutputs[0].Value)
	// Verify that enough money was transferred.
	if fromRenter.Cmp(expectedTransfer) < 0 {
		s := fmt.Sprintf("expected at least %v to be exchanged, but %v was exchanged: ", expectedTransfer, fromRenter)
		return ExtendErr(s, errHighRenterValidOutput)
	}

	// Determine the amount of money that was transferred to the host.
	if existingRevision.NewValidProofOutputs[1].Value.Cmp(paymentRevision.NewValidProofOutputs[1].Value) > 0 {
		return ExtendErr("host valid proof output was decreased: ", errLowHostValidOutput)
	}
	toHost := paymentRevision.NewValidProofOutputs[1].Value.Sub(paymentRevision.NewValidProofOutputs[1].Value, existingRevision.NewValidProofOutputs[1].Value)
	// Verify that enough money was transferred.
	if toHost.Cmp(fromRenter) != 0 {
		s := fmt.Sprintf("expected exactly %v to be transferred to the host, but %v was transferred: ", fromRenter, toHost)
		return ExtendErr(s, errLowHostValidOutput)
	}

	// If the renter's valid proof output is larger than the renter's missed
	// proof output, the renter has incentive to see the host fail. Make sure
	// that this incentive is not present.
	if paymentRevision.NewValidProofOutputs[0].Value.Cmp(paymentRevision.NewMissedProofOutputs[0].Value) > 0 {
		return ExtendErr("renter has incentive to see host fail: ", errHighRenterMissedOutput)
	}

	// Check that the host is not going to be posting collateral.
	if paymentRevision.NewMissedProofOutputs[1].Value.Cmp(existingRevision.NewMissedProofOutputs[1].Value) < 0 {
		collateral := existingRevision.NewMissedProofOutputs[1].Value.Sub(existingRevision.NewMissedProofOutputs[1].Value, paymentRevision.NewMissedProofOutputs[1].Value)
		s := fmt.Sprintf("host not expecting to post any collateral, but contract has host posting %v collateral: ", collateral)
		return ExtendErr(s, errLowHostMissedOutput)
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
	if paymentRevision.NewMissedProofOutputs[1].Value.Cmp(existingRevision.NewMissedProofOutputs[1].Value) != 0 {
		return errLowHostMissedOutput
	}
	return nil
}
