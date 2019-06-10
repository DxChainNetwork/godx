// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"sort"
)

// cancel contract will cancel a contracts based on the storage contract ID provided
// once the contract got canceled, it will not be used for file uploading.
// Moreover, it will not be automatically renewed.
func (cm *ContractManager) CancelContract(id storage.ContractID) (err error) {
	// TODO (mzhang): TO BE CONTINUED
	return
}

// cancel all contracts will cancel all currently active contracts. Once the contracts are
// canceled, they cannot be used for file uploading, and they will not automatically be renewed
func (cm *ContractManager) CancelAllContracts() (err error) {
	// TODO (mzhang): TO BE CONTINUED
	return
}

// resumeContracts will mark all activated contracts "canceled" status
// to be false
func (cm *ContractManager) resumeContracts() (err error) {
	// get all active contracts, and resume any canceled contracts
	ids := cm.activeContracts.IDs()

	// look through all contracts, resume them by updating their status
	for _, id := range ids {
		contract, exists := cm.activeContracts.Acquire(id)

		if !exists {
			continue
		}

		// getting the contract status and setting the canceling status to be false
		status := contract.Status()
		status.Canceled = false
		err = contract.UpdateStatus(status)

		if returnErr := cm.activeContracts.Return(contract); returnErr != nil {
			cm.log.Crit("error return contract after resuming: %s", returnErr.Error())
		}

		if err != nil {
			return
		}
	}
	return
}

// maintainExpiration will loop through active contracts and find out ones that are expired.
// For expired contracts:
// 		1. update the expiredContract list
// 		2. remove from the contractSet
// Expired Contracts Criteria:
// 		1. current block height is greater than the contract's endHeight
// 		2. the contract has been renewed
func (cm *ContractManager) maintainExpiration() {
	// this list will be used to delete contract from the active contract list
	var expiredContractsIDs []storage.ContractID

	// get the current block height
	cm.lock.RLock()
	currentBh := cm.blockHeight
	cm.lock.RUnlock()

	// loop through all active contracts
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// check if the contract is expired or renewed
		cm.lock.RLock()
		_, renewed := cm.renewedTo[contract.ID]
		cm.lock.RUnlock()
		expired := currentBh > contract.EndHeight
		// update the expired contract list
		if expired || renewed {
			cm.updateExpiredContracts(contract)
			expiredContractsIDs = append(expiredContractsIDs, contract.ID)
		}
	}

	// save the updated data information
	if err := cm.saveSettings(); err != nil {
		cm.log.Error("failed to save the expired contracts updates while checking the expired contract")
	}

	// delete the expired contract from the contract set
	cm.delFromContractSet(expiredContractsIDs)
}

// removeDuplications will loop through all active contracts, and find duplicated contracts -> multiple
// contracts belong to the same host, and then:
// 		1. update the expiredContract list based on the start height, the larger the start height is
// 		the newer the contract is. Older contract will be placed to the expiredContractList
// 		2. update the hostToContractID mapping, making sure it always maps to the newed contract
// 		3. update the renewFrom and renewTo map, based on the relationship among them
// 		4. update the contractSet, remove the expired contracts from the contractSet
func (cm *ContractManager) removeDuplications() {
	var distinctContracts = make(map[enode.ID]storage.ContractMetaData)
	var duplicatedContractIDs []storage.ContractID
	var hostToContracts = make(map[enode.ID][]storage.ContractMetaData)

	// loop through all active contracts, find ones that has the same storage host
	// move the duplicated ones to expiredContracts
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// append all the contract to the hostToContracts mapping, used for contract renew
		hostToContracts[contract.EnodeID] = append(hostToContracts[contract.EnodeID], contract)

		// check the distinctContracts and apply updates
		existedContract, exists := distinctContracts[contract.EnodeID]
		// if the contract does not existed, push it into the distinctContracts
		if !exists {
			distinctContracts[contract.EnodeID] = contract
			continue
		}

		// if exists, compare the start height, the larger the start height is, the newer the contract is
		// newer contract has the larger start height. Update the distinctContracts list as well
		if existedContract.StartHeight < contract.StartHeight {
			duplicatedContractIDs = append(duplicatedContractIDs, existedContract.ID)
			// the older contract should be moved to expired contract list
			cm.updateExpiredContracts(existedContract)

			// the newer contract should be added to the mapping
			cm.updateHostToContractID(contract)

			// update the distinct contract list
			distinctContracts[contract.EnodeID] = contract
			continue
		}
		// if the existedContract is newer, has the larger start height
		duplicatedContractIDs = append(duplicatedContractIDs, contract.ID)
		// the older contract should be moved to expired contract list
		cm.updateExpiredContracts(contract)

		// the newer contract should be added to the mapping
		cm.updateHostToContractID(existedContract)
	}

	// update contract renew
	cm.updateContractRenew(hostToContracts)

	// save the update data information
	if err := cm.saveSettings(); err != nil {
		cm.log.Error("failed to save the expired contracts updates while removing the duplications")
	}

	// delete the duplicated contracts from the contract set
	cm.delFromContractSet(duplicatedContractIDs)
}

// maintainHostToContractIDMapping will remove storage host with non-active contracts
func (cm *ContractManager) maintainHostToContractIDMapping() {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	// clear all entries from hostToContract
	cm.hostToContract = make(map[enode.ID]storage.ContractID)

	// re-inserted them again, this time, only storage host with active contract
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		cm.hostToContract[contract.EnodeID] = contract.ID
	}
}

// removeHostWithDuplicateNetworkAddress will perform the IP violation check.
// for active storage hosts (hosts the client signed the active contract with),
// if they have same network address, based on the ip changes time, they will
// be placed under badHosts list. Then the contracts signed with them will be canceled
func (cm *ContractManager) removeHostWithDuplicateNetworkAddress() {
	// loop through all active contracts, get all their host ids
	var storageHostIDs []enode.ID
	var hostToContractID = make(map[enode.ID]storage.ContractID)

	// get all active storageHostIDs and hostToContractID mapping
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// if the contract has been canceled already, ignore them
		if contract.Status.Canceled && !contract.Status.UploadAbility && !contract.Status.RenewAbility {
			continue
		}
		storageHostIDs = append(storageHostIDs, contract.EnodeID)
		hostToContractID[contract.EnodeID] = contract.ID
	}

	// perform ip violation check, got all host ids with duplicated network address
	duplicatedHostIDs := cm.hostManager.FilterIPViolationHosts(storageHostIDs)

	// loop through the host id, cancel the contract
	for _, hid := range duplicatedHostIDs {
		contractID, exists := hostToContractID[hid]

		// check if the contract exists
		if !exists {
			cm.log.Crit("the duplicatedHostID does not match with any active contract")
			continue
		}

		// cancel the contract if exists
		if err := cm.markContractCancel(contractID); err != nil {
			cm.log.Crit(fmt.Sprintf("failed to cancel the contract %v: %s", contractID, err.Error()))
		}
	}
}

// maintainContractStatus will iterate through all active contracts. Based on the storage host's validation
// and contract information to update the contract status
func (cm *ContractManager) maintainContractStatus() (err error) {
	cm.lock.RLock()
	hostsAmount := int(cm.rentPayment.StorageHosts)
	cm.lock.RUnlock()

	// randomly select some storage hosts, and calculate the minimum score
	hosts, err := cm.hostManager.RetrieveRandomHosts(hostsAmount+randomStorageHostsBackup, nil, nil)
	if err != nil {
		return
	}

	// for any storage host whose evaluation is below this base line
	// it will be marked as not good for upload or download
	evalBaseline := cm.calculateMinEvaluation(hosts)

	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		newStatus := cm.checkContractStatus(contract, evalBaseline)
		contract, exists := cm.activeContracts.Acquire(contract.ID)
		if !exists {
			return
		}

		if err = contract.UpdateStatus(newStatus); err != nil {
			return
		}

		if err = cm.activeContracts.Return(contract); err != nil {
			return
		}
	}

	return
}

// updateContractRenew will update the renew to and renew from list. One host can map to multiple
// contracts.
func (cm *ContractManager) updateContractRenew(hostToContracts map[enode.ID][]storage.ContractMetaData) {
	for _, contracts := range hostToContracts {
		// sort the contracts, the newer the contract is, it will be nearer to the front of the slice
		sort.Slice(contracts[:], func(i, j int) bool {
			return contracts[i].StartHeight > contracts[j].StartHeight
		})

		// update the renewedFrom (new contract renewed from the old contract)
		for i := 0; i < len(contracts)-1; i++ {
			cm.lock.Lock()
			cm.renewedFrom[contracts[i].ID] = contracts[i+1].ID
			cm.lock.Unlock()
		}

		// update the renewedTo (old contract renewed to new contract)
		for i := len(contracts) - 1; i > 1; i-- {
			cm.lock.Lock()
			cm.renewedTo[contracts[i].ID] = contracts[i-1].ID
			cm.lock.Unlock()
		}
	}
}

// updateExpireContracts will place the contract into expired contracts list
func (cm *ContractManager) updateExpiredContracts(contract storage.ContractMetaData) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.expiredContracts[contract.ID] = contract
}

// updateHostToContractID will update the hostToContract field, making sure that
// the contract mapped to the enodeID is the newest contract, meaning the contract
// with greater start height, if there are multiple of them
func (cm *ContractManager) updateHostToContractID(contract storage.ContractMetaData) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.hostToContract[contract.EnodeID] = contract.ID
}

// delFromContractSet will delete a list of contract from the activeContracts list
// based on the list of contract id provided
func (cm *ContractManager) delFromContractSet(ids []storage.ContractID) {
	for _, id := range ids {
		contract, exists := cm.activeContracts.Acquire(id)
		if !exists {
			cm.log.Warn("the expired contract that is trying to be deleted does not exist")
			continue
		}

		if err := cm.activeContracts.Delete(contract); err != nil {
			cm.log.Warn(err.Error())
		}
	}
}

// markContractCancel will modify the contract status by marking
// 		1. UploadAbility: false
// 		2. RenewAbility: false
// 		3. Canceled: true
func (cm *ContractManager) markContractCancel(id storage.ContractID) (err error) {
	c, exists := cm.activeContracts.Acquire(id)
	if !exists {
		cm.log.Crit("the contract tyring to cancel does not exist")
		return
	}
	contractStatus := c.Status()
	contractStatus.UploadAbility = false
	contractStatus.RenewAbility = false
	contractStatus.Canceled = true
	err = c.UpdateStatus(contractStatus)
	if failedReturn := cm.activeContracts.Return(c); failedReturn != nil {
		cm.log.Warn("the contract that is trying to be returned does not exist")
	}
	return
}

// markNewlyFormedContractStats will mark the contract status as the following:
// 		1. UploadAbility: true
// 		2. RenewAbility: true
// 		3. Canceled: false
func (cm *ContractManager) markNewlyFormedContractStats(id storage.ContractID) (err error) {
	c, exists := cm.activeContracts.Acquire(id)
	if !exists {
		cm.log.Crit("the newly formed contract's status cannot be found")
		err = fmt.Errorf("the newly formed contract's status cannot be found from the contract set")
		return
	}
	contractStatus := c.Status()
	contractStatus.UploadAbility = true
	contractStatus.RenewAbility = true
	contractStatus.Canceled = false
	err = c.UpdateStatus(contractStatus)
	if failedReturn := cm.activeContracts.Return(c); failedReturn != nil {
		cm.log.Warn("the contract that is trying to be returned does not exist")
	}

	return
}

// calculateMinEvaluation will get the minimum evaluation from a list of hosts, which is used to
// evaluate the current hosts that client signed the contract with.
func (cm *ContractManager) calculateMinEvaluation(hosts []storage.HostInfo) (minEval common.BigInt) {
	// if there are no hosts passed in, return 0 directly
	if len(hosts) == 0 {
		return common.BigInt0
	}

	minEval = cm.hostManager.Evaluation(hosts[0])
	for i := 1; i < len(hosts); i++ {
		eval := cm.hostManager.Evaluation(hosts[i])
		if eval.Cmp(minEval) < 0 {
			minEval = eval
		}
	}

	minEval = minEval.DivUint64(evalFactor)
	return
}

// checkContractStatus will validate and return the new contract status based on the following criteria
// 		1. if the status of the contract is not canceled, then mark the upload and renew ability to be true
// 		2. if the host that the client signed the contract with cannot be found or the host has been filtered, mark
//		upload and renew ability to be false
// 		3. if the host's evaluation is smaller than the baseline, then mark the current contract as not good
// 		for uploading and renewing
// 		4. if the storage host that signed contract with is offline, mark the current contract as
// 		not good for uploading and renewing
// 		5. if the contract has been renewed already, mark the upload ability to false
// 		6. lastly, if the client does not have enough money left, mark the upload ability as false
func (cm *ContractManager) checkContractStatus(contract storage.ContractMetaData, evalBaseline common.BigInt) (stats storage.ContractStatus) {
	stats = contract.Status

	// mark upload and renew ability as true, if the contract is not canceled
	if !stats.Canceled {
		stats.UploadAbility = true
		stats.RenewAbility = true
	}

	// check if the host that signed the contract with is valid
	host, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
	if !exists || host.Filtered {
		stats.UploadAbility = false
		stats.RenewAbility = false
		return
	}

	// check the storage host's evaluation, if the evaluation is smaller than baseline, mark
	// the upload and renew ability to be false
	eval := cm.hostManager.Evaluation(host)

	// if the baseline is bigger than 0 and the host evaluation is smaller than the baseline
	if eval.Cmp(evalBaseline) < 0 && evalBaseline.Cmp(common.BigInt0) > 0 {
		stats.UploadAbility = false
		stats.RenewAbility = false
		return
	}

	// check if the storage host if offline, if so, mark the upload and renew ability to be false
	if isOffline(host) {
		stats.UploadAbility = false
		stats.RenewAbility = false
		return
	}

	// check if the contract should be renewed, if so, mark the contract upload ability to be false
	cm.lock.RLock()
	blockHeight := cm.blockHeight
	renewWindow := cm.rentPayment.RenewWindow
	period := cm.rentPayment.Period
	cm.lock.RUnlock()

	// if the contract is expected to be renewed already
	if blockHeight+renewWindow >= contract.EndHeight {
		stats.UploadAbility = false
		return
	}

	// check if the contract has enough funding for upload payment
	sectorStorageCost := host.StoragePrice.MultUint64(contractset.SectorSize * period)
	sectorUploadBandwidthCost := host.UploadBandwidthPrice.MultUint64(contractset.SectorSize)
	sectorDownloadBandwidthCost := host.DownloadBandwidthPrice.MultUint64(contractset.SectorSize)
	totalSectorCost := sectorUploadBandwidthCost.Add(sectorDownloadBandwidthCost).Add(sectorStorageCost)

	remainingBalancePercentage := contract.ClientBalance.Div(contract.TotalCost).Float64()

	if contract.ClientBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minClientBalanceUploadThreshold {
		stats.UploadAbility = false
		return
	}

	return
}
