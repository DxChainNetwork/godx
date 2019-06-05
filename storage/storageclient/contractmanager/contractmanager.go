// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"os"
	"sync"
)

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

	// NOTE: hostToContract mapping contains both expired and active contracts
	hostToContract map[enode.ID]storage.ContractID

	// contract renew related
	renewedFrom  map[storage.ContractID]storage.ContractID
	renewedTo    map[storage.ContractID]storage.ContractID
	renewing     map[storage.ContractID]bool
	failedRenews map[storage.ContractID]uint64

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
		failedRenews:     make(map[storage.ContractID]uint64),
		hostToContract:   make(map[enode.ID]storage.ContractID),
		renewing:         make(map[storage.ContractID]bool),
		quit:             make(chan struct{}),
	}

	// initialize log
	cm.log = log.New()

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

	cm.log.Info("contract manager started")

	return
}

// Close will send stop signal to threadManager, terminate all
// running go routines
func (cm *ContractManager) Stop() {
	// close the activeContracts related operations first
	if err := cm.activeContracts.Close(); err != nil {
		cm.log.Error(fmt.Sprintf("failed to close the contract manager active contracts: %s", err.Error()))
	}

	// send the quit signal to terminate all the running routines
	close(cm.quit)

	// wait until all routines are stopped
	cm.wg.Wait()

	// log info
	log.Info("ContractManager is stopped")
}

func (cm *ContractManager) SetRateLimits(readBPS int64, writeBPS int64, packetSize uint64) {
	cm.activeContracts.SetRateLimit(readBPS, writeBPS, packetSize)
}

func (cm *ContractManager) RetrieveRateLimit() (readBPS, writeBPS int64, packetSize uint64) {
	return cm.activeContracts.RetrieveRateLimit()
}

func (cm *ContractManager) GetStorageContractSet() (contractSet *contractset.StorageContractSet) {
	return cm.activeContracts
}
