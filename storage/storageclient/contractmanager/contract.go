// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

// CancelStorageContract will cancel all currently active contracts. Once the contracts are
// canceled, they cannot be used for file uploading, and they will not automatically be renewed
//func (cm *ContractManager) CancelStorageContract() (err error) {
//	// adding all contract into renewing list, making sure that
//	// the contract will no longer be able to perform contract revision
//	contractIDs := cm.activeContracts.IDs()
//
//	cm.lock.Lock()
//	for _, id := range contractIDs {
//		cm.renewing[id] = true
//	}
//	cm.lock.Unlock()
//
//	// delete id from the renewing list at the end
//	defer func() {
//		cm.lock.Lock()
//		for _, id := range contractIDs {
//			delete(cm.renewing, id)
//		}
//		cm.lock.Unlock()
//	}()
//
//	// if storage contract is currently revising, return error to user asking them to
//	// try again later
//	for _, id := range contractIDs {
//		if !cm.b.IsRevisionSessionDone(id) {
//			err = fmt.Errorf("contract revising, please try again later")
//			return
//		}
//	}
//
//	// update storage client settings
//	// clear the client rentPayment and current period
//	cm.lock.Lock()
//	cm.rentPayment = storage.RentPayment{}
//	cm.currentPeriod = 0
//	cm.lock.Unlock()
//
//	// save the changes persistently
//	if err = cm.saveSettings(); err != nil {
//		err = fmt.Errorf("failed to cancel storage contract, saving settings persistently failed: %s",
//			err.Error())
//		return
//	}
//
//	// terminate the storage contract maintenance
//	select {
//	case cm.maintenanceStop <- struct{}{}:
//	default:
//	}
//
//	// mark all contracts as canceled (UploadAbility, RenewAbility, Canceled)
//	for _, id := range contractIDs {
//		if err = cm.markContractCancel(id); err != nil {
//			return
//		}
//	}
//
//	return
//}

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
		// The reason that only status.Canceled got modified is that they other two
		// status will be checked in the maintainContractStatus methods during
		// contract maintenance
		status := contract.Status()
		status.Canceled = false
		err = contract.UpdateStatus(status)

		if returnErr := cm.activeContracts.Return(contract); returnErr != nil {
			cm.log.Warn("error return contract after resuming", "err", returnErr)
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

	cm.log.Debug("Maintain expiration started")

	// this list will be used to delete contract from the active contract list
	var expiredContractsIDs []storage.ContractID
	var expiredContracts []storage.ContractMetaData

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
			expiredContracts = append(expiredContracts, contract)
		}
	}

	// save the updated data information
	if err := cm.saveSettings(); err != nil {
		cm.log.Error("failed to save the expired contracts updates while checking the expired contract", "err", err.Error())
	}

	// delete the expired contract from the contract set
	cm.delFromContractSet(expiredContractsIDs)

	// check and update the connection
	cm.checkAndUpdateConnection(expiredContracts)
}

func (cm *ContractManager) checkAndUpdateConnection(contracts []storage.ContractMetaData) {
	for _, contract := range contracts {
		hostInfo, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		if !exists {
			continue
		}
		node, err := enode.ParseV4(hostInfo.EnodeURL)
		if err != nil {
			continue
		}
		cm.b.CheckAndUpdateConnection(node)
	}
}

// removeDuplications will loop through all active contracts, and find duplicated contracts -> multiple
// contracts belong to the same host, and then:
// 		1. update the expiredContract list based on the start height, the larger the start height is
// 		the newer the contract is. Older contract will be placed to the expiredContractList
// 		2. update the hostToContractID mapping, making sure it always maps to the newed contract
// 		3. update the renewFrom and renewTo map, based on the relationship among them
// 		4. update the contractSet, remove the expired contracts from the contractSet
func (cm *ContractManager) removeDuplications() {
	cm.log.Debug("Remove duplications started")

	// contracts that are going to be kept in the active contract list
	var distinctContracts = make(map[enode.ID]storage.ContractMetaData)

	// contracts that are going to mark as expired contracts and removed from the active contract list
	var duplicatedContractIDs []storage.ContractID

	// host to contracts mapping, mainly used to update renewTo and renewFrom fields
	var hostToContracts = make(map[enode.ID][]storage.ContractMetaData)

	// loop through all active contracts, find ones that has the same storage host
	// move the duplicated ones to expiredContracts
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// append all the contract to the hostToContracts mapping, used for updating renewTo and renewFrom fields
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
		cm.log.Error("failed to save the expired contracts updates persistently", "err", err.Error())
	}

	// delete the duplicated contracts from the contract set
	cm.delFromContractSet(duplicatedContractIDs)
}

// maintainHostToContractIDMapping will remove storage host with non-active contracts
func (cm *ContractManager) maintainHostToContractIDMapping() {
	cm.log.Debug("Maintain hostToContract mapping started")

	cm.lock.Lock()
	defer cm.lock.Unlock()

	// clear all entries from hostToContract
	cm.hostToContract = make(map[enode.ID]storage.ContractID)

	// re-insert them again, this time, only storage host with active contract
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		cm.hostToContract[contract.EnodeID] = contract.ID
	}
}

// removeHostWithDuplicateNetworkAddress will perform the IP violation check.
// for active storage hosts (hosts the client signed the active contract with),
// if they have same network address, based on the ip changes time, they will
// be placed under badHosts list. Then the contracts signed with them will be canceled
func (cm *ContractManager) removeHostWithDuplicateNetworkAddress() {
	cm.log.Debug("Remove host with duplicate network address started")

	// loop through all active contracts, get all their host ids
	// removeDuplications ensures that storage client can only sign one contract with each storage host
	// multiple contracts mapping to one storage host should not be possible
	var storageHostIDs []enode.ID
	var hostToContractID = make(map[enode.ID]storage.ContractID)

	// get all active storageHostIDs and hostToContractID mapping
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// if the contract has been canceled already, ignore them
		if contract.Status.Canceled && !contract.Status.UploadAbility && !contract.Status.RenewAbility {
			continue
		}

		// add the host id and contract id to mapping
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
			cm.log.Error("for contract maintenance, while removing the host with duplicate network address, the duplicatedHostID does not match with any active contract")
			continue
		}

		// cancel the contract if exists
		if err := cm.markContractCancel(contractID); err != nil {
			cm.log.Error("failed to mark the contract's status as canceled", "err", err.Error())
		}
	}
}

// maintainContractStatus will iterate through all active contracts. Based on the storage host's validation
// and contract information to update the contract status
func (cm *ContractManager) maintainContractStatus(hostsAmount int) (err error) {
	cm.log.Debug("Maintain contract status started")

	// randomly select some storage hosts, and calculate the minimum score
	hosts, err := cm.hostManager.RetrieveRandomHosts(hostsAmount+randomStorageHostsBackup, nil, nil)
	if err != nil {
		return
	}

	// for any storage host whose evaluation is below this base line
	// it will be marked as not good for upload or download
	evalBaseline := cm.calculateMinEvaluation(hosts)

	// update the contract status
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		newStatus := cm.checkContractStatus(contract, evalBaseline)
		if err = cm.updateContractStatus(contract.ID, newStatus); err != nil {
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
		// acquire the contract that is trying to delete
		contract, exists := cm.activeContracts.Acquire(id)

		// if the contract does not exist, meaning somehow it has been deleted, log a warning
		if !exists {
			cm.log.Warn("the expired contract that is trying to be deleted does not exist")
			continue
		}

		// delete the contract
		if err := cm.activeContracts.Delete(contract); err != nil {
			cm.log.Error("failed to delete the contract from the active contract list", "err", err.Error())
		}
	}
}

// markContractCancel will modify the contract status by marking
// 		1. UploadAbility: false
// 		2. RenewAbility: false
// 		3. Canceled: true
func (cm *ContractManager) markContractCancel(id storage.ContractID) (err error) {
	// get the contract
	c, exists := cm.activeContracts.Acquire(id)
	if !exists {
		cm.log.Error("the contract tyring to cancel does not exist")
		return
	}

	// at the end, return the contract
	defer func() {
		if failedReturn := cm.activeContracts.Return(c); failedReturn != nil {
			cm.log.Warn("the contract that is trying to be returned does not exist")
		}
	}()

	// update the contract status
	contractStatus := c.Status()
	contractStatus.UploadAbility = false
	contractStatus.RenewAbility = false
	contractStatus.Canceled = true
	err = c.UpdateStatus(contractStatus)

	return
}

// markNewlyFormedContractStats will mark the contract status as the following:
// 		1. UploadAbility: true
// 		2. RenewAbility: true
// 		3. Canceled: false
func (cm *ContractManager) markNewlyFormedContractStats(id storage.ContractID) (err error) {
	c, exists := cm.activeContracts.Acquire(id)
	if !exists {
		cm.log.Error("the newly formed contract's status cannot be found")
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

	// get the minimum evaluation
	minEval = cm.hostManager.Evaluation(hosts[0])
	for i := 1; i < len(hosts); i++ {
		eval := cm.hostManager.Evaluation(hosts[i])
		if eval < minEval {
			minEval = eval
		}
	}

	// divided by 100
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
		cm.log.Debug("already to renew", "blockHeight", blockHeight, "renewWindow", renewWindow, "endHeight", contract.EndHeight)
		stats.UploadAbility = false
		return
	}

	// check if the contract has enough funding for upload payment
	// each contract is in charge of a data sector, sectorStorageCost specifies the storage price
	// needed for storing a data sector in a certain period time
	sectorStorageCost := host.StoragePrice.MultUint64(contractset.SectorSize * period)

	// upload cost for uploading a sector
	sectorUploadBandwidthCost := host.UploadBandwidthPrice.MultUint64(contractset.SectorSize)

	// download cost for downloading a sector
	sectorDownloadBandwidthCost := host.DownloadBandwidthPrice.MultUint64(contractset.SectorSize)

	// total cost for store / upload / download a sector
	totalSectorCost := sectorUploadBandwidthCost.Add(sectorDownloadBandwidthCost).Add(sectorStorageCost)

	remainingBalancePercentage := contract.ContractBalance.DivWithFloatResult(contract.TotalCost)

	// if the remaining contract balance is not enough to pay for the 3X total sector cost
	// or the remainingBalancePercentage is smaller than 5%, we consider this contract is not good
	// for data uploading
	if contract.ContractBalance.Cmp(totalSectorCost.MultUint64(3)) < 0 || remainingBalancePercentage < minClientBalanceUploadThreshold {
		cm.log.Debug("the remaining contract balance is not enough to pay for the 3X total sector cost", "contractBalance",
			contract.ContractBalance.Float64(), "totalSectorCost", totalSectorCost.Float64(), "contractTotalCost", contract.TotalCost.Float64(),
			"remainingBalancePercentage", remainingBalancePercentage)
		stats.UploadAbility = false
		return
	}

	return
}

// retrieveContractStatus will get and return the contract's newest status
func (cm *ContractManager) retrieveContractStatus(id storage.ContractID) (stats storage.ContractStatus, exists bool) {
	contract, exists := cm.activeContracts.RetrieveContractMetaData(id)
	if !exists {
		return
	}
	stats = contract.Status

	return
}

// updateContractStatus will update the contract status for the contract with provided id
// with the contract status caller provided
func (cm *ContractManager) updateContractStatus(id storage.ContractID, status storage.ContractStatus) (err error) {
	// acquire the contract first
	contract, exists := cm.activeContracts.Acquire(id)
	if !exists {
		return fmt.Errorf("failed to acquire the contract: contract does not exist")
	}

	// return the contract after the function return
	defer func() {
		if err := cm.activeContracts.Return(contract); err != nil {
			cm.log.Warn("failed to return the contract, it has been deleted already", "err", err.Error())
		}
	}()

	return contract.UpdateStatus(status)
}
