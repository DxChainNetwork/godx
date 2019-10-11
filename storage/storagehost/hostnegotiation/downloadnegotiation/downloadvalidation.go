// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package downloadnegotiation

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// downloadSectorValidation validates the requested download data sector. It will validates the following characteristics:
// 	1. validate the data sector size
//  2. validate the data sector length
//  3. if the merkle proof is requested, validate data sector offset and length against the segment size
func downloadSectorValidation(dataSector storage.DownloadRequestSector, merkleProof bool) error {
	// 1. validate data sector size
	if uint64(dataSector.Offset)+uint64(dataSector.Length) > storage.SectorSize {
		return fmt.Errorf("the download requested data sector is greater than the required data sector size")
	}

	// 2. validate the sector length
	if dataSector.Length == 0 {
		return fmt.Errorf("the download data sector's length cannot be 0")
	}

	// 3. validate sector against segment size if merkleProof is requested
	if merkleProof {
		if dataSector.Offset%storage.SegmentSize != 0 || dataSector.Length%storage.SegmentSize != 0 {
			return fmt.Errorf("the download data sector's length and offset must be multiples of the segment size when requesting a merkle proof")
		}
	}

	return nil
}

func downloadRequestPaybackValidation(validProofPaybacks, missedProofPaybacks []*big.Int, latestRevision types.StorageContractRevision) error {
	// validate the number of valid proof paybacks
	if len(validProofPaybacks) != len(latestRevision.NewValidProofOutputs) {
		return fmt.Errorf("the number of new valid proof paybacks does not match with the number of old valid proof paybacks")
	}

	// validate the number of missed proof paybacks
	if len(missedProofPaybacks) != len(latestRevision.NewMissedProofOutputs) {
		return fmt.Errorf("the number of new missed proof paybacks does not match with the number of old missed proof paybacks")
	}

	return nil
}

// downloadRevPaybackValidation validate the new download revision payback information
func downloadRevPaybackValidation(oldRev, newRev types.StorageContractRevision) error {
	// validate the number of paybacks
	if len(newRev.NewValidProofOutputs) != expectedValidProofPaybackCounts || len(newRev.NewMissedProofOutputs) != expectedMissedProofPaybackCounts {
		return errBadPaybackCount
	}

	// validate the host's payback addresses
	if newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Address != oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Address {
		return errValidPaybackAddress
	}

	if newRev.NewMissedProofOutputs[missedProofPaybackHostAddressIndex].Address != oldRev.NewMissedProofOutputs[missedProofPaybackHostAddressIndex].Address {
		return errMissedPaybackAddress
	}

	// the host's missed proof payback should not be changed
	if newRev.NewMissedProofOutputs[missedProofPaybackHostAddressIndex].Value.Cmp(oldRev.NewMissedProofOutputs[missedProofPaybackHostAddressIndex].Value) != 0 {
		return errMissedPaybackValue
	}

	// check the client's valid proof payback, it should be shrunk
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value) > 0 {
		return errValidPaybackValue
	}

	// check if client has the incentive to let the host fail. If so, client may purposely set larger missed payback value
	if newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value.Cmp(newRev.NewMissedProofOutputs[missedProofPaybackClientAddressIndex].Value) < 0 {
		return errClientHighMissedPayback
	}

	// check if the new revision has more valid proof payback
	if newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value) < 0 {
		return errHostLowValidPayback
	}

	return nil
}

// downloadNewRevAndOldRevValidation compares new revision and old revision non-changeable fields
func downloadNewRevAndOldRevValidation(oldRev, newRev types.StorageContractRevision) error {
	// compare new revision and old revision unchangeable fields
	if newRev.ParentID != oldRev.ParentID {
		return errBadParentID
	}

	if newRev.UnlockConditions.UnlockHash() != oldRev.UnlockConditions.UnlockHash() {
		return errBadUnlockConditions
	}

	if newRev.NewFileSize != oldRev.NewFileSize {
		return errBadFileSize
	}

	if newRev.NewFileMerkleRoot != oldRev.NewFileMerkleRoot {
		return errBadFileMerkleRoot
	}

	if newRev.NewWindowStart != oldRev.NewWindowStart {
		return errBadWindowStart
	}

	if newRev.NewWindowEnd != oldRev.NewWindowEnd {
		return errBadWindowEnd
	}

	if newRev.NewUnlockHash != oldRev.NewUnlockHash {
		return errBadUnlockHash
	}

	return nil
}

// revNumberAndWindowValidation compares the revision number between old revision and new revision. This method
// also checks if the client submit the revision after the revision deadline
func revNumberAndWindowValidation(np hostnegotiation.Protocol, oldRev, newRev types.StorageContractRevision) error {
	// validate revision number
	if newRev.NewRevisionNumber <= oldRev.NewRevisionNumber {
		return errBadRevNumber
	}

	// validate if the storage host received the revision after the revision deadline
	if oldRev.NewWindowStart-storagehost.PostponedExecutionBuffer <= np.GetBlockHeight() {
		return errLateRevision
	}

	return nil
}

// clientTokenTransferValidation validates if the client has transferred enough tokens
// for download operation and if storage host received enough token from the client
func clientTokenTransferValidation(np hostnegotiation.Protocol, dataSector storage.DownloadRequestSector, oldRev, newRev types.StorageContractRevision) error {
	// calculate amount of tokens transferred by storage client
	oldClientValidPayback := oldRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value
	newClientValidPayback := newRev.NewValidProofOutputs[validProofPaybackClientAddressIndex].Value
	tokenTrans := common.PtrBigInt(oldClientValidPayback).Sub(common.PtrBigInt(newClientValidPayback))

	// calculate the expected download cost and compare it with amount of token transferred by the
	// storage client
	expectedDownloadCost := calcExpectedDownloadCost(np, dataSector)
	if tokenTrans.Cmp(expectedDownloadCost) < 0 {
		return errLowDownloadPay
	}

	return hostTokenReceivedValidation(oldRev, newRev, tokenTrans)
}

// hostTokenReceivedValidation compares the amount of token sent by the storage client and amount of token
// received by the storage host. If they are not equivalent to each other, return error. Otherwise, return nil
func hostTokenReceivedValidation(oldRev, newRev types.StorageContractRevision, tokenTrans common.BigInt) error {
	// calculate amount of tokens received by the storage host
	oldHostValidPayback := oldRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value
	newHostValidPayback := newRev.NewValidProofOutputs[validProofPaybackHostAddressIndex].Value
	tokenReceived := common.PtrBigInt(newHostValidPayback).Sub(common.PtrBigInt(oldHostValidPayback))

	// host token received validation
	if tokenReceived.Cmp(tokenTrans) != 0 {
		return errHostTokenReceive
	}

	return nil
}
