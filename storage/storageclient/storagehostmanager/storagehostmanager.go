// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"os"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

// StorageHostManager contains necessary fields that are used to manage storage hosts
// establishing connection with them and getting their settings
type StorageHostManager struct {
	// getting related information
	ethInfo *ethapi.PublicEthereumAPI
	netInfo *ethapi.PublicNetAPI

	rent            storage.RentPayment
	evalFunc        storagehosttree.EvaluationFunc
	storageHostTree *storagehosttree.StorageHostTree

	// ip violation check
	disableIPViolationCheck bool

	// maintenance related
	initialScan          bool
	initialScanLatencies []time.Duration
	scanList             []storage.HostInfo
	scanHosts            map[string]struct{}
	scanWait             bool
	scanRoutines         int

	// persistent directory
	persistDir string

	// utils
	log  log.Logger
	lock sync.RWMutex
	tm   threadmanager.ThreadManager

	// TODO: (mzhang) the filteredHosts has the value type PublicKey. May need to change later
	// filter mode related
	filterMode    FilterMode
	filteredHosts map[string]string
	filteredTree  *storagehosttree.StorageHostTree

	//TODO:(mzhang) Consensus Change Related, sync with HZ Office
	// consider if the lastChange is needed`
	blockHeight uint64
}

// New will initialize HostPoolManager, making the host pool stay updated
func New(persistDir string, ethInfo *ethapi.PublicEthereumAPI, netInfo *ethapi.PublicNetAPI) (*StorageHostManager, error) {
	// initialization
	shm := &StorageHostManager{
		ethInfo:    ethInfo,
		netInfo:    netInfo,
		persistDir: persistDir,

		rent: storage.DefaultRentPayment,

		scanHosts:     make(map[string]struct{}),
		filteredHosts: make(map[string]string),
	}

	shm.evalFunc = shm.calculateEvaluationFunc(shm.rent)
	shm.storageHostTree = storagehosttree.New(shm.evalFunc)
	shm.filteredTree = shm.storageHostTree
	shm.log = log.New()

	// load prior settings
	shm.lock.Lock()
	err := shm.loadSettings()
	shm.lock.Unlock()

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	err = shm.tm.AfterStop(func() error {
		// save the settings
		shm.lock.Lock()
		err = shm.saveSettings()
		shm.lock.Unlock()

		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// automatically save the settings every 2 minutes
	go shm.autoSaveSettings()

	// TODO: (mzhang) consensus subscription and related operations

	// started scan and update storage host information
	go shm.scan()

	return shm, nil
}

// insert will insert host information into the storageHostTree
func (shm *StorageHostManager) insert(hi storage.HostInfo) error {
	err := shm.storageHostTree.Insert(hi)
	_, exists := shm.filteredHosts[hi.PublicKey]
	filterWhiteList := shm.filterMode == ActiveWhitelist
	if filterWhiteList == exists {
		errF := shm.filteredTree.Insert(hi)
		if errF != nil && errF != storagehosttree.ErrHostExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}
