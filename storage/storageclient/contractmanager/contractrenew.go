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
func (cm *ContractManager) checkForContractRenew() (closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {
	cm.lock.RLock()
	currentBlockHeight := cm.blockHeight
	rentPayment := cm.rentPayment
	cm.lock.RUnlock()

	// loop through all active contracts, get the priorityRenews and emptyRenews
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

		remainingBalancePercentage := contract.ClientBalance.Div(contract.TotalCost).Float64()

		if contract.ClientBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minContractPaymentRenewalThreshold {
			insufficientFundingRenews = append(insufficientFundingRenews, contractRenewRecord{
				id:   contract.ID,
				cost: contract.TotalCost.MultUint64(2),
			})
		}
	}

	return
}

// resetFailedRenews will update the failedRenews list, which only includes the failedRenews
// in the current renew lists
func (cm *ContractManager) resetFailedRenews(closeToExpireRenews []contractRenewRecord, insufficientFundingRenews []contractRenewRecord) {
	cm.lock.Lock()
	filteredFailedRenews := make(map[storage.ContractID]uint64)

	// loop through the closeToExpireRenews, get the failedRenews
	for _, renewRecord := range closeToExpireRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenews[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenews[contractID]
		}
	}

	// loop through
	for _, renewRecord := range insufficientFundingRenews {
		contractID := renewRecord.id
		if _, exists := cm.failedRenews[contractID]; exists {
			filteredFailedRenews[contractID] = cm.failedRenews[contractID]
		}
	}

	// reset the failedRenews
	cm.failedRenews = filteredFailedRenews
	cm.lock.Unlock()
}

// prepareContractRenew will loop through all record in the renewRecords and start to renew
// each contract. Before contract renewing get started, the fund will be validated first.
func (cm *ContractManager) prepareContractRenew(renewRecords []contractRenewRecord, clientRemainingFund common.BigInt) (remainingFund common.BigInt, terminate bool) {
	// get the data needed
	cm.lock.RLock()
	rentPayment := cm.rentPayment
	blockHeight := cm.blockHeight
	currentPeriod := cm.currentPeriod
	contractEndHeight := cm.currentPeriod + cm.rentPayment.Period + cm.rentPayment.RenewWindow
	cm.lock.RUnlock()

	for _, record := range renewRecords {
		// verify that the cost needed for contract renew does not exceed the clientRemainingFund
		if clientRemainingFund.Cmp(record.cost) < 0 {
			continue
		}

		// renew the contract, get the spending for the renew
		renewCost, err := cm.renewContract(record, currentPeriod, rentPayment, blockHeight, contractEndHeight)
		if err != nil {
			cm.log.Error(fmt.Sprintf("contract renew failed. id: %v, err : %v", record.id, err.Error()))
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

// renewContract will start to perform contract renew operation
func (cm *ContractManager) renewContract(record contractRenewRecord, currentPeriod uint64, rentPayment storage.RentPayment, blockHeight uint64, contractEndHeight uint64) (renewCost common.BigInt, err error) {
	// TODO (mzhang): renewContarct
	return
}
