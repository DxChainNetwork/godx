// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

// checkForContractRenew will loop through all active contracts and filter out those needs to be renewed.
// There are two types of contract needs to be renewed
// 		1. contracts that are about to expired. they need to be renewed
// 		2. contracts that have insufficient amount of funding, meaning the contract is about to be
// 		   marked as not good for data uploading
func (cm *ContractManager) checkForContractRenew(rentPayment storage.RentPayment) (closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {

	cm.lock.RLock()
	currentBlockHeight := cm.blockHeight
	cm.lock.RUnlock()

	// loop through all active contracts, get the closeToExpireRenews and insufficientFundingRenews
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// validate the storage host for the contract, check if the host exists or get filtered
		host, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		if !exists || host.Filtered {
			continue
		}

		// verify if the contract is good for renew
		if !contract.Status.RenewAbility {
			continue
		}

		// for contract that is about to expire, it will be added to the priorityRenews
		// calculate the renewCostEstimation and update the priorityRenews
		if currentBlockHeight+rentPayment.RenewWindow >= contract.EndHeight {
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
		remainingBalancePercentage := contract.ContractBalance.DivWithFloatResult(contract.TotalCost)

		if contract.ContractBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minContractPaymentRenewalThreshold {
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

	contractMeta, exists := cm.RetrieveActiveContract(renewContractID)
	if !exists {
		renewCost = common.BigInt0
		err = fmt.Errorf("the oldContract that is trying to be renewed no longer exists")
		return
	}

	// if the contract is revising, return error directly
	if cm.b.TryToRenewOrRevise(contractMeta.EnodeID) {
		renewCost = common.BigInt0
		err = fmt.Errorf("the contract is revising, cannot be renewed")
		return
	}

	// finished renewing
	defer cm.b.RevisionOrRenewingDone(contractMeta.EnodeID)

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
		cm.log.Warn("failed to update the renewed oldContract status", "err", err.Error())
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
		RentPayment:          rentPayment,
		HostEnodeURL:         host.EnodeURL,
		Funding:              contractFund,
		StartHeight:          startHeight,
		EndHeight:            contractEndHeight,
		ClientPaymentAddress: clientPaymentAddress,
		Host:                 host,
	}

	// 4. contract renew
	if renewedContract, err = cm.ContractRenewNegotiate(renewContract, params); err != nil {
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
			cm.log.Warn("failed to update the contract status during renew failed handling", "err", err.Error())
		}

		err = fmt.Errorf("marked the contract %v as canceled due to the large amount of renew fails: %s", failedContract.Metadata().ID, renewError.Error())
		return
	}

	// otherwise, do nothing, a renew attempt to the same contract will be performed again
	err = fmt.Errorf("failed to renew the contract: %s", renewError.Error())
	return
}
