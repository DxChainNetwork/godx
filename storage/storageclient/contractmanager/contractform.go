// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

func (cm *ContractManager) prepareFormContract(neededContracts int, clientRemainingFund common.BigInt) (terminated bool, err error) {
	// get some random hosts for contract formation
	randomHosts, err := cm.randomHostsForContractForm(neededContracts)
	if err != nil {
		return
	}

	cm.lock.RLock()
	contractFund := cm.rentPayment.Fund.DivUint64(cm.rentPayment.StorageHosts).DivUint64(3)
	contractEndHeight := cm.currentPeriod + cm.rentPayment.Period + cm.rentPayment.RenewWindow
	cm.lock.RUnlock()

	// loop through each host and try to form contract with them
	for _, host := range randomHosts {
		// check if the client has enough fund for forming contract
		if contractFund.Cmp(clientRemainingFund) > 0 {
			err = fmt.Errorf("the contract fund %v is larger than client remaining fund %v. Impossible to form contract",
				contractFund, clientRemainingFund)
			return
		}

		// start to form contract
		formCost, contract, errFormContract := cm.formContract(host, contractFund, contractEndHeight)
		if errFormContract != nil {
			cm.log.Info("trying to form contract with %v, failed: %s", host.EnodeID, err.Error())
			continue
		}

		// update the client remaining fund, and try to change the newly formed contract's status
		clientRemainingFund = clientRemainingFund.Sub(formCost)
		if err = cm.markNewlyFormedContractStats(contract.ID); err != nil {
			return
		}

		// save persistently
		if failedSave := cm.saveSettings(); failedSave != nil {
			cm.log.Warn("after formed the contract, failed to save the contract manager settings")
		}

		// update the number of needed contracts
		neededContracts--
		if neededContracts <= 0 {
			break
		}

		// check if the maintenance termination signal was sent
		if terminated = cm.checkMaintenanceTermination(); terminated {
			break
		}
	}

	return
}

func (cm *ContractManager) formContract(host storage.HostInfo, contractFund common.BigInt, contractEndHeight uint64) (formCost common.BigInt, contract storage.ContractMetaData, err error) {
	// TODO (mzhang): formContract
	return
}

func (cm *ContractManager) randomHostsForContractForm(neededContracts int) (randomHosts []storage.HostInfo, err error) {
	// for all active contracts, the storage host will be added to be blacklist
	// for all active contracts which are not canceled, good for uploading, and renewing
	// the storage host will be added to the addressBlackList
	var blackList []enode.ID
	var addressBlackList []enode.ID
	activeContracts := cm.activeContracts.RetrieveAllContractsMetaData()

	cm.lock.RLock()
	for _, contract := range activeContracts {
		blackList = append(blackList, contract.EnodeID)

		// update the addressBlackList
		if contract.Status.UploadAbility && contract.Status.RenewAbility && !contract.Status.Canceled {
			addressBlackList = append(addressBlackList, contract.EnodeID)
		}
	}
	cm.lock.RUnlock()

	// randomly retrieve some hosts
	return cm.hostManager.RetrieveRandomHosts(neededContracts*randomStorageHostsFactor+randomStorageHostsBackup, blackList, addressBlackList)
}
