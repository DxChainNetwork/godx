package storagehost

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	tg "github.com/DxChainNetwork/godx/common/threadManager"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
	"os"
	"path/filepath"
	"sync"
)

// TODO: What is a Public key, where to store and find
// TODO: What is a secret key, where to store and find
// TODO: What is a unlock Hash, where to store and find

type StorageHost struct {

	// storageHost basic settings
	broadcast          	bool
	broadcastConfirmed	bool
	blockHeight 		uint64	// TODO: not sure if need or not, just place here

	autoAddress			string
	financialMetrics 	HostFinancialMetrics
	settings           	StorageHostIntSetting
	revisionNumber     uint64

	// storage host manager for manipulating the file storage system
	sm.StorageManagerAPI

	// TODO: field for storageHostContract

	// things for log and persistence
	persistDir string
	log        log.Logger

	// TODO: to store the info of storage obligation
	db         *ethdb.LDBDatabase

	// things for thread safety
	mu sync.RWMutex
	tg tg.ThreadManager
}

// TODO: mock start, register the api that needed
func (h *StorageHost) Start(eth Backend) {
	//for _, api := range apis {
	//	switch typ := reflect.TypeOf(api.Service); typ {
	//	case reflect.TypeOf(&ethapi.PublicNetAPI{}):
	//		sc.net = api.Service.(*ethapi.PublicNetAPI)
	//	case reflect.TypeOf(&ethapi.PrivateAccountAPI{}):
	//		sc.account = api.Service.(*ethapi.PrivateAccountAPI)
	//	default:
	//		continue
	//	}
	//}
}

// Initialize the Host
// Guarantee to return a host object, if the making dir or open file error, A host with
// default setting would be returned. The build
// debug is a vargs,
func NewStorageHost(persistDir string) (*StorageHost, error) {
	// do a host creation, but incomplete settings
	// TODO: init the storageHostObligation
	host := StorageHost{
		persistDir: persistDir,
	}

	var err error
	var tgErr error

	// use the thread manager to close the things open
	defer func(){
		if err != nil{
			tgErr := host.tg.Stop()
			if tgErr != nil{
				err = errors.New(err.Error() + "; " + tgErr.Error())
			}
		}
	}()

	// TODO: log does not working as expected
	// TODO: close the log file by thread manager
	host.log = log.New(filepath.Join(host.persistDir, HostLog))


	// initialize the storage manager
	host.StorageManagerAPI, err = sm.New(filepath.Join(persistDir, StorageManager))
	if err != nil{
		host.log.Error("Error caused by Creating StorageManager: " + err.Error())
		return nil, err
	}

	if tgErr = host.tg.AfterStop(func() error {
		if err := host.StorageManagerAPI.Close(); err != nil{
			host.log.Error("Fail to close storage manager: " + err.Error())
			return err
		}
		return nil
	}); tgErr != nil{
		host.log.Error(tgErr.Error())
	}


	// try to make the dir for storing host files.
	// here is going to check if there is error and the error is not
	// caused by the already existence of folder
	if err = os.MkdirAll(persistDir, 0700); err != nil && !os.IsExist(err) {
		host.log.Crit("Fail to make dir: " + err.Error())
		return nil, err
	}

	// load the data from file or from default setting
	err = host.load()
	if err != nil{
		return nil, err
	}

	if tgErr = host.tg.AfterStop(func() error{
		if err := host.syncSetting(); err != nil{
			host.log.Error("Fail to synchronize to setting file: " + err.Error())
			return err
		}
		return nil
	}); tgErr != nil{
		// just log the cannot syn problem, the does not sever enough to panic the system
		host.log.Error(tgErr.Error())
	}


	// TODO: storageObligation handle

	// TODO: Init the networking

	return &host, nil
}


// 1. init the database
// 2. load the setting from file
// 3. load from database
// 4. if the setting file not found, create the setting file, and use the default setting
// 5. finally synchronize the data to setting file

func (h *StorageHost) load() error{
	var err error

	err = h.initDB()
	if err != nil{
		return err
	}

	// try to load from the file
	if err = h.loadFromFile(); err == nil{
		// TODO: mock the loading from database
		err = h.loadFromDB()
		return err
	}else if !os.IsNotExist(err){
		// if the error is not caused by FILE NOT FOUND Exception
		return err
	}

	// currently the error is caused by file not found exception
	// create the file
	if file, err := os.Create(filepath.Join(h.persistDir, HostSettingFile)); err != nil{
		// if the error is throw when create the file
		// close the file and directly return the error
		_ = file.Close()
		return err
	}else{
		// assert the error is nil, close the file
		if err := file.Close(); err != nil{
			// TODO: just log the close file fail, this does not matter
		}

	}

	// load the default settings
	h.loadDefaults()

	// and get synchronization
	if syncErr := h.syncSetting(); syncErr != nil {
		// TODO: just log the cannot syn err, because this is not a critical problem
	}

	return nil
}

func (h *StorageHost) initDB() error{
	var err error
	h.db, err = ethdb.NewLDBDatabase(filepath.Join(h.persistDir, HostDB), 16, 16)
	if err != nil{
		return err
	}

	// add the close of database to the thread manager
	_ = h.tg.AfterStop(func() error{
		h.db.Close()
		return nil
	})

	// TODO: create the table if not exist

	return nil
}


// loads the settings from the database, if the setting file load sucess
// TODO: load the database, storage obligation, currently mock
func (h *StorageHost) loadFromDB() error{
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

	// TODO: private & public key generate
	// TODO: init Consensus Subscription
}


// close the storage host and persist the data
func (h *StorageHost) Close() error {
	return h.tg.Stop()
}


// TODO: not sure the exact procedure
func (h *StorageHost) SetIntSetting(settings StorageHostIntSetting) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// TODO: thread group handling

	// TODO: check the unlock hash, if does not need the hash, remove this part of code
	if settings.AcceptingContracts {
		err := h.checkUnlockHash()
		if err != nil {
			return errors.New("no unlock hash, stop updating: " + err.Error())
		}
	}

	// TODO: NetAddress may be get automatically
	if settings.NetAddress != "" {
		// TODO: currently mock the checking of net address
		err := IsValidAddress(settings.NetAddress)
		if err != nil {
			return errors.New("invalid NetAddress, stop updating: " + err.Error())
		}
	}

	// TODO: ALL FOLLOWING
	//  make sure the logic is expected by when announcing, auto address is mocked
	if h.settings.NetAddress != settings.NetAddress && settings.NetAddress != h.autoAddress {
		h.broadcast = false
	}

	h.settings = settings
	h.revisionNumber ++

	err := h.syncSetting()
	if err != nil {
		return errors.New("internal setting update fail: " + err.Error())
	}

	return nil
}

// TODO: currently mock the return host external settings
func (h *StorageHost) StorageHostExtSetting() StorageHostExtSetting {
	h.mu.Lock()
	defer h.mu.Unlock()
	err := h.tg.Add()
	if err != nil{
		h.log.Crit("Call to StorageHostExtSetting fail")
	}
	defer h.tg.Done()
	// mock the return of host external settings
	return StorageHostExtSetting{}
}

func (h *StorageHost) InternalSetting() StorageHostIntSetting{
	h.mu.RLock()
	defer h.mu.RUnlock()

	// TODO: thread group

	return h.settings
}


// TODO: here just mock the checking unlock hash part, not sure where to get the unlock hash
func (h *StorageHost) checkUnlockHash() error {
	return nil
}

// TODO: mock checking if the net address is valid
func IsValidAddress(NetAddress string) error {
	return nil
}
