// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"testing"
)

func TestContractManager_Start(t *testing.T) {
	shm, err := newStorageHostManagerTest()
	if err != nil {
		t.Fatalf("error creating storage host manager: %s", err.Error())
	}

	cm, err := newContractManagerTest(shm)

	if err := cm.Start(&storageClientBackendContractManager{}); err != nil {
		t.Fatalf("failed to start the contract manager: %s", err.Error())
	}
}

/*
 _____  _____  _______      __  _______ ______          ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|        |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__           | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|          |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____         | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|        |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func createNewContractManager() (cm *ContractManager, err error) {
	shm, err := newStorageHostManagerTest()
	if err != nil {
		err = fmt.Errorf("failed to create storage host manager while create contract manager: %s",
			err.Error())
		return
	}

	cm, err = newContractManagerTest(shm)

	return
}

func newContractManagerTest(hm *storagehostmanager.StorageHostManager) (cm *ContractManager, err error) {
	// create and initialize host manager
	cm = &ContractManager{
		b:                &storageClientBackendContractManager{},
		persistDir:       "test",
		hostManager:      hm,
		maintenanceStop:  make(chan struct{}),
		expiredContracts: make(map[storage.ContractID]storage.ContractMetaData),
		renewedFrom:      make(map[storage.ContractID]storage.ContractID),
		renewedTo:        make(map[storage.ContractID]storage.ContractID),
		failedRenews:     make(map[storage.ContractID]uint64),
		hostToContract:   make(map[enode.ID]storage.ContractID),
		renewing:         make(map[storage.ContractID]bool),
		quit:             make(chan struct{}),
		log:              log.New(),
	}
	cs, err := contractset.New("test")
	if err != nil {
		err = fmt.Errorf("failed to create contract set: %s", err.Error())
		return
	}

	cm.activeContracts = cs

	return
}

func newStorageHostManagerTest() (shm *storagehostmanager.StorageHostManager, err error) {
	shm = storagehostmanager.New("test")
	if err = shm.Start(&storageClientBackendContractManager{}); err != nil {
		return
	}
	return
}

type storageClientBackendContractManager struct{}

func (st *storageClientBackendContractManager) Online() bool {
	return true
}

func (st *storageClientBackendContractManager) Syncing() bool {
	return false
}

func (st *storageClientBackendContractManager) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	config = &storage.HostExtConfig{
		AcceptingContracts: true,
		Deposit:            common.NewBigInt(10),
		MaxDeposit:         common.NewBigInt(100),
	}
	return nil
}

func (st *storageClientBackendContractManager) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (st *storageClientBackendContractManager) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return nil, nil
}
