package storagehost

import (
	"errors"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	tg "github.com/DxChainNetwork/godx/common/threadManager"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
	"os"
	"path/filepath"
	"sync"
)

// TODO: Network, Transaction, protocol related implementations BELOW:
// TODO: check unlock hash
func (h *StorageHost) checkUnlockHash() error {
	return nil
}

// TODO: return the externalSettings for host
func (h *StorageHost) externalSettings() StorageHostExtSetting {
	return StorageHostExtSetting{}
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
	_ = h.tg.AfterStop(func() error {
		h.db.Close()
		return nil
	})
	// TODO: create the table if not exist
	return nil
}

// TODO: load the database, storage obligation, currently mock loads the settings from the database,
//  	 if the setting file load sucess
func (h *StorageHost) loadFromDB() error {
	return nil
}

type StorageHost struct {

	// Account manager for wallet/account related operation
	am *accounts.Manager

	// storageHost basic settings
	broadcast          bool
	broadcastConfirmed bool

	financialMetrics HostFinancialMetrics
	settings         StorageHostIntSetting
	revisionNumber   uint64

	// storage host manager for manipulating the file storage system
	sm.StorageManagerAPI

	// things for log and persistence
	// TODO: database to store the info of storage obligation, here just a mock
	db         *ethdb.LDBDatabase
	persistDir string
	log        log.Logger

	// things for thread safety
	mu sync.RWMutex
	tg tg.ThreadManager
}

// TODO: Load all APIs and make them mapping
func (h *StorageHost) Start(eth Backend) {
	// init the account manager
	h.am = eth.AccountManager()
}

// Initialize the Host
func NewStorageHost(persistDir string) (*StorageHost, error) {
	// do a host creation, but incomplete settings

	host := StorageHost{
		log:        log.New(),
		persistDir: persistDir,
		// TODO: init the storageHostObligation
	}

	var err error   // error potentially affect the system
	var tgErr error // error for thread manager, could be handle, would be log only

	// use the thread manager to close the things open
	defer func() {
		if err != nil {
			if tgErr := host.tg.Stop(); tgErr != nil {
				err = errors.New(err.Error() + "; " + tgErr.Error())
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
	host.StorageManagerAPI, err = sm.New(filepath.Join(persistDir, StorageManager))
	if err != nil {
		host.log.Crit("Error caused by Creating StorageManager: " + err.Error())
		return nil, err
	}

	// add the storage manager to the thread group
	// log if closing fail
	if tgErr = host.tg.AfterStop(func() error {
		err := host.StorageManagerAPI.Close()
		if err != nil {
			host.log.Warn("Fail to close storage manager: " + err.Error())
		}
		return err
	}); tgErr != nil {
		host.log.Warn(tgErr.Error())
	}

	// load the data from file or from default setting
	err = host.load()
	if err != nil {
		return nil, err
	}

	// add the syncSetting to the thread group, make sure it would be store before system down
	if tgErr = host.tg.AfterStop(func() error {
		err := host.syncSetting()
		if err != nil {
			host.log.Warn("Fail to synchronize to setting file: " + err.Error())
		}
		return err
	}); tgErr != nil {
		// just log the cannot syn problem, the does not sever enough to panic the system
		host.log.Warn(tgErr.Error())
	}

	// TODO: storageObligation handle

	// TODO: Init the networking

	return &host, nil
}

// close the storage host and persist the data
func (h *StorageHost) Close() error {
	return h.tg.Stop()
}

// return the host external settings, which configure host through,
// user should not able to modify the setting
func (h *StorageHost) StorageHostExtSetting() StorageHostExtSetting {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.tg.Add(); err != nil {
		h.log.Crit("Call to StorageHostExtSetting fail")
	}

	defer h.tg.Done()
	// mock the return of host external settings
	return h.externalSettings()
}

// Financial Metrics contains the information about the activities,
// commitments, rewards of host
func (h *StorageHost) FinancialMetrics() HostFinancialMetrics {
	h.mu.RUnlock()
	defer h.mu.RUnlock()
	if err := h.tg.Add(); err != nil {
		h.log.Crit("Fail to add FinancialMetrics Getter to thread manager")
	}
	defer h.tg.Done()

	return h.financialMetrics
}

// TODO: not sure the exact procedure
// TODO: For debugging purpose, currently use vargs and tag for directly ignore the
//  checking parts, set the settings and increase the revisionNumber, for future
//  development, please follow the logic to make the test case success as expected,
//  or delete and  do another test case for convenience
func (h *StorageHost) SetIntSetting(settings StorageHostIntSetting, debug ...bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.tg.Add(); err != nil {
		h.log.Crit("Fail to add StorageHostIntSetting Getter to thread manager")
		return err
	}
	defer h.tg.Done()

	// for debugging purpose, just jump to the last part, so it won't be affected
	// by the implementation of checking parts (check unlock hash and network address)
	if debug != nil && len(debug) >= 1 && debug[0] {
		goto LOADSETTING
	}

	// TODO: check the unlock hash, if does not need the hash, remove this part of code
	if settings.AcceptingContracts {
		err := h.checkUnlockHash()
		if err != nil {
			return errors.New("no unlock hash, stop updating: " + err.Error())
		}
	}

	// TODO: Checking the NetAddress

LOADSETTING:
	h.settings = settings
	h.revisionNumber++

	// synchronize the setting to file
	if err := h.syncSetting(); err != nil {
		return errors.New("internal setting update fail: " + err.Error())
	}

	return nil
}

// Return the internal setting of host
func (h *StorageHost) InternalSetting() StorageHostIntSetting {
	h.mu.RLock()
	defer h.mu.RUnlock()
	// if not able to add to the thread manager, simply return a empty setting structure
	if err := h.tg.Add(); err != nil {
		return StorageHostIntSetting{}
	}
	defer h.tg.Done()
	return h.settings
}

// 1. init the database
// 2. load the setting from file
// 3. load from database
// 4. if the setting file not found, create the setting file, and use the default setting
// 5. finally synchronize the data to setting file
func (h *StorageHost) load() error {
	var err error

	// Initialize the database
	if err = h.initDB(); err != nil {
		h.log.Crit("Unable to initialize the database: " + err.Error())
		return err
	}

	// try to load from the setting files,
	if err = h.loadFromFile(); err == nil {
		// TODO: mock the loading from database
		err = h.loadFromDB()
		return err
	} else if !os.IsNotExist(err) {
		// if the error is NOT caused by FILE NOT FOUND Exception
		return err
	}

	// At this step, the error is caused by FILE NOT FOUND Exception
	// Create the setting file
	h.log.Info("Creat a new HostSetting file")

	// currently the error is caused by file not found exception
	// create the file
	if file, err := os.Create(filepath.Join(h.persistDir, HostSettingFile)); err != nil {
		// if the error is throw when create the file
		// close the file and directly return the error
		_ = file.Close()
		return err
	} else {
		// assert the error is nil, close the file
		if err := file.Close(); err != nil {
			h.log.Info("Unable to close the setting file")
		}
	}

	// load the default settings
	h.loadDefaults()

	// and get synchronization
	if syncErr := h.syncSetting(); syncErr != nil {
		h.log.Warn("Tempt to synchronize setting to file failed: " + syncErr.Error())
	}

	return nil
}

// load host setting from the file, guarantee that host would not be
// modified if error happen
func (h *StorageHost) loadFromFile() error {
	h.mu.Lock()
	defer h.mu.Unlock()

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

// loads the default setting for the host
func (h *StorageHost) loadDefaults() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.settings = loadDefaultSetting()
}
