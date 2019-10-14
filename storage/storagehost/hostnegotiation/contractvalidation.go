// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"fmt"

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
		return fmt.Errorf("storage host failed to validate the new revision's payback: %s", err.Error())
	}

	// validate the expiration block height
	if sr.Expiration()-storagehost.PostponedExecutionBuffer <= blockHeight {
		return fmt.Errorf("storage client is requesting revision afer the reivision deadline")
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

func uploadMerkleRootAndFileSizeValidation(newRev types.StorageContractRevision, sectorRoots []common.Hash) error {
	// validate the merkle root
	if newRev.NewFileMerkleRoot != merkle.Sha256CachedTreeRoot(sectorRoots, sectorHeight) {
		return fmt.Errorf("merkle root validation failed: revision contains bad file merkle root")
	}

	// validate the file size
	if newRev.NewFileSize != uint64(len(sectorRoots))*storage.SectorSize {
		return fmt.Errorf("file size validation failed: contract revision contains bad file size")
	}

	return nil
}

func uploadPaymentValidation(oldRev, newRev types.StorageContractRevision, expectedHostRevenue common.BigInt) error {
	// validate payment from the storage client
	paymentFromClient := common.PtrBigInt(oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value).Sub(common.PtrBigInt(newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value))
	if paymentFromClient.Cmp(expectedHostRevenue) < 0 {
		return fmt.Errorf("uploadPaymentValidation failed: expected client to pay at least %v, insteand paied: %s", expectedHostRevenue, paymentFromClient)
	}

	// validate money transferred to storage host
	paymentToHost := common.PtrBigInt(newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value).Sub(common.PtrBigInt(oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value))
	if paymentToHost.Cmp(paymentFromClient) != 0 {
		return fmt.Errorf("uploadPaymentValidation failed: payment from storage client does not equivalent to payment to the storage hsot")
	}

	return nil
}

func uploadRevisionPaybackValidation(oldRev, newRev types.StorageContractRevision) error {
	// check if the client's validProofPayback is greater than missedProofPayback
	// if so, return error
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(newRev.NewMissedProofOutputs[missedProofPaybackClientAddressIndex].Value) > 0 {
		return fmt.Errorf("uploadRevisionPaybackValidation failed: high client missed payback")
	}

	// check if the new revision client's valid payback is greater than old revision valid payback
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value) > 0 {
		return fmt.Errorf("uploadRevisionPaybackValidation failed: new revision client's validProofPayback should be smaller than old revision validProofPayback")
	}

	// check if host's old revision valid proof payback is greater than new revision valid proof payback
	// if so, return error
	if oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value) > 0 {
		return fmt.Errorf("uploadRevisionPaybackValidation failed: host's new revision validProofPayback should be greater than old revision validProofPayback")
	}

	return nil
}

// oldAndNewRevisionValidation will compare new and old revision. If data are not equivalent to each
// other, then meaning error occurred
func oldAndNewRevisionValidation(oldRev, newRev types.StorageContractRevision) error {
	// parentID validation
	if oldRev.ParentID != newRev.ParentID {
		return fmt.Errorf("oldAndNewRevisionValidation failed: parentID are not equivalent")
	}

	// unlock conditions validation
	if oldRev.UnlockConditions.UnlockHash() != newRev.UnlockConditions.UnlockHash() {
		return fmt.Errorf("oldAndNewRevisionValidation failed: unlock conditions are not equivalent")
	}

	//revision number validation
	if oldRev.NewRevisionNumber >= newRev.NewRevisionNumber {
		return fmt.Errorf("oldAndNewRevisionValidation failed: the revision number from old revision must be smaller than reviison number from new revision")
	}

	// window start and window end validation
	if oldRev.NewWindowStart != newRev.NewWindowStart {
		return fmt.Errorf("oldAndNewRevisionValidation failed: the newWindowStart are not equivalent")
	}

	if oldRev.NewWindowEnd != newRev.NewWindowEnd {
		return fmt.Errorf("oldAndNewRevisionValidation failed: the newWindowEnd are not equivalent")
	}

	// unlock hash validation
	if oldRev.NewUnlockHash != newRev.NewUnlockHash {
		return fmt.Errorf("oldAndNewRevisionValidation failed: unlock hash are not equivalent")
	}

	return nil
}

// contractPaybackValidation validates both contractValidProof payback and contractMissedProof payback
func contractPaybackValidation(hostAddress common.Address, validProofPayback, missedProofPayback []types.DxcoinCharge) error {
	// validate the amount of valid proof payback
	if len(validProofPayback) != expectedValidProofPaybackCounts {
		return fmt.Errorf("something wrong")
	}

	// validate the amount of missed proof payback
	if len(missedProofPayback) != expectedMissedProofPaybackCounts {
		return fmt.Errorf("something wrong")
	}

	// validate the validProofPayback host address
	if validProofPayback[validProofPaybackHostAddressIndex].Address != hostAddress {
		return fmt.Errorf("something wrong")
	}

	// validate the missedProofPayback host address
	if missedProofPayback[missedProofPaybackHostAddressIndex].Address != hostAddress {
		return fmt.Errorf("something wrong")
	}

	return nil
}
