package storagehost

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	tg "github.com/DxChainNetwork/godx/common/threadManager"
	"github.com/DxChainNetwork/godx/log"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
	"github.com/davecgh/go-spew/spew"
	"os"
	"path/filepath"
	"sync"
)

type StorageHost struct {

	// storageHost basic settings
	broadcast          bool
	broadcastConfirmed bool
	revisionNumber     uint64
	settings           StorageHostIntSetting

	// storage host manager for manipulating the file storage system
	sm.StorageManager

	// TODO: field for storageHostContract

	// things for log and persistence
	persistDir string
	log        log.Logger

	// things for thread safety
	mu sync.Mutex
	tg tg.ThreadManager
}

// Initialize the Host
// Guarantee to return a host object, if the making dir or open file error,
// A host with default setting would be returned
func NewStorageHost(persistDir string) (*StorageHost, error) {

	// do a host creation, but incomplete settings
	// TODO: also init the storageHostContract
	host := StorageHost{
		persistDir: persistDir,
	}

	// try to make the dir for storing host files.
	// here is going to check if there is error and the error is not
	// caused by the already existence of folder
	if err := os.MkdirAll(persistDir, 0700); err != nil && !os.IsExist(err) {
		// TODO: log the severe information, consider if fatal the system or not
	}

	_, err := os.Stat(filepath.Join(persistDir, HostSettingFile))

	// if open the file successfully, which mean the data could be load
	if err == nil {
		// load the data from file, if there is no error, return the host
		if err = host.loadFromFile(); err == nil {
			fmt.Println("I load the file")
			return &host, nil
		}
	} else if os.IsNotExist(err) {
		// if the error is the file not exist, creat the file, and load the default setting
		_, err = os.Create(filepath.Join(persistDir, HostSettingFile))

	} else {
		// TODO: handling of some unexpected error
		// some other error
	}

	// something unexpected or no file found, use the default setting
	host.loadDefaults()
	if syncErr := host.SyncSetting(); syncErr != nil{
		err = errors.New(err.Error() + "; " + syncErr.Error())
	}

	return &host, err
}

// close the storage host and persist the data
func (h *StorageHost) Close() error {
	// TODO: no handling of thread manager
	return h.SyncSetting()
}

// load host setting from the file, guarantee that host would not be
// modified if error happen
func (h *StorageHost) loadFromFile() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// load and create a persist from JSON file
	persist := new(persistence)		// NOTE:
	spew.Dump(persist)

	// if it is loaded the file causing the error, directly return the error info
	// and not do any modification to the host
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(h.persistDir, HostSettingFile), persist); err != nil {

		fmt.Println("+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")

		return err
	}

	fmt.Println("-----------------load the things from hostsetting to persist----------------------")
	fmt.Println("from path: " , filepath.Join(h.persistDir, HostSettingFile))

	spew.Dump(persist)

	h.loadPersistence(persist)

	spew.Dump(h)
	return nil
}

// loads the default setting for the host
func (h *StorageHost) loadDefaults() {
	h.mu.Lock()
	defer h.mu.Unlock()

	persist := loadDefaultsPersistence()
	h.broadcast = persist.BroadCast
	h.revisionNumber = persist.RevisionNumber
	h.settings = persist.Settings
}
