// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/crypto/merkle"

	"github.com/DxChainNetwork/godx/accounts"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

// contractValidation will validate the storage contract sent by the storage client
func contractValidation(np NegotiationProtocol, req storage.ContractCreateRequest, sc types.StorageContract, hostPubKey, clientPubKey *ecdsa.PublicKey) error {
	// get needed data
	blockHeight := np.GetBlockHeight()
	hostConfig := np.GetHostConfig()
	lockedStorageDeposit := np.GetFinancialMetrics().LockedStorageDeposit

	// a part of validation that is needed no matter the validation is for contract create
	// or contract renew
	if err := contractCreateOrRenewValidation(clientPubKey, hostPubKey, sc, blockHeight, hostConfig); err != nil {
		return fmt.Errorf("contract create and renew validation failed: %s", err.Error())
	}

	// check if the contract is renewing
	if req.Renew {
		return contractRenewValidation(np, req.OldContractID, sc, hostConfig, lockedStorageDeposit)
	}

	// contract create validation
	return contractCreateValidation(sc, hostConfig, lockedStorageDeposit)
}

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

// hostAddressValidation is used to validate the host address obtained from the storage contract sent
// by the storage host
func hostAddressValidation(hostAddress common.Address, nd *contractNegotiationData, np NegotiationProtocol) error {
	// trying to get the wallet based on the hostAddress parsed from the
	account := accounts.Account{Address: hostAddress}
	wallet, err := np.FindWallet(account)
	if err != nil {
		return fmt.Errorf("failed to get the wallet based on the address provided in the storage contract: %s", err.Error())
	}

	// update the negotiation data, account and wallet
	nd.account = account
	nd.wallet = wallet

	return nil
}

// hostBalanceValidation validates the host balance to see if the host is able to pay
// the deposit
func hostBalanceValidation(np NegotiationProtocol, hostAddress common.Address, hostDeposit *big.Int) error {
	// get the stateDB
	stateDB, err := np.GetStateDB()
	if err != nil {
		err = fmt.Errorf("failed to get the stateDb: %s", err.Error())
		return common.ErrCompose(err, storage.ErrHostNegotiate)
	}

	// check storage host balance
	if stateDB.GetBalance(hostAddress).Cmp(hostDeposit) < 0 {
		err := fmt.Errorf("storage host has insufficient balance")
		return common.ErrCompose(err, storage.ErrHostNegotiate)
	}

	return nil
}

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

// contractRenewValidation will perform validation to contract that needs to be renewed. The validation procedure
// include the following:
// 	1. file information validation
// 	2. contract payback validation
//  3. deposit and payout validation
func contractRenewValidation(np NegotiationProtocol, oldContractID common.Hash, sc types.StorageContract, hostConfig storage.HostIntConfig, lockedStorageDeposit common.BigInt) error {
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

func contractCreateOrRenewValidation(clientPubKey, hostPubKey *ecdsa.PublicKey, sc types.StorageContract, blockHeight uint64, hostConfig storage.HostIntConfig) error {
	// get host address based on its public key
	hostAddress := crypto.PubkeyToAddress(*hostPubKey)

	// contract window validation
	if err := contractWindowValidation(sc.WindowStart, sc.WindowEnd, hostConfig.WindowSize, hostConfig.MaxDuration, blockHeight); err != nil {
		return fmt.Errorf("host failed to validate the contract window: %s", err.Error())
	}

	// validate the contract unlock hash
	if err := contractUnlockHashValidation(clientPubKey, hostPubKey, sc.UnlockHash); err != nil {
		return fmt.Errorf("host failed to validate the contract unlock hash: %s", err.Error())
	}

	// contract payback validation
	if err := contractPaybackValidation(hostAddress, sc.ValidProofOutputs, sc.MissedProofOutputs); err != nil {
		return fmt.Errorf("host failed to validate the contract payback: %s", err.Error())
	}

	return nil
}

func contractRenewFileInfoValidation(sr storagehost.StorageResponsibility, storageContract types.StorageContract) error {
	// validate the file size
	if storageContract.FileSize != sr.FileSize() {
		return fmt.Errorf("contract renew fileInfo validation failed: file size form storage contract does not match with the one stored in the storage responsibility")
	}

	// validate the file merkle root
	if storageContract.FileMerkleRoot != sr.MerkleRoot() {
		return fmt.Errorf("contract renew fileinfo validation failed: file merkle root from storage contract does not match with the one stored in the storage responsibility")
	}

	return nil
}

func contractCreateFileInfoValidation(fileSize uint64, fileMerkleRoot common.Hash) error {
	// in terms of contract create, the file size should be 0
	if fileSize != 0 {
		return fmt.Errorf("contract create file validation failed: the file size should be 0")
	}

	// in terms of contract root, the merkle root should be empty
	if fileMerkleRoot != (common.Hash{}) {
		return fmt.Errorf("contract create file validation failed: the file merkle root should be empty")
	}

	return nil
}

func contractWindowValidation(windowStart, windowEnd, windowSize, maxDuration, blockHeight uint64) error {
	// WindowStart must be at least postponedExecutionBuffer blocks in the future
	if windowStart <= blockHeight+storagehost.PostponedExecutionBuffer {
		return fmt.Errorf("contract window validation failed: window starts too soon")
	}

	// windowEnd must be at least settings.WindowSize blocks after WindowStart
	if windowEnd < windowStart+windowSize {
		return fmt.Errorf("contract window validation failed: small window size")
	}

	// windowStart must not be more than maxDuration blocks in the future
	if windowStart > blockHeight+maxDuration {
		return fmt.Errorf("contract window valiation failed: client proposd a file contract with long duration")
	}

	return nil
}

func contractPaybackValidation(hostAddress common.Address, validProofPayback, missedProofPayback []types.DxcoinCharge) error {
	// validate the amount of valid proof payback
	if len(validProofPayback) != expectedValidProofPaybackCounts {
		return fmt.Errorf("contract payback validation failed: unexpected amount of validation proof payback")
	}

	// validate the amount of missed proof payback
	if len(missedProofPayback) != expectedMissedProofPaybackCounts {
		return fmt.Errorf("contract payback validation failed: unexpected amount of missed proof payback")
	}

	// validate the validProofPayback host address
	if validProofPayback[validProofPaybackHostAddressIndex].Address != hostAddress {
		return fmt.Errorf("contract payback validation failed: host address contained in validProofPayback is not correct")
	}

	// validate the missedProofPayback host address
	if missedProofPayback[missedProofPaybackHostAddressIndex].Address != hostAddress {
		return fmt.Errorf("contract payback validation failed: host address contained in missedProofPayback is correct")
	}

	return nil
}

func contractCreatePaybackValidation(sc types.StorageContract, contractPrice common.BigInt) error {
	// validate host's validProof payback and missedProof payback, they should be equivalent
	// when creating a new storage contract
	if sc.ValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(sc.MissedProofOutputs[missedProofPaybackHostAddressIndex].Value) != 0 {
		return fmt.Errorf("contract create payback validation failed: host valid payout is not equal to host missed payout")
	}

	// host valid payout should at least contain the contract fee when creating a new storage contract
	if sc.ValidProofOutputs[validProofPaybackHostAddressIndex].Value.Cmp(contractPrice.BigIntPtr()) < 0 {
		return fmt.Errorf("contract create payback validationf ailed: host validpayout is too low")
	}

	return nil
}

// depositAndPayoutValidation is used to validate if the storage host
// has enough money. This validation will only be proceed in contract renew
// not contract create
func depositAndPayoutValidation(hostConfig storage.HostIntConfig, sc types.StorageContract, sr storagehost.StorageResponsibility, lockedStorageDeposit common.BigInt) error {
	// validate the storage host deposit
	baseContractRenewPrice := renewBasePrice(sr, hostConfig.StoragePrice, sc.WindowEnd, sc.FileSize)
	hostPriceShare := common.PtrBigInt(sc.ValidProofOutputs[1].Value)
	expectedDeposit := hostPriceShare.Sub(hostConfig.ContractPrice).Sub(baseContractRenewPrice)
	if expectedDeposit.Cmp(hostConfig.MaxDeposit) > 0 {
		return fmt.Errorf("deposit and payout validation failed: client expected host to pay more deposit than the max allowed deposit")
	}

	// validate the host valid proof payout
	baseDeposit := renewBaseDeposit(sr, sc.WindowEnd, sc.FileSize, hostConfig.Deposit)
	totalPayout := baseContractRenewPrice.Add(baseDeposit)
	if sc.ValidProofOutputs[1].Value.Cmp(totalPayout.BigIntPtr()) < 0 {
		return fmt.Errorf("deposit and payout validation failed: host rejected the contract for low payling host valid payout")
	}

	// validate the deposit budget
	if lockedStorageDeposit.Add(expectedDeposit).Cmp(hostConfig.DepositBudget) > 0 {
		return fmt.Errorf("deposit and payout validation failed: host has reached to its deposit budget and cannot accept any contract")
	}

	// validate the host missed proof payout
	expectedHostMissedPayout := common.PtrBigInt(sc.ValidProofOutputs[1].Value).Sub(baseContractRenewPrice).Sub(baseDeposit)
	if sc.MissedProofOutputs[1].Value.Cmp(expectedHostMissedPayout.BigIntPtr()) < 0 {
		return fmt.Errorf("deposit and payout validation failed: host rejected the contract for low paying missed payout")
	}

	return nil
}

func contractCreateDepositValidation(lockedStorageDeposit common.BigInt, validProofOutputs []types.DxcoinCharge, hostConfig storage.HostIntConfig) error {
	// get the host valid proof payback
	hostValidProofPayback := validProofOutputs[validProofPaybackHostAddressIndex].Value

	// contractHostDeposit means money contained in the storage host validProofOutput without
	// contract price
	contractHostDeposit := common.PtrBigInt(hostValidProofPayback).Sub(hostConfig.ContractPrice)

	// check if the deposit is larger than the max deposit that storage host is allowed
	if contractHostDeposit.Cmp(hostConfig.MaxDeposit) > 0 {
		return fmt.Errorf("contract create deposit validatioin failed: contract host deposit exceed the max deposit that host is specified")
	}

	// validate the locked storage deposit
	if lockedStorageDeposit.Add(contractHostDeposit).Cmp(hostConfig.DepositBudget) > 0 {
		return fmt.Errorf("contract create deposit validation failed: storage host has reached its deposit budget and cannot accept any new contract")
	}

	return nil
}

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
		return fmt.Errorf("contract unlock hash validation failed: storage host failed to validate the unlock hash")
	}

	return nil
}
