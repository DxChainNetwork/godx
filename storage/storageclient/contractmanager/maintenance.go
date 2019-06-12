// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"reflect"
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

	cm.log.Info("Contract Maintenance Started")

	// start maintenance
	cm.maintainExpiration()
	cm.removeDuplications()
	cm.maintainHostToContractIDMapping()
	cm.removeHostWithDuplicateNetworkAddress()

	// get the rentPayment, this rentPayment will be used for all future
	// contract renew and contract create
	cm.lock.RLock()
	rentPayment := cm.rentPayment
	cm.lock.RUnlock()

	// when RentPayment is empty, meaning that the storage client does
	// not want to sign contract with anyone
	if reflect.DeepEqual(rentPayment, storage.RentPayment{}) {
		return
	}

	if err := cm.maintainContractStatus(int(rentPayment.StorageHosts)); err != nil {
		log.Error("failed to maintain contract status, contractMaintenance terminating", "err", err.Error())
		return
	}

	// get the contract renew list
	closeToExpireRenews, insufficientFundingRenews := cm.checkForContractRenew(rentPayment)

	// reset the failed renew set, making sure it only keep track of
	// current renew list
	cm.resetFailedRenews(closeToExpireRenews, insufficientFundingRenews)

	// calculate amount of money the storage client need to spend within
	// one period cycle. It includes cost for all contracts
	var clientRemainingFund common.BigInt
	periodCost := cm.CalculatePeriodCost(rentPayment)

	// calculate the clientRemainingFund, in case the remaining fund is negative
	// set it to 0
	clientRemainingFund = rentPayment.Fund.Sub(periodCost.ContractFund)
	if clientRemainingFund.IsNeg() {
		clientRemainingFund = common.BigInt0
	}

	// start to renew the contracts in the closeToExpireRenews list, which has higher priority
	clientRemainingFund, terminate := cm.prepareContractRenew(closeToExpireRenews, clientRemainingFund, rentPayment)
	if terminate {
		return
	}

	// start to renew contract in the insufficientFundingRenews list, lower priority
	clientRemainingFund, terminate = cm.prepareContractRenew(insufficientFundingRenews, clientRemainingFund, rentPayment)
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

	// get the number of contracts that needed to be formed
	neededContracts := int(rentPayment.StorageHosts - uploadableContracts)
	if neededContracts <= 0 {
		return
	}

	// prepare to for forming contract based on the number of extract contracts needed
	terminated, err := cm.prepareCreateContract(neededContracts, clientRemainingFund, rentPayment)
	if err != nil {
		cm.log.Error("failed to create the contract", "err", err.Error())
		return
	}

	// why terminated is checked explicitly?
	// in case more codes need to be added in the future after this function
	if terminated {
		return
	}
}

// checkMaintenanceTermination will check if the maintenanceStop signal has been sent
// if so, return true to terminate the current maintenance process. Otherwise, return false
func (cm *ContractManager) checkMaintenanceTermination() (terminate bool) {
	cm.log.Debug("Maintenance termination signal received")

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
