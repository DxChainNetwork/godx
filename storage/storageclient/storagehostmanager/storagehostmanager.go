// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"os"
	"sync"
)

// StorageHostManager contains necessary fields that are used to manage storage hosts
// establishing connection with them and getting their settings
type StorageHostManager struct {
	// storage client backend
	b storage.ClientBackend

	// peer to peer communication
	p2pServer *p2p.Server

	// getting related information
	ethInfo *ethapi.PublicEthereumAPI
	netInfo *ethapi.PublicNetAPI

	rent            storage.RentPayment
	evalFunc        storagehosttree.EvaluationFunc
	storageHostTree *storagehosttree.StorageHostTree

	// ip violation check
	disableIPViolationCheck bool

	// maintenance related
	initialScanFinished bool
	scanWaitList        []storage.HostInfo
	scanPool            map[string]struct{}
	scanWait            bool
	scanningRoutines    int

	// persistent directory
	persistDir string

	// utils
	log  log.Logger
	lock sync.RWMutex
	tm   threadmanager.ThreadManager

	// filter mode related
	filterMode    FilterMode
	filteredHosts map[string]enode.ID
	filteredTree  *storagehosttree.StorageHostTree

	blockHeight uint64
}

// New will initialize HostPoolManager, making the host pool stay updated
func New(persistDir string, ethInfo *ethapi.PublicEthereumAPI, netInfo *ethapi.PublicNetAPI, server *p2p.Server, eth storage.ClientBackend) (*StorageHostManager, error) {
	// initialization
	shm := &StorageHostManager{
		b:          eth,
		p2pServer:  server,
		ethInfo:    ethInfo,
		netInfo:    netInfo,
		persistDir: persistDir,

		rent: storage.DefaultRentPayment,

		scanPool:      make(map[string]struct{}),
		filteredHosts: make(map[string]enode.ID),
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

	// TODO: (mzhang) consensus subscription

	// started scan and update storage host information
	go shm.scan()

	return shm, nil
}

// insert will insert host information into the storageHostTree
func (shm *StorageHostManager) insert(hi storage.HostInfo) error {
	err := shm.storageHostTree.Insert(hi)
	_, exists := shm.filteredHosts[hi.EnodeID.String()]
	filterWhiteList := shm.filterMode == ActiveWhitelist
	if filterWhiteList == exists {
		errF := shm.filteredTree.Insert(hi)
		if errF != nil && errF != storagehosttree.ErrHostExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}

// remove will remove the host information from the storageHostTree
func (shm *StorageHostManager) remove(enodeid string) error {
	err := shm.storageHostTree.Remove(enodeid)
	_, exists := shm.filteredHosts[enodeid]
	filterWhiteList := shm.filterMode == ActiveWhitelist
	if filterWhiteList == exists {
		errF := shm.filteredTree.Remove(enodeid)
		if errF != nil && errF != storagehosttree.ErrHostNotExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}

// modify will modify the host information from the StorageHostTree
func (shm *StorageHostManager) modify(hi storage.HostInfo) error {
	err := shm.storageHostTree.HostInfoUpdate(hi)
	_, exists := shm.filteredHosts[hi.EnodeID.String()]
	filterWhiteList := shm.filterMode == ActiveWhitelist
	if filterWhiteList == exists {
		errF := shm.filteredTree.HostInfoUpdate(hi)
		if errF != nil && errF != storagehosttree.ErrHostNotExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}
