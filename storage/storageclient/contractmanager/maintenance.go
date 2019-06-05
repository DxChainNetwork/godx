// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
)

type contractRenewRecord struct {
	id   storage.ContractID
	cost common.BigInt
}

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
	if terminated, err := cm.prepareFormContract(neededContracts, clientRemainingFund); err != nil || terminated {
		return
	}

}

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
