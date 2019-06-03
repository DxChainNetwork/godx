// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"errors"
	"os"
	"reflect"
	"sort"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
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
	ipViolationCheck bool

	// maintenance related
	initialScan     bool
	scanWaitList    []storage.HostInfo
	scanLookup      map[enode.ID]struct{}
	scanWait        bool
	scanningWorkers int

	// persistent directory
	persistDir string

	// utils
	log  log.Logger
	lock sync.RWMutex
	tm   threadmanager.ThreadManager

	// filter mode related
	filterMode    FilterMode
	filteredHosts map[enode.ID]struct{}
	filteredTree  *storagehosttree.StorageHostTree

	blockHeight uint64
}

// New will initialize HostPoolManager, making the host pool stay updated
func New(persistDir string) *StorageHostManager {
	// initialization
	shm := &StorageHostManager{
		persistDir:    persistDir,
		rent:          storage.DefaultRentPayment,
		scanLookup:    make(map[enode.ID]struct{}),
		filterMode:    DisableFilter,
		filteredHosts: make(map[enode.ID]struct{}),
	}

	shm.evalFunc = shm.calculateEvaluationFunc(shm.rent)
	shm.storageHostTree = storagehosttree.New(shm.evalFunc)
	shm.filteredTree = storagehosttree.New(shm.evalFunc)
	shm.log = log.New()

	shm.log.Info("Storage host manager initialized")

	return shm
}

// Start will start to load prior settings, start go routines to automatically save
// the settings every 2 min, and go routine to start storage host maintenance
func (shm *StorageHostManager) Start(server *p2p.Server, b storage.ClientBackend) error {
	// initialization
	shm.b = b
	shm.p2pServer = server

	// load prior settings
	err := shm.loadSettings()

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := shm.tm.AfterStop(func() error {
		return shm.saveSettings()
	}); err != nil {
		return err
	}

	// automatically save the settings every 2 minutes
	go shm.autoSaveSettings()

	// subscribe block chain change event
	go shm.subscribeChainChangEvent()

	// started scan and update storage host information
	go shm.scan()

	shm.log.Info("Storage Host Manager Started")

	return nil
}

// Close will send stop signal to threadmanager, terminate all the
// running go routines
func (shm *StorageHostManager) Close() error {
	return shm.tm.Stop()
}

// ActiveStorageHosts will return all active storage host information
func (shm *StorageHostManager) ActiveStorageHosts() (activeStorageHosts []storage.HostInfo) {
	allHosts := shm.storageHostTree.All()
	// based on the host information, filter out active hosts
	for _, host := range allHosts {
		numScanRecords := len(host.ScanRecords)
		if numScanRecords == 0 {
			continue
		}
		if !host.ScanRecords[numScanRecords-1].Success {
			continue
		}
		if !host.AcceptingContracts {
			continue
		}
		activeStorageHosts = append(activeStorageHosts, host)
	}
	return
}

// SetRentPayment will modify the rent payment and update the storage host
// evaluation function
func (shm *StorageHostManager) SetRentPayment(rent storage.RentPayment) (err error) {
	// during initialization, the value might be empty
	if reflect.DeepEqual(rent, storage.RentPayment{}) {
		rent = storage.DefaultRentPayment
	}

	// update the rent
	shm.lock.Lock()
	shm.rent = rent
	shm.lock.Unlock()

	// update the storage host evaluation function
	evalFunc := shm.calculateEvaluationFunc(rent)
	shm.lock.Lock()
	shm.evalFunc = evalFunc
	shm.lock.Unlock()

	// update the storage host tree and filtered tree evaluation func
	err = shm.storageHostTree.SetEvaluationFunc(evalFunc)
	err = common.ErrCompose(err, shm.filteredTree.SetEvaluationFunc(evalFunc))

	return
}

// RetrieveHostInfo will acquire the storage host information based on the enode ID provided
func (shm *StorageHostManager) RetrieveHostInfo(id enode.ID) (hi storage.HostInfo, exists bool) {
	return shm.storageHostTree.RetrieveHostInfo(id)
}

// SetIPViolationCheck will set the ipViolationCheck to be true. For storage hosts
// who are located in the same network, they will be marked as bad storage hosts
func (shm *StorageHostManager) SetIPViolationCheck(violationCheck bool) {
	shm.lock.Lock()
	defer shm.lock.Unlock()
	shm.ipViolationCheck = violationCheck
}

// FilterIPViolationHosts will evaluate the storage hosts passed in. For hosts located under the same
// network, it will be considered as badHosts if the IPViolation is enabled
func (shm *StorageHostManager) FilterIPViolationHosts(hostIDs []enode.ID) (badHostIDs []enode.ID) {
	shm.lock.RLock()
	defer shm.lock.RUnlock()

	// check if the ipViolationCheck is enabled
	if !shm.ipViolationCheck {
		return
	}

	var hostsInfo []storage.HostInfo

	// hosts validation
	for _, id := range hostIDs {
		hi, exists := shm.storageHostTree.RetrieveHostInfo(id)
		if !exists {
			badHostIDs = append(badHostIDs, id)
			continue
		}
		hostsInfo = append(hostsInfo, hi)
	}

	// sort the information based on the LastIPChange time. When there are two storage hosts
	// with same network address. The one that changes the IP earliest will not be filtered
	// out
	sort.Slice(hostsInfo[:], func(i, j int) bool {
		return hostsInfo[i].LastIPNetWorkChange.Before(hostsInfo[j].LastIPNetWorkChange)
	})

	// start the filter
	ipFilter := storagehosttree.NewFilter()
	for _, hi := range hostsInfo {
		if ipFilter.Filtered(hi.IP) {
			badHostIDs = append(badHostIDs, hi.EnodeID)
			continue
		}
		ipFilter.Add(hi.IP)
	}

	return
}

// RetrieveRandomHosts will randomly select storage hosts from the storage host pool
//  1. blacklist represents the storage host that are prohibited to be selected
//  2. addrBlacklist represents for any storage host whose network address is caontine
func (shm *StorageHostManager) RetrieveRandomHosts(num int, blacklist, addrBlacklist []enode.ID) (infos []storage.HostInfo, err error) {
	shm.lock.RLock()
	initScan := shm.initialScan
	ipCheck := shm.ipViolationCheck
	shm.lock.RUnlock()

	// if the initialize scan is not complete
	if !initScan {
		err = errors.New("storage host pool initial scan is not finished")
		return
	}

	// select random
	if ipCheck {
		infos = shm.filteredTree.SelectRandom(num, blacklist, addrBlacklist)
	} else {
		infos = shm.filteredTree.SelectRandom(num, blacklist, nil)
	}

	return
}

// Evaluation will calculate and return the evaluation of a single storage host
func (shm *StorageHostManager) Evaluation(host storage.HostInfo) (eval common.BigInt) {
	return shm.evalFunc(host).Evaluation()
}

// EvaluationDetail will calculate and return the evaluation detail of a single storage host
func (shm *StorageHostManager) EvaluationDetail(host storage.HostInfo) (detail storagehosttree.EvaluationDetail) {
	// retrieve all active storage hosts
	activeHosts := shm.ActiveStorageHosts()

	// get the total evaluation
	shm.lock.Lock()
	defer shm.lock.Unlock()

	totalEval := common.BigInt0
	for _, activeHost := range activeHosts {
		totalEval = totalEval.Add(shm.evalFunc(activeHost).Evaluation())
	}

	// compute the evaluation detail
	detail = shm.evalFunc(host).EvaluationDetail(totalEval, false, false)
	return
}

// insert will insert host information into the storageHostTree
func (shm *StorageHostManager) insert(hi storage.HostInfo) error {
	err := shm.storageHostTree.Insert(hi)
	_, exists := shm.filteredHosts[hi.EnodeID]

	if exists && shm.filterMode == WhitelistFilter {
		errF := shm.filteredTree.Insert(hi)
		if errF != nil && errF != storagehosttree.ErrHostExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}

// remove will remove the host information from the storageHostTree
func (shm *StorageHostManager) remove(enodeid enode.ID) error {
	err := shm.storageHostTree.Remove(enodeid)
	_, exists := shm.filteredHosts[enodeid]

	if exists && shm.filterMode == WhitelistFilter {
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
	_, exists := shm.filteredHosts[hi.EnodeID]

	if exists && shm.filterMode == WhitelistFilter {
		errF := shm.filteredTree.HostInfoUpdate(hi)
		if errF != nil && errF != storagehosttree.ErrHostNotExists {
			err = common.ErrCompose(err, errF)
		}
	}
	return err
}
