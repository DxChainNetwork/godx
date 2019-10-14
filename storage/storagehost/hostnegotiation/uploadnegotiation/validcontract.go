// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

func uploadRevisionValidation(sr storagehost.StorageResponsibility, newRevision types.StorageContractRevision, blockHeight uint64, expectedHostRevenue common.BigInt) error {
	// contract payback validation
	oldRevision := sr.StorageContractRevisions[len(sr.StorageContractRevisions)-1]
	hostAddress := oldRevision.NewValidProofOutputs[validProofPaybackHostAddressIndex].Address
	if err := contractPaybackValidation(hostAddress, newRevision.NewValidProofOutputs, newRevision.NewMissedProofOutputs); err != nil {
		return err
	}

	// validate the expiration block height
	if sr.Expiration()-storagehost.PostponedExecutionBuffer <= blockHeight {
		return errReachDeadline
	}

	// compare old revision and new revision
	if err := oldAndNewRevisionValidation(oldRevision, newRevision); err != nil {
		return err
	}

	// revision payback validation
	if err := uploadRevisionPaybackValidation(oldRevision, newRevision); err != nil {
		return err
	}

	// upload payment validation
	if err := uploadPaymentValidation(oldRevision, newRevision, expectedHostRevenue); err != nil {
		return err
	}

	// merkle root and file size validation
	if err := uploadMerkleRootAndFileSizeValidation(newRevision, sr.SectorRoots); err != nil {
		return err
	}

	return nil
}

// contractPaybackValidation validates both contractValidProof payback and contractMissedProof payback
func contractPaybackValidation(hostAddress common.Address, validProofPayback, missedProofPayback []types.DxcoinCharge) error {
	// validate the amount of valid proof payback
	if len(validProofPayback) != expectedValidProofPaybackCounts {
		return errBadValidPaybackCounts
	}

	// validate the amount of missed proof payback
	if len(missedProofPayback) != expectedMissedProofPaybackCounts {
		return errBadMissedPaybackCounts
	}

	// validate the validProofPayback host address
	if validProofPayback[validProofPaybackHostAddressIndex].Address != hostAddress {
		return errBadValidHostAddress
	}

	// validate the missedProofPayback host address
	if missedProofPayback[missedProofPaybackHostAddressIndex].Address != hostAddress {
		return errBadMissedHostAddress
	}

	return nil
}

// oldAndNewRevisionValidation will compare new and old revision. If data are not equivalent to each
// other, then meaning error occurred
func oldAndNewRevisionValidation(oldRev, newRev types.StorageContractRevision) error {
	// parentID validation
	if oldRev.ParentID != newRev.ParentID {
		return errBadParentID
	}

	// unlock conditions validation
	if oldRev.UnlockConditions.UnlockHash() != newRev.UnlockConditions.UnlockHash() {
		return errBadUnlockCondition
	}

	//revision number validation
	if oldRev.NewRevisionNumber >= newRev.NewRevisionNumber {
		return errSmallRevisionNumber
	}

	// window start and window end validation
	if oldRev.NewWindowStart != newRev.NewWindowStart {
		return errBadWindowStart
	}

	if oldRev.NewWindowEnd != newRev.NewWindowEnd {
		return errBadWindowEnd
	}

	// unlock hash validation
	if oldRev.NewUnlockHash != newRev.NewUnlockHash {
		return errBadUnlockHash
	}

	return nil
}

// uploadRevisionPaybackValidation validates the revision's payback, including both
// missedProofPayback and validProofPayback
func uploadRevisionPaybackValidation(oldRev, newRev types.StorageContractRevision) error {
	// check if the client's validProofPayback is greater than missedProofPayback
	// if so, return error
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(newRev.NewMissedProofOutputs[missedProofPaybackClientAddressIndex].Value) > 0 {
		return errHighClientMissedPayback
	}

	// check if the new revision client's valid payback is greater than old revision valid payback
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value) > 0 {
		return errSmallerClientValidPayback
	}

	// check if host's old revision valid proof payback is greater than new revision valid proof payback
	// if so, return error
	if oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value) > 0 {
		return errSmallHostValidPayback
	}

	return nil
}

// uploadPaymentValidation validates the upload payment that storage client paid to the storage host
func uploadPaymentValidation(oldRev, newRev types.StorageContractRevision, expectedHostRevenue common.BigInt) error {
	// validate payment from the storage client
	paymentFromClient := common.PtrBigInt(oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value).Sub(common.PtrBigInt(newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value))
	if paymentFromClient.Cmp(expectedHostRevenue) < 0 {
		return errClientPayment
	}

	// validate money transferred to storage host
	paymentToHost := common.PtrBigInt(newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value).Sub(common.PtrBigInt(oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value))
	if paymentToHost.Cmp(paymentFromClient) != 0 {
		return errHostPaymentReceived
	}

	return nil
}

// uploadMerkleRootAndFileSizeValidation validates the merkle root and the file size
func uploadMerkleRootAndFileSizeValidation(newRev types.StorageContractRevision, sectorRoots []common.Hash) error {
	// validate the merkle root
	if newRev.NewFileMerkleRoot != merkle.Sha256CachedTreeRoot(sectorRoots, sectorHeight) {
		return errBadMerkleRoot
	}

	// validate the file size
	if newRev.NewFileSize != uint64(len(sectorRoots))*storage.SectorSize {
		return errBadFileSize
	}

	return nil
}
