package storagemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type configPersist struct {
	SectorSalt [32]byte
	Folders    []folderPersist
}

type folderPersist struct {
	Index uint16
	Path  string
	Usage []BitVector
}

func (sm *storageManager) extractConfig() *configPersist {
	return &configPersist{
		SectorSalt: sm.sectorSalt,
		Folders:    sm.extractFolderList(),
	}
}

func (sm *storageManager) extractFolderList() []folderPersist {
	folders := make([]folderPersist, len(sm.folders))
	var idx int
	for _, folder := range sm.folders {
		folders[idx] = folder.extractFolder()
		idx++
	}
	return folders
}

func (sf *storageFolder) extractFolder() folderPersist {
	return folderPersist{
		Index: sf.index,
		Path:  sf.path,
	}
}

// syncResources loop through all the folder,
// and synchronize the sector data and metadata
func (sm *storageManager) syncFiles(wg *sync.WaitGroup) {

	for _, sf := range sm.folders {
		if atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
			continue
		}

		wg.Add(2)

		// syn the metadata files
		go func(sf *storageFolder) {
			defer wg.Done()
			err := sf.sectorMeta.Sync()
			if err != nil {
				// TODO: log the failure
			}
		}(sf)

		// sync the data file
		go func(sf *storageFolder) {
			defer wg.Done()
			err := sf.sectorData.Sync()
			if err != nil {
				// TODO: log the failure
			}
		}(sf)
	}

}

// synchronize the wal file
func (sm *storageManager) syncWAL() {
	// avoid null pointer exception
	if sm.wal.walFileTmp == nil {
		return
	}

	// force to synchronize the wal
	err := sm.wal.walFileTmp.Sync()
	if err != nil {
		// TODO: log the failure, but no return
	}

	// save the temporary to solid wal file
	if err = os.Rename(filepath.Join(sm.persistDir, walFileTmp),
		filepath.Join(sm.persistDir, walFile)); err != nil {
	}
}

// syncConfig synchronize the config file and change the name of
// temporary config to config
func (sm *storageManager) syncConfig() {
	// if the config file is nil, no need to persist
	if sm.wal.configTmp == nil {
		return
	}

	// if the temporary config file exist, persist it to a config file
	// force to synchronize the temporary file to disk
	err := sm.wal.configTmp.Sync()
	if err != nil {
		// TODO: log the failure, but no return
	}

	// rename to rewrite the original config file
	if err = os.Rename(filepath.Join(sm.persistDir, configFileTmp),
		filepath.Join(sm.persistDir, configFile)); err != nil {
		// TODO: log the failure
	}
}

func (sm *storageManager) syncConfigForce() error {
	config := sm.extractConfig()
	return common.SaveDxJSON(configMetadata,
		filepath.Join(sm.persistDir, configFile), config)
}
