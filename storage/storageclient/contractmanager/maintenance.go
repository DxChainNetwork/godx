// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
)

// contractRenewRecord data structure stores information
// of the contract that needs to be renewed
type contractRenewRecord struct {
	id   storage.ContractID
	cost common.BigInt
}

// contractMaintenance will perform the following actions:
// 		1. maintainExpiration: remove all expired contract from the active contract list and adding
//		them to expired contract list
//		2. removeDuplications: contracts belong to the same storage host will be removed from the
//		active contract list
// 		3. maintainHostToContractIDMapping: update the host to contractID mapping
// 		4. removeHostWithDuplicateNetworkAddress: for storage host located under same network address, only
// 		one can be saved
// 		5. filter out contracts need to be renewed, renew contract
// 		6. check out how many more contracts need to be created, create the contracts
func (cm *ContractManager) contractMaintenance() {
	// if the maintenance is running, return directly
	// otherwise, start the maintaining job
	cm.lock.Lock()
	if cm.maintenanceRunning {
		cm.lock.Unlock()
		return
	}
	cm.maintenanceRunning = true
	cm.lock.Unlock()

	// add wait group function, register defer function
	cm.maintenanceWg.Add(1)
	defer func() {
		cm.maintenanceRunning = false
		cm.maintenanceWg.Done()
	}()

	// start maintenance
	cm.maintainExpiration()
	cm.removeDuplications()
	cm.maintainHostToContractIDMapping()
	cm.removeHostWithDuplicateNetworkAddress()
	if err := cm.maintainContractStatus(); err != nil {
		log.Warn("failed to maintain contract status, contractMaintenance terminating")
		return
	}

	// check the number of storage host needed. If the storage client does not want to
	// sign contract with anyone, return directly
	cm.lock.RLock()
	neededHosts := cm.rentPayment.StorageHosts
	cm.lock.RUnlock()

	if neededHosts <= 0 {
		return
	}

	// get the contract renew list
	closeToExpireRenews, insufficientFundingRenews := cm.checkForContractRenew()

	// reset the failed renew set, making sure it only keep track of
	// current renew list
	cm.resetFailedRenews(closeToExpireRenews, insufficientFundingRenews)

	// calculate amount of money the storage client need to spend within
	// one period cycle. It includes cost for all contracts
	var clientRemainingFund common.BigInt
	periodCost := cm.CalculatePeriodCost()

	// in case the rentPayment has been changed in the middle
	if cm.rentPayment.Fund.Cmp(periodCost.ContractFund) > 0 {
		clientRemainingFund = cm.rentPayment.Fund.Sub(periodCost.ContractFund)
	}

	// start to renew the contracts in the closeToExpireRenews list, which have higher priority
	clientRemainingFund, terminate := cm.prepareContractRenew(closeToExpireRenews, clientRemainingFund)
	if terminate {
		return
	}

	// start to renew contract in the insufficientFundingRenews list, lower priority
	clientRemainingFund, terminate = cm.prepareContractRenew(insufficientFundingRenews, clientRemainingFund)
	if terminate {
		return
	}

	// find out how many contracts are good for data uploading. Based on that data
	// calculate how many extra contracts are needed
	var uploadableContracts uint64
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		if contract.Status.UploadAbility {
			uploadableContracts++
		}
	}

	cm.lock.RLock()
	neededContracts := int(cm.rentPayment.StorageHosts - uploadableContracts)
	cm.lock.RUnlock()

	if neededContracts <= 0 {
		return
	}

	// prepare to for forming contract based on the number of extract contracts needed
	if terminated, err := cm.prepareCreateContract(neededContracts, clientRemainingFund); err != nil || terminated {
		return
	}
}

// checkMaintenanceTermination will check if the maintenanceStop signal has been sent
// if so, return true to terminate the current maintenance process. Otherwise, return false
func (cm *ContractManager) checkMaintenanceTermination() (terminate bool) {
	select {
	case <-cm.maintenanceStop:
		terminate = true
	case <-cm.quit:
		terminate = true
	default:
		terminate = false
	}
	return
}
