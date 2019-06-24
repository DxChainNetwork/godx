// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"os"
	"sync"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// ContractManager is a data structure that is used to keep track of all contracts, including
// both signed contracts and expired contracts
type ContractManager struct {
	// storage client backend
	b storage.ClientBackend

	// persistent directory
	persistDir string

	// expected payment from the storage client
	rentPayment storage.RentPayment

	// storage host manager
	hostManager *storagehostmanager.StorageHostManager

	// contract maintenance related
	maintenanceStop    chan struct{}
	maintenanceRunning bool
	maintenanceWg      sync.WaitGroup

	// contract related
	activeContracts  *contractset.StorageContractSet
	expiredContracts map[storage.ContractID]storage.ContractMetaData

	// hostID to contractID mapping
	hostToContract map[enode.ID]storage.ContractID

	// contract renew related, where renewed from connect [new] -> old
	// and renewed to connect [old] -> new
	renewedFrom      map[storage.ContractID]storage.ContractID
	renewedTo        map[storage.ContractID]storage.ContractID
	renewing         map[storage.ContractID]bool
	failedRenewCount map[storage.ContractID]uint64

	// used to acquire storage contract
	blockHeight   uint64
	currentPeriod uint64

	// utils
	log  log.Logger
	lock sync.RWMutex
	wg   sync.WaitGroup
	quit chan struct{}
}

// New will initialize the ContractManager object, which is used for contract maintenance
func New(persistDir string, hm *storagehostmanager.StorageHostManager) (cm *ContractManager, err error) {
	// contract manager initialization
	cm = &ContractManager{
		persistDir:       persistDir,
		hostManager:      hm,
		maintenanceStop:  make(chan struct{}),
		expiredContracts: make(map[storage.ContractID]storage.ContractMetaData),
		renewedFrom:      make(map[storage.ContractID]storage.ContractID),
		renewedTo:        make(map[storage.ContractID]storage.ContractID),
		failedRenewCount: make(map[storage.ContractID]uint64),
		hostToContract:   make(map[enode.ID]storage.ContractID),
		renewing:         make(map[storage.ContractID]bool),
		quit:             make(chan struct{}),
	}

	// initialize log
	cm.log = log.New("module", "contract manager")

	// initialize contract set
	cs, err := contractset.New(persistDir)
	if err != nil {
		err = fmt.Errorf("error initialize contract set: %s", err.Error())
		return
	}
	cm.activeContracts = cs

	// load the active contracts to the hostToContract mapping
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		cm.hostToContract[contract.EnodeID] = contract.ID
	}

	return
}

// Start will start the contract manager by loading the prior settings, subscribe the the block
// chain change event, save the settings, and set rentPayment payment for storage host manager
func (cm *ContractManager) Start(b storage.ClientBackend) (err error) {
	// initialize client backend
	cm.b = b

	// load prior contract information
	if err = cm.loadSettings(); err != nil && !os.IsNotExist(err) {
		return
	}

	// subscribe block chain change event
	go cm.subscribeChainChangeEvent()

	// save contract information
	if err = cm.saveSettings(); err != nil {
		return
	}

	// set allowance
	if err = cm.hostManager.SetRentPayment(cm.rentPayment); err != nil {
		return
	}

	cm.log.Info("Contract Manager Started")

	return
}

// Stop will send stop signal to threadManager, terminate all
// running go routines
func (cm *ContractManager) Stop() {
	// close the activeContracts related operations first
	if err := cm.activeContracts.Close(); err != nil {
		cm.log.Error("failed to close the contract set", "err", err.Error())
	}

	// send the quit signal to terminate all the running routines
	close(cm.quit)

	// wait until all routines are stopped
	cm.wg.Wait()
	cm.maintenanceWg.Wait()

	// log info
	log.Info("ContractManager Terminated")
}

// SetRateLimits will set the rate limits for the active contracts, which limited the
// data upload, download speed, and the packet size per upload/download
func (cm *ContractManager) SetRateLimits(readBPS int64, writeBPS int64, packetSize uint64) {
	cm.activeContracts.SetRateLimit(readBPS, writeBPS, packetSize)
}

// RetrieveRateLimit will acquire the current rate limit
func (cm *ContractManager) RetrieveRateLimit() (readBPS, writeBPS int64, packetSize uint64) {
	return cm.activeContracts.RetrieveRateLimit()
}

// GetStorageContractSet will be used to get the contract set stored with active contracts
func (cm *ContractManager) GetStorageContractSet() (contractSet *contractset.StorageContractSet) {
	return cm.activeContracts
}

// RetrieveActiveContracts will be used to retrieve all the signed contracts
func (cm *ContractManager) RetrieveActiveContracts() (cms []storage.ContractMetaData) {
	return cm.activeContracts.RetrieveAllContractsMetaData()
}

// RetrieveActiveContract will return the contract meta data based on the contract id provided
func (cm *ContractManager) RetrieveActiveContract(contractID storage.ContractID) (contract storage.ContractMetaData, exists bool) {
	return cm.activeContracts.RetrieveContractMetaData(contractID)
}

// IsRenewing will return if the contract with the contract ID passed in is renewing
func (cm *ContractManager) IsRenewing(contractID storage.ContractID) (renewing bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	_, renewing = cm.renewing[contractID]
	return
}

// HostHealthMapByID return storage.HostHealthInfoTable for hosts specified by the output
func (cm *ContractManager) HostHealthMapByID(hostIDs []enode.ID) (infoTable storage.HostHealthInfoTable) {
	// loop through the storage host id provided
	for _, id := range hostIDs {
		// get the storage host information first
		info, exists := cm.hostManager.RetrieveHostInfo(id)
		if !exists {
			continue
		}

		// get the contract id signed with that storage host
		cm.lock.RLock()
		contractID, exists := cm.hostToContract[id]
		cm.lock.RUnlock()
		if !exists {
			continue
		}

		// based on the contractID, get the contract status
		contract, exists := cm.activeContracts.RetrieveContractMetaData(contractID)
		if !exists {
			continue
		}

		// save the information into HostHealthInfo table
		infoTable[id] = storage.HostHealthInfo{
			Offline:      isOffline(info),
			GoodForRenew: contract.Status.RenewAbility,
		}
	}
	return
}

// HostHealthMap returns all storage host information and contract information from active contract list
func (cm *ContractManager) HostHealthMap() (infoTable storage.HostHealthInfoTable) {
	// loop through all active contracts
	for _, contract := range cm.activeContracts.RetrieveAllContractsMetaData() {
		// find the storage host based on the enode id
		info, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		if !exists {
			continue
		}

		// save the information into HostHealthInfo Table
		infoTable[contract.EnodeID] = storage.HostHealthInfo{
			Offline:      isOffline(info),
			GoodForRenew: contract.Status.RenewAbility,
		}
	}

	return
}
