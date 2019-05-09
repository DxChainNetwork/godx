// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
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

	rent            storage.RentPayment
	evalFunc        storagehosttree.EvaluationFunc
	storageHostTree *storagehosttree.StorageHostTree

	// ip violation check
	disableIPViolationCheck bool

	// maintenance related
	initialScan         bool
	scanWaitList        []storage.HostInfo
	scanLookup            map[string]struct{}
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
func New(persistDir string, server *p2p.Server, b storage.ClientBackend) (*StorageHostManager, error) {
	// initialization
	shm := &StorageHostManager{
		b:          b,
		p2pServer:  server,
		persistDir: persistDir,

		rent: storage.DefaultRentPayment,

		scanLookup:      make(map[string]struct{}),
		filteredHosts: make(map[string]enode.ID),
	}

	shm.evalFunc = shm.calculateEvaluationFunc(shm.rent)
	shm.storageHostTree = storagehosttree.New(shm.evalFunc)
	shm.filteredTree = shm.storageHostTree
	shm.log = log.New()

	// load prior settings
	err := shm.loadSettings()

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err := shm.tm.AfterStop(func() error {
		return shm.saveSettings()
	}); err != nil {
		return nil, err
	}

	// automatically save the settings every 2 minutes
	go shm.autoSaveSettings()

	// subscribe consensus change
	go shm.SubscribeChainChangEvent()

	// started scan and update storage host information
	go shm.scan()

	return shm, nil
}

func (shm *StorageHostManager) Close() error {
	return shm.tm.Stop()
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
