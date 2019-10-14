// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiation

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// contractCreateOrRenewValidation will perform validation no matter the if the request is a contract
// create request or contract renew request
func contractCreateOrRenewValidation(clientPubKey, hostPubKey *ecdsa.PublicKey, sc types.StorageContract, blockHeight uint64, hostConfig storage.HostIntConfig) error {
	// get host address based on its public key
	hostAddress := crypto.PubkeyToAddress(*hostPubKey)

	// contract window validation
	if err := contractWindowValidation(sc.WindowStart, sc.WindowEnd, hostConfig.WindowSize, hostConfig.MaxDuration, blockHeight); err != nil {
		return err
	}

	// validate the contract unlock hash
	if err := contractUnlockHashValidation(clientPubKey, hostPubKey, sc.UnlockHash); err != nil {
		return err
	}

	// contract payback validation
	if err := contractPaybackValidation(hostAddress, sc.ValidProofOutputs, sc.MissedProofOutputs); err != nil {
		return err
	}

	return nil
}

// contractRenewValidation will perform validation to contract that needs to be renewed. The validation procedure
// include the following:
// 	1. file information validation
// 	2. contract payback validation
//  3. deposit and payout validation
func contractRenewValidation(np hostnegotiation.Protocol, oldContractID common.Hash, sc types.StorageContract, hostConfig storage.HostIntConfig, lockedStorageDeposit common.BigInt) error {
	// try to get storage responsibility first
	sr, err := np.GetStorageResponsibility(oldContractID)
	if err != nil {
		return err
	}

	// file info validation
	if err := contractRenewFileInfoValidation(sr, sc); err != nil {
		return err
	}

	// deposit and payout validation
	if err := depositAndPayoutValidation(hostConfig, sc, sr, lockedStorageDeposit); err != nil {
		return err
	}

	return nil
}

// contractCreateValidation validates the storage contract when the request is contract create
func contractCreateValidation(sc types.StorageContract, hostConfig storage.HostIntConfig, lockedStorageDeposit common.BigInt) error {
	// validate the contract create file information
	if err := contractCreateFileInfoValidation(sc.FileSize, sc.FileMerkleRoot); err != nil {
		return err
	}

	// validate the contract create host payback
	if err := contractCreatePaybackValidation(sc, hostConfig.ContractPrice); err != nil {
		return err
	}

	// validate the storage host deposit
	if err := contractCreateDepositValidation(lockedStorageDeposit, sc.ValidProofOutputs, hostConfig); err != nil {
		return err
	}

	return nil
}

// contractCreateFileInfoValidation validates the contract file information when creating
// the storage contract
func contractCreateFileInfoValidation(fileSize uint64, fileMerkleRoot common.Hash) error {
	// in terms of contract create, the file size should be 0
	if fileSize != 0 {
		return errBadFileSizeCreate
	}

	// in terms of contract root, the merkle root should be empty
	if fileMerkleRoot != (common.Hash{}) {
		return errBadFileRootCreate
	}

	return nil
}

// contractCreatePaybackValidation validates the contract payback information when creating the
// storage contract for both validProofPayback and missedProofPayback
func contractCreatePaybackValidation(sc types.StorageContract, contractPrice common.BigInt) error {
	// validate host's validProof payback and missedProof payback, they should be equivalent
	// when creating a new storage contract
	if sc.ValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(sc.MissedProofOutputs[missedProofPaybackHostAddressIndex].Value) != 0 {
		return errPayoutMismatch
	}

	// host valid payout should at least contain the contract fee when creating a new storage contract
	if sc.ValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(contractPrice.BigIntPtr()) < 0 {
		return errLowHostValidPayout
	}

	return nil
}

// contractCreateDepositValidation validates the contract create deposit information when creating
// the storage contract
func contractCreateDepositValidation(lockedStorageDeposit common.BigInt, validProofOutputs []types.DxcoinCharge, hostConfig storage.HostIntConfig) error {
	// get the host valid proof payback
	hostValidProofPayback := validProofOutputs[validProofPaybackHostAddressIndex].Value

	// contractHostDeposit means money contained in the storage host validProofOutput without
	// contract price
	contractHostDeposit := common.PtrBigInt(hostValidProofPayback).Sub(hostConfig.ContractPrice)

	// check if the deposit is larger than the max deposit that storage host is allowed
	if contractHostDeposit.Cmp(hostConfig.MaxDeposit) > 0 {
		return errExceedDeposit
	}

	// validate the locked storage deposit
	if lockedStorageDeposit.Add(contractHostDeposit).Cmp(hostConfig.DepositBudget) > 0 {
		return errReachBudget
	}

	return nil
}

// contractRenewFileInfoValidation validates the file information when renewing the storage contract
func contractRenewFileInfoValidation(sr storagehost.StorageResponsibility, storageContract types.StorageContract) error {
	// validate the file size
	if storageContract.FileSize != sr.FileSize() {
		return errBadRenewFileSize
	}

	// validate the file merkle root
	if storageContract.FileMerkleRoot != sr.MerkleRoot() {
		return errBadRenewFileRoot
	}

	return nil
}

// depositAndPayoutValidation is used to validate if the storage host
// has enough tokens. This validation will only be proceed in contract renew
// not contract create
func depositAndPayoutValidation(hostConfig storage.HostIntConfig, sc types.StorageContract, sr storagehost.StorageResponsibility, lockedStorageDeposit common.BigInt) error {
	// validate the storage host deposit
	baseContractRenewPrice := renewBaseCost(sr, hostConfig.StoragePrice, sc.WindowEnd, sc.FileSize)
	hostPriceShare := common.PtrBigInt(sc.ValidProofOutputs[1].Value)
	expectedDeposit := hostPriceShare.Sub(hostConfig.ContractPrice).Sub(baseContractRenewPrice)
	if expectedDeposit.Cmp(hostConfig.MaxDeposit) > 0 {
		return errBadMaxDeposit
	}

	// validate the host valid proof payout
	baseDeposit := renewBaseDeposit(sr, sc.WindowEnd, sc.FileSize, hostConfig.Deposit)
	totalPayout := baseContractRenewPrice.Add(baseDeposit)
	if sc.ValidProofOutputs[1].Value.Cmp(totalPayout.BigIntPtr()) < 0 {
		return errLowHostExpectedValidPayout
	}

	// validate the deposit budget
	if lockedStorageDeposit.Add(expectedDeposit).Cmp(hostConfig.DepositBudget) > 0 {
		return errExpectedDepositReachBudget
	}

	// validate the host missed proof payout
	expectedHostMissedPayout := common.PtrBigInt(sc.ValidProofOutputs[1].Value).Sub(baseContractRenewPrice).Sub(baseDeposit)
	if sc.MissedProofOutputs[1].Value.Cmp(expectedHostMissedPayout.BigIntPtr()) < 0 {
		return errLowMissedPayout
	}

	return nil
}

// contractWindowValidation validates the contract's window start and window end information
func contractWindowValidation(windowStart, windowEnd, windowSize, maxDuration, blockHeight uint64) error {
	// WindowStart must be at least postponedExecutionBuffer blocks in the future
	if windowStart <= blockHeight+storagehost.PostponedExecutionBuffer {
		return errEarlyWindow
	}

	// windowEnd must be at least settings.WindowSize blocks after WindowStart
	if windowEnd < windowStart+windowSize {
		return errSmallWindowSize
	}

	// windowStart must not be more than maxDuration blocks in the future
	if windowStart > blockHeight+maxDuration {
		return errLongDuration
	}

	return nil
}

// contractUnlockHashValidation validates the contract's unlock hash
func contractUnlockHashValidation(clientPubKey, hostPubKey *ecdsa.PublicKey, unlockHash common.Hash) error {
	// calculate the expected unlockHash
	expectedUnlockHash := types.UnlockConditions{
		PaymentAddresses: []common.Address{
			crypto.PubkeyToAddress(*clientPubKey),
			crypto.PubkeyToAddress(*hostPubKey),
		},
		SignaturesRequired: contractRequiredSignatures,
	}.UnlockHash()

	// unlockHash validation
	if unlockHash != expectedUnlockHash {
		return errBadUnlockHash
	}

	return nil
}

// contractPaybackValidation validates both contractValidProof payback and contractMissedProof payback
func contractPaybackValidation(hostAddress common.Address, validProofPayback, missedProofPayback []types.DxcoinCharge) error {
	// validate the amount of valid proof payback
	if len(validProofPayback) != expectedValidProofPaybackCounts {
		return errBadValidProofPaybackCount
	}

	// validate the amount of missed proof payback
	if len(missedProofPayback) != expectedMissedProofPaybackCounts {
		return errBadMissedProofPaybackCount
	}

	// validate the validProofPayback host address
	if validProofPayback[validProofPaybackHostAddressIndex].Address != hostAddress {
		return errBadValidProofAddress
	}

	// validate the missedProofPayback host address
	if missedProofPayback[missedProofPaybackHostAddressIndex].Address != hostAddress {
		return errBadMissedProofAddress
	}

	return nil
}
