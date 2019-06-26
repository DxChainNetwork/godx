// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

// checkForContractRenew will loop through all active contracts and filter out those needs to be renewed.
// There are two types of contract needs to be renewed
// 		1. contracts that are about to expired. they need to be renewed
// 		2. contracts that have insufficient amount of funding, meaning the contract is about to be
// 		   marked as not good for data uploading
func (cm *ContractManager) checkForContractRenew(rentPayment storage.RentPayment) (closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {

	cm.log.Debug("Contract renew check started")

	cm.lock.RLock()
	currentBlockHeight := cm.blockHeight
	log.Error("[ContractManager]currentBlockHeight", "height", cm.blockHeight)
	cm.lock.RUnlock()

	// loop through all active contracts, get the closeToExpireRenews and insufficientFundingRenews
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// validate the storage host for the contract, check if the host exists or get filtered
		host, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		if !exists || host.Filtered {
			continue
		}

		a, _ := json.Marshal(contract)
		log.Error("CONTRACT InfoXXXXXX", "contract", string(a))

		// verify if the contract is good for renew
		if !contract.Status.RenewAbility {
			continue
		}

		// for contract that is about to expire, it will be added to the priorityRenews
		// calculate the renewCostEstimation and update the priorityRenews
		if currentBlockHeight+rentPayment.RenewWindow >= contract.EndHeight {
			b, _ := json.Marshal(rentPayment)
			log.Error("RentpamentOOOOOO", "info", string(b))

			estimateContractRenewCost := cm.renewCostEstimation(host, contract, currentBlockHeight, rentPayment)
			closeToExpireRenews = append(closeToExpireRenews, contractRenewRecord{
				id:   contract.ID,
				cost: estimateContractRenewCost,
			})
			continue
		}

		// for those contracts has insufficient funding, they should be renewed because otherwise
		// after a while, they will be marked as not good for upload
		sectorStorageCost := host.StoragePrice.MultUint64(contractset.SectorSize * rentPayment.Period)
		sectorUploadBandwidthCost := host.UploadBandwidthPrice.MultUint64(contractset.SectorSize)
		sectorDownloadBandwidthCost := host.DownloadBandwidthPrice.MultUint64(contractset.SectorSize)
		totalSectorCost := sectorUploadBandwidthCost.Add(sectorDownloadBandwidthCost).Add(sectorStorageCost)

		remainingBalancePercentage := contract.ContractBalance.Div(contract.TotalCost).Float64()

		if contract.ContractBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minContractPaymentRenewalThreshold {
			log.Error("insufficientFundingRenewsXXXXXXXX", "ContractBalance", contract.ContractBalance, "totalSectorCost", totalSectorCost, "remainingBalancePercentage", remainingBalancePercentage)
			insufficientFundingRenews = append(insufficientFundingRenews, contractRenewRecord{
				id:   contract.ID,
				cost: contract.TotalCost.MultUint64(2),
			})
		}
	}

	return
}

// resetFailedRenews will update the failedRenewCount list, which only includes the failedRenewCount
// in the current renew lists
func (cm *ContractManager) resetFailedRenews(closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	filteredFailedRenews := make(map[storage.ContractID]uint64)

	// loop through the closeToExpireRenews, get the failedRenewCount
	for _, renewRecord := range closeToExpireRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenewCount[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenewCount[contractID]
		}
	}

	// loop through the insufficientFundingRenews, get the failed renews
	for _, renewRecord := range insufficientFundingRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenewCount[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenewCount[contractID]
		}
	}

	// reset the failedRenewCount
	cm.failedRenewCount = filteredFailedRenews
}

// prepareContractRenew will loop through all record in the renewRecords and start to renew
// each contract. Before contract renewing get started, the fund will be validated first.
func (cm *ContractManager) prepareContractRenew(renewRecords []contractRenewRecord, clientRemainingFund common.BigInt, rentPayment storage.RentPayment) (remainingFund common.BigInt, terminate bool) {

	cm.log.Debug("Prepare to renew the contract")

	// get the data needed
	cm.lock.RLock()
	currentPeriod := cm.currentPeriod
	contractEndHeight := cm.currentPeriod + rentPayment.Period + rentPayment.RenewWindow
	cm.lock.RUnlock()

	// initialize remaining fund first
	remainingFund = clientRemainingFund

	// loop through all contracts that need to be renewed, and prepare to renew the contract
	for _, record := range renewRecords {
		// verify that the cost needed for contract renew does not exceed the clientRemainingFund
		if clientRemainingFund.Cmp(record.cost) < 0 {
			cm.log.Debug("client does not have enough fund to renew the contract", "contractID", record.id, "cost", record.cost)
			continue
		}

		// renew the contract, get the spending for the renew
		renewCost, err := cm.contractRenewStart(record, currentPeriod, rentPayment, contractEndHeight)
		if err != nil {
			cm.log.Error("contract renew failed", "contractID", record.id, "err", err.Error())
		}

		// update the remaining fund
		remainingFund = clientRemainingFund.Sub(renewCost)

		// check maintenance termination
		if terminate = cm.checkMaintenanceTermination(); terminate {
			return
		}
	}

	return
}

// contractRenewStart will start to perform contract renew operation
// 		1. before contract renew, validate the contract first
// 		2. renew the contract
// 		3. if the renew failed, handle the failed situation
//   	4. otherwise, update the contract manager
func (cm *ContractManager) contractRenewStart(record contractRenewRecord, currentPeriod uint64, rentPayment storage.RentPayment, contractEndHeight uint64) (renewCost common.BigInt, err error) {
	// get the information needed
	renewContractID := record.id
	renewContractCost := record.cost

	// mark the oldContract as renewing, and remove it at the end
	cm.lock.Lock()
	cm.renewing[renewContractID] = true
	cm.lock.Unlock()

	defer func() {
		cm.lock.Lock()
		delete(cm.renewing, renewContractID)
		cm.lock.Unlock()
	}()

	// if the contract is revising, return error directly
	if !cm.b.IsRevisionSessionDone(renewContractID) {
		renewCost = common.BigInt0
		err = fmt.Errorf("the contract is revising, cannot be renewed")
		return
	}

	// acquire the oldContract (contract that is about to be renewed)
	oldContract, exists := cm.activeContracts.Acquire(renewContractID)
	if !exists {
		renewCost = common.BigInt0
		err = fmt.Errorf("the oldContract that is trying to be renewed with id %v no longer exists", renewContractID)
		return
	}

	// 1. get the oldContract status and check its renewAbility, oldContract validation
	stats, exists := cm.retrieveContractStatus(renewContractID)
	if !exists || !stats.RenewAbility {
		if err := cm.activeContracts.Return(oldContract); err != nil {
			cm.log.Warn("during the oldContract renew process, the oldContract cannot be returned because it has been deleted already")
		}
		renewCost = common.BigInt0
		err = fmt.Errorf("oldContract with id %v is marked as unable to be renewed", renewContractID)
		return
	}

	// 2. oldContract renew
	renewedContract, renewErr := cm.renew(oldContract, rentPayment, renewContractCost, contractEndHeight)

	// 3. handle the failed renews
	if renewErr != nil {
		if err := cm.activeContracts.Return(oldContract); err != nil {
			cm.log.Warn("during the handle oldContract renew failed process, the oldContract cannot be returned because it has been deleted already")
		}
		renewCost = common.BigInt0
		err = cm.handleRenewFailed(oldContract, renewErr, rentPayment, stats)
		return
	}

	// at this point, the contract has been renewed successfully
	renewCost = renewContractCost

	// 4. update the oldContract manager
	renewedContractStatus := storage.ContractStatus{
		UploadAbility: true,
		RenewAbility:  true,
		Canceled:      false,
	}
	if err = cm.updateContractStatus(renewedContract.ID, renewedContractStatus); err != nil {
		// renew succeed, but status update failed
		cm.log.Warn(fmt.Sprintf("failed to update the renewed oldContract status: %s", err.Error()))
		if err = cm.activeContracts.Return(oldContract); err != nil {
			cm.log.Warn("during the updating renewed oldContract failed process, the oldContract cannot be returned because it has been deleted already")
			err = nil
			return
		}
	}

	// update the old oldContract status
	stats.RenewAbility = false
	stats.UploadAbility = false
	stats.Canceled = true
	if err = oldContract.UpdateStatus(stats); err != nil {
		cm.log.Warn("failed to update the old oldContract (before renew) status")
		if err = cm.activeContracts.Return(oldContract); err != nil {
			cm.log.Warn("during the old oldContract status update process, the oldContract cannot be returned because it has been deleted already")
			err = nil
			return
		}
	}

	// update the renewedFrom, renewedTo, expiredContract field
	cm.lock.Lock()
	cm.renewedFrom[renewedContract.ID] = oldContract.Metadata().ID
	cm.renewedTo[oldContract.Metadata().ID] = renewedContract.ID
	cm.expiredContracts[oldContract.Metadata().ID] = oldContract.Metadata()
	cm.lock.Unlock()

	// save the information persistently
	if err = cm.saveSettings(); err != nil {
		cm.log.Error("failed to save the settings persistently", "err", err.Error())
	}

	// delete the old oldContract from the active oldContract list
	if err = cm.activeContracts.Delete(oldContract); err != nil {
		cm.log.Error("failed to delete the contract from the active oldContract list after renew", "err", err.Error())
	}

	err = nil
	return
}

// renew will start to perform the contract renew operation:
// 		1. contract renewAbility validation
// 		2. storage host validation
// 		3. form the contract renew needed params
// 		4. perform the contract renew operation
// 		5. update the storage host to contract id mapping
func (cm *ContractManager) renew(renewContract *contractset.Contract, rentPayment storage.RentPayment, contractFund common.BigInt, contractEndHeight uint64) (renewedContract storage.ContractMetaData, err error) {
	// 1. contract renewAbility validation
	contractMeta := renewContract.Metadata()
	status, exists := cm.retrieveContractStatus(contractMeta.ID)
	if !exists || !status.RenewAbility {
		err = fmt.Errorf("the contract is not able to be renewed, the renewAbility is marked as false")
		return
	}

	// 2. storage host validation
	host, exists := cm.hostManager.RetrieveHostInfo(contractMeta.EnodeID)

	if !exists {
		err = fmt.Errorf("the storage host recorded in the contract that needs to be renewed, cannot be found")
		return
	} else if host.Filtered {
		err = fmt.Errorf("the storage host has been filtered")
		return
	} else if host.StoragePrice.Cmp(maxHostStoragePrice) > 0 {
		err = fmt.Errorf("the storage price exceed the maximum storage price alloweed")
		return
	} else if host.MaxDuration < rentPayment.Period {
		err = fmt.Errorf("the max duration cannot be smaller than the storage contract period")
		return
	}

	// validate the storage host max deposit
	if host.MaxDeposit.Cmp(maxHostDeposit) > 0 {
		host.MaxDeposit = maxHostDeposit
	}

	// 3. form the contract renew needed params
	// The reason to get the newest blockHeight here is that during the checking time period
	// many blocks may be generated already, which is unfair to the storage client.
	cm.lock.RLock()
	startHeight := cm.blockHeight
	cm.lock.RUnlock()

	// try to get the clientPaymentAddress. If failed, return error directly and set the contract creation cost
	// to be zero
	var clientPaymentAddress common.Address
	if clientPaymentAddress, err = cm.b.GetPaymentAddress(); err != nil {
		err = fmt.Errorf("failed to create the contract with host: %v, failed to get the clientPayment address: %s", host.EnodeID, err.Error())
		return
	}

	// form the contract parameters
	params := storage.ContractParams{
		Allowance:            rentPayment,
		HostEnodeUrl:         host.EnodeURL,
		Funding:              contractFund,
		StartHeight:          startHeight,
		EndHeight:            contractEndHeight,
		ClientPaymentAddress: clientPaymentAddress,
		Host:                 host,
	}

	// 4. contract renew
	if renewedContract, err = cm.ContractRenew(renewContract, params); err != nil {
		return
	}

	// 5. update the storage host to contract id mapping
	cm.lock.Lock()
	cm.hostToContract[renewedContract.EnodeID] = renewedContract.ID
	cm.lock.Unlock()

	return
}

// handleRenewFailed will handle the failed contract renews.
// 		1. check if the error is caused by storage host, if so, increase the failed renew count
// 		2. if the amount of renew fails exceed a limit or it is already passed the second half of renew window,
// 		meaning the contract needs to be replaced, mark the contract as canceled
// 		3. return the error message
func (cm *ContractManager) handleRenewFailed(failedContract *contractset.Contract, renewError error, rentPayment storage.RentPayment, contractStatus storage.ContractStatus) (err error) {
	// if renew failed is caused by the storage host, update the the failedRenewsCount
	if common.ErrContains(renewError, ErrHostFault) {
		cm.lock.Lock()
		cm.failedRenewCount[failedContract.Metadata().ID]++
		cm.lock.Unlock()
	}

	// get the number of failed renews, to check if the contract needs to be replaced
	// get the newest block height as well
	cm.lock.RLock()
	numFailed, _ := cm.failedRenewCount[failedContract.Metadata().ID]
	blockHeight := cm.blockHeight
	cm.lock.RUnlock()

	secondHalfRenewWindow := blockHeight+rentPayment.RenewWindow/2 >= failedContract.Metadata().EndHeight
	contractReplace := numFailed >= consecutiveRenewFailsBeforeReplacement

	// if the contract has been failed before, passed the second half renew window, and need replacement
	// mark the contract that is trying to be renewed as canceled
	if secondHalfRenewWindow && contractReplace {
		contractStatus.UploadAbility = false
		contractStatus.RenewAbility = false
		contractStatus.Canceled = true
		if err := failedContract.UpdateStatus(contractStatus); err != nil {
			cm.log.Warn(fmt.Sprintf("failed to update the contract status during renew failed handling: %s", err.Error()))
		}

		err = fmt.Errorf("marked the contract %v as canceled due to the large amount of renew fails: %s", failedContract.Metadata().ID, renewError.Error())
		return
	}

	// otherwise, do nothing, a renew attempt to the same contract will be performed again
	err = fmt.Errorf("failed to renew the contract: %s", renewError.Error())
	return
}

//ContractRenew renew transaction initiated by the storage client
func (cm *ContractManager) ContractRenew(oldContract *contractset.Contract, params storage.ContractParams) (md storage.ContractMetaData, err error) {

	contract := oldContract.Header()
	lastRev := contract.LatestContractRevision

	// Extract vars from params, for convenience
	allowance, funding, startHeight, endHeight, host := params.Allowance, params.Funding, params.StartHeight, params.EndHeight, params.Host

	var basePrice, baseCollateral common.BigInt
	if endHeight+host.WindowSize > lastRev.NewWindowEnd {
		timeExtension := uint64(endHeight+host.WindowSize) - lastRev.NewWindowEnd
		basePrice = host.StoragePrice.Mult(common.NewBigIntUint64(lastRev.NewFileSize)).Mult(common.NewBigIntUint64(timeExtension))
		baseCollateral = host.Deposit.Mult(common.NewBigIntUint64(lastRev.NewFileSize)).Mult(common.NewBigIntUint64(timeExtension))
	}

	// Calculate the payouts for the client, host, and whole contract
	period := endHeight - startHeight
	expectedStorage := allowance.ExpectedStorage / allowance.StorageHosts
	clientPayout, hostPayout, hostCollateral, err := ClientPayoutsPreTax(host, funding, basePrice, baseCollateral, period, expectedStorage)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// check for negative currency
	if hostCollateral.Cmp(baseCollateral) < 0 {
		baseCollateral = hostCollateral
	}

	//Calculate the account address of the client
	clientAddr := lastRev.NewValidProofOutputs[0].Address
	//Calculate the account address of the host
	hostAddr := lastRev.NewValidProofOutputs[1].Address
	// Create storage contract
	storageContract := types.StorageContract{
		FileSize:         lastRev.NewFileSize,
		FileMerkleRoot:   lastRev.NewFileMerkleRoot, // no proof possible without data
		WindowStart:      endHeight,
		WindowEnd:        endHeight + host.WindowSize,
		ClientCollateral: types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: clientPayout.BigIntPtr(), Address: clientAddr}},
		HostCollateral:   types.DxcoinCollateral{DxcoinCharge: types.DxcoinCharge{Value: hostPayout.BigIntPtr(), Address: hostAddr}},
		UnlockHash:       lastRev.NewUnlockHash,
		RevisionNumber:   0,
		ValidProofOutputs: []types.DxcoinCharge{
			// Deposit is returned to client
			{Value: clientPayout.BigIntPtr(), Address: clientAddr},
			// Deposit is returned to host
			{Value: hostPayout.BigIntPtr(), Address: hostAddr},
		},
		MissedProofOutputs: []types.DxcoinCharge{
			{Value: clientPayout.BigIntPtr(), Address: clientAddr},
			{Value: hostPayout.Sub(baseCollateral).Add(host.ContractPrice).BigIntPtr(), Address: hostAddr},
		},
	}

	// Increase Successful/Failed interactions accordingly
	defer func() {
		if err != nil {
			cm.hostManager.IncrementFailedInteractions(contract.EnodeID)
			err = common.ErrExtend(err, ErrHostFault)
		} else {
			cm.hostManager.IncrementSuccessfulInteractions(contract.EnodeID)
		}
	}()

	account := accounts.Account{Address: clientAddr}
	wallet, err := cm.b.AccountManager().Find(account)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("find client account error", err)
	}

	// Setup connection with storage host
	session, err := cm.b.SetupConnection(host.EnodeURL)
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("setup connection with host failed", err)
	}
	defer func() {
		cm.log.Error("contract renew: disconnect session")
		cm.b.Disconnect(session, host.EnodeURL)
	}()

	clientContractSign, err := wallet.SignHash(account, storageContract.RLPHash().Bytes())
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("contract sign by client failed", err)
	}

	// Send the ContractCreate request
	req := storage.ContractCreateRequest{
		StorageContract: storageContract,
		Sign:            clientContractSign,
		Renew:           true,
		OldContractID:   lastRev.ParentID,
	}

	if err := session.SendStorageContractCreation(req); err != nil {
		return storage.ContractMetaData{}, err
	}

	var hostSign []byte
	msg, err := session.ReadMsg()
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return storage.ContractMetaData{}, negotiationErr
	}

	if err := msg.Decode(&hostSign); err != nil {
		return storage.ContractMetaData{}, err
	}

	storageContract.Signatures = [][]byte{clientContractSign, hostSign}

	// Assemble init revision and sign it
	storageContractRevision := types.StorageContractRevision{
		ParentID:              storageContract.RLPHash(),
		UnlockConditions:      lastRev.UnlockConditions,
		NewRevisionNumber:     1,
		NewFileSize:           storageContract.FileSize,
		NewFileMerkleRoot:     storageContract.FileMerkleRoot,
		NewWindowStart:        storageContract.WindowStart,
		NewWindowEnd:          storageContract.WindowEnd,
		NewValidProofOutputs:  storageContract.ValidProofOutputs,
		NewMissedProofOutputs: storageContract.MissedProofOutputs,
		NewUnlockHash:         storageContract.UnlockHash,
	}

	clientRevisionSign, err := wallet.SignHash(account, storageContractRevision.RLPHash().Bytes())
	if err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("client sign revision error", err)
	}
	storageContractRevision.Signatures = [][]byte{clientRevisionSign}

	if err := session.SendStorageContractCreationClientRevisionSign(clientRevisionSign); err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("send revision sign by client error", err)
	}

	var hostRevisionSign []byte
	msg, err = session.ReadMsg()
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// if host send some negotiation error, client should handler it
	if msg.Code == storage.NegotiationErrorMsg {
		var negotiationErr error
		msg.Decode(&negotiationErr)
		return storage.ContractMetaData{}, negotiationErr
	}

	if err := msg.Decode(&hostRevisionSign); err != nil {
		return storage.ContractMetaData{}, err
	}

	scBytes, err := rlp.EncodeToBytes(storageContract)
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	if _, err := cm.b.SendStorageContractCreateTx(clientAddr, scBytes); err != nil {
		return storage.ContractMetaData{}, storagehost.ExtendErr("Send storage contract creation transaction error", err)
	}

	// wrap some information about this contract
	header := contractset.ContractHeader{
		ID:                     storage.ContractID(storageContract.ID()),
		EnodeID:                PubkeyToEnodeID(&host.NodePubKey),
		StartHeight:            startHeight,
		TotalCost:              funding,
		ContractFee:            host.ContractPrice,
		LatestContractRevision: storageContractRevision,
		Status: storage.ContractStatus{
			UploadAbility: true,
			RenewAbility:  true,
		},
	}

	oldRoots, err := oldContract.MerkleRoots()
	if err != nil {
		return storage.ContractMetaData{}, err
	}

	// store this contract info to client local
	contractMetaData, err := cm.GetStorageContractSet().InsertContract(header, oldRoots)
	if err != nil {
		return storage.ContractMetaData{}, err
	}
	return contractMetaData, nil
}

// PubkeyToEnodeID calculate Enode.ContractID, reference:
// p2p/discover/node.go:41
// p2p/discover/node.go:59
func PubkeyToEnodeID(pubkey *ecdsa.PublicKey) enode.ID {
	var pubBytes [64]byte
	math.ReadBits(pubkey.X, pubBytes[:len(pubBytes)/2])
	math.ReadBits(pubkey.Y, pubBytes[len(pubBytes)/2:])
	return enode.ID(crypto.Keccak256Hash(pubBytes[:]))
}

// ClientPayoutsPreTax calculate client and host collateral
func ClientPayoutsPreTax(host storage.HostInfo, funding common.BigInt, basePrice common.BigInt, baseCollateral common.BigInt, period uint64, expectedStorage uint64) (clientPayout common.BigInt, hostPayout common.BigInt, hostCollateral common.BigInt, err error) {
	// Divide by zero check.
	if host.StoragePrice.Sign() == 0 {
		host.StoragePrice = common.NewBigIntUint64(1)
	}

	// Underflow check.
	if funding.Cmp(host.ContractPrice) <= 0 {
		err = errors.New("underflow detected, funding < contractPrice")
		return
	}

	// Calculate clientPayout.
	clientPayout = funding.Sub(host.ContractPrice).Sub(basePrice)

	// Calculate hostCollateral
	maxStorageSizeTime := clientPayout.Div(host.StoragePrice)
	hostCollateral = maxStorageSizeTime.Mult(host.Deposit).Add(baseCollateral)
	maxClientCollateral := host.Deposit.Mult(common.NewBigIntUint64(period)).Mult(common.NewBigIntUint64(expectedStorage)).Mult(common.NewBigIntUint64(5))
	if hostCollateral.Cmp(maxClientCollateral) > 0 {
		hostCollateral = maxClientCollateral
	}

	// Don't add more collateral than the host is willing to put into a single
	// contract.
	if hostCollateral.Cmp(host.MaxDeposit) > 0 {
		hostCollateral = host.MaxDeposit
	}

	// Calculate hostPayout.
	hostPayout = hostCollateral.Add(host.ContractPrice).Add(basePrice)
	return
}
