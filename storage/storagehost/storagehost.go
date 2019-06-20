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
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p"
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

// Start loads all APIs and make them mapping, also introduce the account
// manager as a member variable in side the StorageHost
func (h *StorageHost) Start(eth storage.EthBackend) (err error) {
	// TODO: Start Load all APIs and make them mapping
	// init the account manager
	h.am = eth.AccountManager()
	h.ethBackend = eth

	if err = h.StorageManager.Start(); err != nil {
		return err
	}
	// parse storage contract tx API
	err = storage.FilterAPIs(h.ethBackend.APIs(), &h.parseAPI)
	if err != nil {
		h.log.Error("responsibilityFailed to parse storage contract tx API for host", "error", err)
		return
	}

	// subscribe block chain change event
	go h.subscribeChainChangEvent()
	return nil
}

// New Initialize the Host, including init the structure
// load or use the default config, init db and ext.
func New(persistDir string) (*StorageHost, error) {
	// do a host creation, but incomplete config
	host := StorageHost{
		log:                         log.New(),
		persistDir:                  persistDir,
		lockedStorageResponsibility: make(map[common.Hash]*TryMutex),
		// TODO: init the storageHostObligation
	}

	var err error   // error potentially affect the system
	var tmErr error // error for thread manager, could be handle, would be log only

	// use the thread manager to close the things open
	defer func() {
		if err != nil {
			if tmErr := host.tm.Stop(); tmErr != nil {
				err = errors.New(err.Error() + "; " + tmErr.Error())
			}
		}
	}()

	// try to make the dir for storing host files.
	// Because MkdirAll does nothing is the folder already exist, no worry to the existing folder
	if err = os.MkdirAll(persistDir, 0700); err != nil {
		host.log.Crit("Making directory hit unexpected error: " + err.Error())
		return nil, err
	}

	// initialize the storage manager
	host.StorageManager, err = sm.New(filepath.Join(persistDir, StorageManager))
	if err != nil {
		host.log.Crit("Error caused by Creating StorageManager: " + err.Error())
		return nil, err
	}

	// add the storage manager to the thread group
	// log if closing fail
	if tmErr = host.tm.AfterStop(func() error {
		err := host.StorageManager.Close()
		if err != nil {
			host.log.Warn("Fail to close storage manager: " + err.Error())
		}
		return err
	}); tmErr != nil {
		host.log.Warn(tmErr.Error())
	}

	// load the data from file or from default config
	err = host.load()
	if err != nil {
		return nil, err
	}

	// add the syncConfig to the thread group, make sure it would be store before system down
	if tmErr = host.tm.AfterStop(func() error {
		err := host.syncConfig()
		if err != nil {
			host.log.Warn("Fail to synchronize to config file: " + err.Error())
		}
		return err
	}); tmErr != nil {
		// just log the cannot syn problem, the does not sever enough to panic the system
		host.log.Warn(tmErr.Error())
	}
	//Delete residual storage responsibility
	if err = host.pruneStaleStorageResponsibilities(); err != nil {
		host.log.Info("Could not prune stale storage responsibilities:", err)
	}

	// TODO: Init the networking

	return &host, nil
}

// Close the storage host and persist the data
func (h *StorageHost) Close() error {
	err := h.tm.Stop()
	newErr := h.StorageManager.Close()
	err = common.ErrCompose(err, newErr)
	return err
}

// TODO: mock the database for storing storage obligation, currently use the
//  	 LDBDatabase, not sure which tables should be init here, modify the database
//  	 for developer's convenience
func (h *StorageHost) initDB() error {
	var err error
	h.db, err = ethdb.NewLDBDatabase(filepath.Join(h.persistDir, HostDB), 16, 16)
	if err != nil {
		return err
	}
	// add the close of database to the thread manager
	_ = h.tm.AfterStop(func() error {
		h.db.Close()
		return nil
	})
	// TODO: create the table if not exist
	return nil
}

// TODO: load the database, storage obligation, currently mock loads the config from the database,
//  	 if the config file load sucess
func (h *StorageHost) loadFromDB() error {
	return nil
}

// HostExtConfig return the host external config, which configure host through,
// user should not able to modify the config
func (h *StorageHost) HostExtConfig() storage.HostExtConfig {
	h.lock.Lock()
	defer h.lock.Unlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Call to HostExtConfig fail")
	}

	defer h.tm.Done()
	// mock the return of host external config
	return h.externalConfig()
}

// FinancialMetrics contains the information about the activities,
// commitments, rewards of host
func (h *StorageHost) FinancialMetrics() HostFinancialMetrics {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Fail to add FinancialMetrics Getter to thread manager")
	}
	defer h.tm.Done()

	return h.financialMetrics
}

// SetIntConfig set the input hostconfig to the current host if check all things are good
func (h *StorageHost) SetIntConfig(config storage.HostIntConfig, debug ...bool) error {
	// TODO: not sure the exact procedure
	// TODO: For debugging purpose, currently use vargs and tag for directly ignore the
	//  checking parts, set the config and increase the revisionNumber, for future
	//  development, please follow the logic to make the test case success as expected,
	//  or delete and  do another test case for convenience

	h.lock.Lock()
	defer h.lock.Unlock()
	if err := h.tm.Add(); err != nil {
		h.log.Crit("Fail to add HostIntConfig Getter to thread manager")
		return err
	}
	defer h.tm.Done()

	// for debugging purpose, just jump to the last part, so it won't be affected
	// by the implementation of checking parts (check unlock hash and network address)
	if debug != nil && len(debug) >= 1 && debug[0] {
		goto LOADSETTING
	}

	// TODO: Checking the NetAddress

LOADSETTING:
	h.config = config
	h.revisionNumber++

	// synchronize the config to file
	if err := h.syncConfig(); err != nil {
		return errors.New("internal config update fail: " + err.Error())
	}

	return nil
}

// InternalConfig Return the internal config of host
func (h *StorageHost) InternalConfig() storage.HostIntConfig {
	h.lock.RLock()
	defer h.lock.RUnlock()
	// if not able to add to the thread manager, simply return a empty config structure
	if err := h.tm.Add(); err != nil {
		return storage.HostIntConfig{}
	}
	defer h.tm.Done()
	return h.config
}

// load do the following things:
// 1. init the database
// 2. load the config from file
// 3. load from database
// 4. if the config file not found, create the config file, and use the default config
// 5. finally synchronize the data to config file
func (h *StorageHost) load() error {
	var err error

	// Initialize the database
	// TODO: clear the log error print
	if err = h.initDB(); err != nil {
		h.log.Crit("Unable to initialize the database: " + err.Error())
		return err
	}

	// try to load from the config files,
	if err = h.loadFromFile(); err == nil {
		// TODO: mock the loading from database
		err = h.loadFromDB()
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
		h.log.Info("Unable to close the config file")
	}

	// load the default config
	h.loadDefaults()

	// and get synchronization
	if syncErr := h.syncConfig(); syncErr != nil {
		h.log.Warn("Tempt to synchronize config to file responsibilityFailed: " + syncErr.Error())
	}

	return nil
}

// loadFromFile load host config from the file, guarantee that host would not be
// modified if error happen
func (h *StorageHost) loadFromFile() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// load and create a persist from JSON file
	persist := new(persistence)

	// NOTE:
	// if it is loaded the file causing the error, directly return the error info
	// and not do any modification to the host
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(h.persistDir, HostSettingFile), persist); err != nil {
		return err
	}

	h.loadPersistence(persist)
	return nil
}

// loadDefaults loads the default config for the host
func (h *StorageHost) loadDefaults() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.config = loadDefaultConfig()
}

func (h *StorageHost) HandleSession(s *storage.Session) error {
	msg, err := s.ReadMsg()
	if err != nil {
		return err
	}
	if handler, ok := handlerMap[msg.Code]; ok {
		return handler(h, s, msg)
	}
	return nil
}

func handleHostSettingRequest(h *StorageHost, s *storage.Session, beginMsg *p2p.Msg) error {
	s.SetDeadLine(storage.HostSettingTime)

	settings := h.externalConfig()
	if err := s.SendHostExtSettingsResponse(settings); err == nil {
		return errors.New("host setting request done")
	} else {
		return err
	}
}

// renewBasePrice returns the base cost of the storage in the  contract,
// using the host external settings and the starting file contract.
func renewBasePrice(so StorageResponsibility, settings storage.HostExtConfig, fc types.StorageContract) common.BigInt {
	if fc.WindowEnd <= so.proofDeadline() {
		return common.BigInt0
	}
	timeExtension := fc.WindowEnd - so.proofDeadline()
	return settings.StoragePrice.Mult(common.NewBigIntUint64(fc.FileSize)).Mult(common.NewBigIntUint64(uint64(timeExtension)))
}

// renewBaseDeposit returns the base cost of the storage in the  contract,
// using the host external settings and the starting  contract.
func renewBaseDeposit(so StorageResponsibility, settings storage.HostExtConfig, fc types.StorageContract) common.BigInt {
	if fc.WindowEnd <= so.proofDeadline() {
		return common.BigInt0
	}
	timeExtension := fc.WindowEnd - so.proofDeadline()
	return settings.Deposit.Mult(common.NewBigIntUint64(fc.FileSize)).Mult(common.NewBigIntUint64(uint64(timeExtension)))
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
