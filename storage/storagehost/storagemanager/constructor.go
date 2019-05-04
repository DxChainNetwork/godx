package storagemanager

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"os"
	"path/filepath"
	"sync/atomic"
)

func newStorageManager(persistDir string) (*storageManager, error) {
	sm := &storageManager{
		// loads the persist dir to current object
		persistDir: persistDir,
		// create the logger
		log: log.New(),
		// initialize folders and sectors map
		folders: make(map[uint16]*storageFolder),
		sectors: make(map[sectorID]*storageSector),
	}

	// try to load every thing from the config file
	if err := sm.constructManager(); err != nil {
		return nil, err
	}

	return sm, nil
}

// constructManager construct the manager object through the config file
// if the config file could not be found, use the default setting, if the
// error is not caused by the FILE NOT FOUND Exception, return the error
func (sm *storageManager) constructManager() error {
	var err error

	if err = os.MkdirAll(sm.persistDir, 0700); err != nil {
		return err
	}
	persist := &persistence{}
	err = common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), persist)
	if os.IsNotExist(err) {
		// if the config file could not be found, do the default settings
		return sm.constructManagerDefault()
	} else if err != nil {
		return err
	}

	// loads the storageSector salt to the storageManager
	sm.sectorSalt = persist.SectorSalt
	// create each of the storageFolder through storageFolder factory
	sm.constructFolders(persist.Folders)

	fmt.Println(sm.sectorSalt)

	return nil
}

// constructFolders read from metadata and construct each folder
func (sm *storageManager) constructFolders(folders []folderPersist) {
	var err error

	// loop through each folders, and check the availability
	for _, folder := range folders {
		f := &storageFolder{
			index:      folder.Index,
			path:       folder.Path,
			usage:      folder.Usage,
			freeSector: make(map[sectorID]uint32),
		}

		f.sectorMeta, err = os.OpenFile(filepath.Join(f.path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&f.atomicUnavailable, 1)
			sm.log.Warn("fail to open sector metadata file for folder: ", f.path)
		}

		f.sectorData, err = os.OpenFile(filepath.Join(f.path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&f.atomicUnavailable, 1)
			sm.log.Warn("fail to open sector data file for folder: ", f.path)
		}

		sm.folders[f.index] = f

		// TODO: init the sectors from each folder
	}
}

// constructManagerDefault construct storageManager using default setting
// read a random bytes for salt
func (sm *storageManager) constructManagerDefault() error {
	var err error
	// get a random bytes for sector Salt
	if _, err = rand.Read(sm.sectorSalt[:]); err != nil {
		return err
	}

	// synchronize the configuration
	return sm.syncConfig()
}
