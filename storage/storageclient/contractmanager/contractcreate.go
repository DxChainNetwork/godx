// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/storage/storageclient/clientnegotiation/contractcreatenegotiate"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// prepareCreateContract refers that client will sign some contracts with hosts, which satisfies the upload/download demand
func (cm *ContractManager) prepareCreateContract(neededContracts int, clientRemainingFund common.BigInt, rentPayment storage.RentPayment) (terminated bool, err error) {
	// get some random hosts for contract formation
	randomHosts, err := cm.randomHostsForContractForm(neededContracts)
	if err != nil {
		return
	}

	cm.lock.RLock()
	contractFund := rentPayment.Fund.DivUint64(rentPayment.StorageHosts).DivUint64(3)
	contractEndHeight := cm.currentPeriod + rentPayment.Period + storage.RenewWindow
	cm.lock.RUnlock()

	// loop through each host and try to form contract with them
	for _, host := range randomHosts {
		// check if the client has enough fund for forming contract
		if contractFund.Cmp(clientRemainingFund) > 0 {
			err = fmt.Errorf("the contract fund %v is larger than client remaining fund %v. Impossible to create contract",
				contractFund, clientRemainingFund)
			return
		}

		// start to form contract
		formCost, contract, errFormContract := cm.createContract(host, contractFund, contractEndHeight, rentPayment)
		// if contract formation failed, the error do not need to be returned, just try to form the
		// contract with another storage host
		if errFormContract != nil {
			cm.log.Warn("failed to create the contract", "err", errFormContract.Error())
			continue
		}

		// update the client remaining fund, and try to change the newly formed contract's status
		clientRemainingFund = clientRemainingFund.Sub(formCost)
		if err = cm.markNewlyFormedContractStats(contract.ID); err != nil {
			return
		}

		// save persistently
		if failedSave := cm.saveSettings(); failedSave != nil {
			cm.log.Warn("after created the contract, failed to save the contract manager settings")
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

// createContract will try to create the contract with the host that caller passed in:
// 		1. storage host validation
// 		2. form the contract create parameters
// 		3. start to create the contract
// 		4. update the contract manager fields
func (cm *ContractManager) createContract(host storage.HostInfo, contractFund common.BigInt, contractEndHeight uint64, rentPayment storage.RentPayment) (formCost common.BigInt, newlyCreatedContract storage.ContractMetaData, err error) {
	// 1. storage host validation
	// validate the storage price
	if host.StoragePrice.Cmp(maxHostStoragePrice) > 0 {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, the storage price is too high", host.EnodeID)
		return
	}

	// validate the storage host max deposit
	if host.MaxDeposit.Cmp(maxHostDeposit) > 0 {
		host.MaxDeposit = maxHostDeposit
	}

	// validate the storage host max duration
	if host.MaxDuration < rentPayment.Period {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, the max duration is smaller than period", host.EnodeID)
		return
	}

	// 2. form the contract create parameters
	// The reason to get the newest blockHeight here is that during the checking time period
	// many blocks may be generated already, which is unfair to the storage client.
	cm.lock.RLock()
	startHeight := cm.blockHeight
	cm.lock.RUnlock()

	// try to get the clientPaymentAddress. If failed, return error directly and set the contract creation cost
	// to be zero
	var clientPaymentAddress common.Address
	if clientPaymentAddress, err = cm.b.GetPaymentAddress(); err != nil {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract with host: %v, failed to get the clientPayment address: %s", host.EnodeID, err.Error())
		return
	}

	// form the contract create parameters
	params := storage.ContractParams{
		RentPayment:          rentPayment,
		HostEnodeURL:         host.EnodeURL,
		Funding:              contractFund,
		StartHeight:          startHeight,
		EndHeight:            contractEndHeight,
		ClientPaymentAddress: clientPaymentAddress,
		Host:                 host,
	}

	// 3. create the contract
	if newlyCreatedContract, err = contractcreatenegotiate.Handler(cm, params); err != nil {
		formCost = common.BigInt0
		err = fmt.Errorf("failed to create the contract: %s", err.Error())
		return
	}

	// 4. update the contract manager fields
	cm.lock.Lock()
	// check if the storage client have created another contract with the same storage host
	if _, exists := cm.hostToContract[newlyCreatedContract.EnodeID]; exists {
		cm.lock.Unlock()
		formCost = contractFund
		err = fmt.Errorf("client already formed a contract with the same storage host %v", newlyCreatedContract.EnodeID)
		return
	}

	// if not exists, update the host to contract mapping
	cm.hostToContract[newlyCreatedContract.EnodeID] = newlyCreatedContract.ID
	cm.lock.Unlock()

	formCost = contractFund
	return
}

// randomHostsForContractForm will randomly retrieve some storage hosts from the storage host pool
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
