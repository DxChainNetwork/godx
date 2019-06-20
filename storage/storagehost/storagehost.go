// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	tm "github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
)

// StorageHost provide functions for storageHost management
// It loads or use default config when it have been initialized
// It aims at communicate by protocal with client and lent its own storage to the client
type StorageHost struct {
	// backend support
	ethBackend storage.EthBackend
	parseAPI   storage.ParsedAPI

	// Account manager for wallet/account related operation
	am *accounts.Manager

	// storageHost basic config
	broadcast          bool
	broadcastConfirmed bool
	blockHeight        uint64

	financialMetrics HostFinancialMetrics
	config           storage.HostIntConfig
	revisionNumber   uint64

	// storage host manager for manipulating the file storage system
	sm.StorageManager

	lockedStorageResponsibility map[common.Hash]*TryMutex

	// things for log and persistence
	db         *ethdb.LDBDatabase
	persistDir string
	log        log.Logger

	// things for thread safety
	lock sync.RWMutex
	tm   tm.ThreadManager
}

// New Initialize the Host, including init the structure
// load or use the default config, init db and ext.
func New(persistDir string) (*StorageHost, error) {
	// do a host creation, but incomplete config
	h := StorageHost{
		log:                         log.New(),
		persistDir:                  persistDir,
		lockedStorageResponsibility: make(map[common.Hash]*TryMutex),
	}

	var err error
	// Create the data path
	if err = os.MkdirAll(h.persistDir, 0700); err != nil {
		return nil, err
	}
	// Create the database

	// initialize the storage manager
	if h.StorageManager, err = sm.New(persistDir); err != nil {
		return nil, err
	}
	// open the database
	if h.db, err = openDB(filepath.Join(persistDir, databaseFile)); err != nil {
		return nil, err
	}

	return &h, nil
}

// Start loads all APIs and make them mapping, also introduce the account
// manager as a member variable in side the StorageHost
func (h *StorageHost) Start(eth storage.EthBackend) (err error) {
	// init the account manager
	h.am = eth.AccountManager()
	h.ethBackend = eth

	// load the data from file or from default config
	if err = h.load(); err != nil {
		return err
	}
	// start the storage manager
	if err = h.StorageManager.Start(); err != nil {
		return err
	}
	// parse storage contract tx API
	err = storage.FilterAPIs(h.ethBackend.APIs(), &h.parseAPI)
	if err != nil {
		h.log.Error("responsibilityFailed to parse storage contract tx API for host", "error", err)
		return
	}
	//Delete residual storage responsibility
	if err = h.pruneStaleStorageResponsibilities(); err != nil {
		return err
	}
	// subscribe block chain change event
	go h.subscribeChainChangEvent()
	return nil
}

// Close the storage host and persist the data
func (h *StorageHost) Close() error {
	err := h.tm.Stop()

	newErr := h.StorageManager.Close()
	err = common.ErrCompose(err, newErr)

	newErr = h.syncConfig()
	err = common.ErrCompose(err, newErr)
	return err
}

// hostExtConfig return the host external config, which configure host through,
// user should not able to modify the config
func (h *StorageHost) hostExtConfig() (storage.HostExtConfig, error) {
	h.lock.Lock()
	defer h.lock.Unlock()

	// mock the return of host external config
	return h.externalConfig()
}

// financialStats contains the information about the activities,
// commitments, rewards of host
func (h *StorageHost) financialStats() HostFinancialMetrics {
	h.lock.RLock()
	defer h.lock.RUnlock()

	return h.financialMetrics
}

// internalConfig Return the internal config of host
func (h *StorageHost) internalConfig() storage.HostIntConfig {
	h.lock.RLock()
	defer h.lock.RUnlock()

	return h.config
}

// SetIntConfig set the input hostconfig to the current host if check all things are good
func (h *StorageHost) setIntConfig(config storage.HostIntConfig) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if err := h.tm.Add(); err != nil {
		return errStopped
	}
	defer h.tm.Done()

	h.config = config

	// synchronize the config to file
	if err := h.syncConfig(); err != nil {
		return errors.New("internal config update fail: " + err.Error())
	}
	return nil
}

// load do the following things:
// 1. init the database
// 2. load the config from file
// 3. load from database
// 4. if the config file not found, create the config file, and use the default config
// 5. finally synchronize the data to config file
func (h *StorageHost) load() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// try to load from the config files,
	if err := h.loadConfig(); err == nil {
		return err
	} else if !os.IsNotExist(err) {
		// if the error is NOT caused by FILE NOT FOUND Exception
		return err
	}

	// At this step, the error is caused by FILE NOT FOUND Exception
	// Create the config file
	h.log.Info("Creat a new HostSetting file")

	// currently the error is caused by file not found exception
	// create the file
	file, err := os.Create(filepath.Join(h.persistDir, HostSettingFile))
	if err != nil {
		// if the error is throw when create the file
		// close the file and directly return the error
		_ = file.Close()
		return err
	}
	// assert the error is nil, close the file
	if err := file.Close(); err != nil {
		return err
	}
	// load the default config
	h.config = defaultConfig()

	// and get synchronization
	if syncErr := h.syncConfig(); syncErr != nil {
		h.log.Warn("Tempt to synchronize config to file responsibilityFailed: " + syncErr.Error())
	}
	return nil
}

// StorageResponsibilities fetches the set of storage Responsibility in the host and
// returns metadata on them.
func (h *StorageHost) StorageResponsibilities() (sos []StorageResponsibility) {
	if len(h.lockedStorageResponsibility) < 1 {
		return nil
	}

	for i := range h.lockedStorageResponsibility {
		so, err := GetStorageResponsibility(h.db, i)
		if err != nil {
			h.log.Warn("Failed to get storage responsibility", "err", err)
			continue
		}
		sos = append(sos, so)
	}
	return sos
}
