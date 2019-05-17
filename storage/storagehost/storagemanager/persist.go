package storagemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// configPersist used to represent the data to be
// stored in json file
type configPersist struct {
	SectorSalt [32]byte
	Folders    []folderPersist
}

// folderPersist represent the folder information to be
// stored in json file
type folderPersist struct {
	Index uint16
	Path  string
	Usage []BitVector
}

// sectorPersist represent the sector information to be stored
// in json file
type sectorPersist struct {
	Count  uint16
	Folder uint16
	ID     sectorID
	Index  uint32
}

// extractConfig extract the configuration from given
// storage manager, includes its sector salt and folder config
func extractConfig(sm storageManager) *configPersist {
	return &configPersist{
		SectorSalt: sm.sectorSalt,
		Folders:    extractFolderList(sm.folders),
	}
}

// extractFolderList extract the configuration from given
// folders map, folders in the map won't be made to modified
func extractFolderList(folders map[uint16]*storageFolder) []folderPersist {
	sfs := make([]folderPersist, len(folders))

	// loop to extract each folders, the folder is pass by value
	// guarantee not making any modification
	var idx int
	for _, folder := range folders {
		sfs[idx] = extractFolder(*folder)
		idx++
	}

	return sfs
}

// extractFolder extract folder's information,
// guarantee not to make any modification on the folder passed in
func extractFolder(sf storageFolder) folderPersist {
	return folderPersist{
		Index: sf.index,
		Path:  sf.path,
		Usage: clearUsage(sf),
	}
}

// clearUsage extract the actual usage information from a folder
// some usage is only marked to be used, but actually not, extract
// but guarantee not to make any modification on the passed in folder
func clearUsage(sf storageFolder) []BitVector {
	var ss = make([]BitVector, len(sf.usage))

	copy(ss, sf.usage)

	for _, sectorIndex := range sf.freeSectors {
		usageIndex := sectorIndex / granularity
		bitIndex := sectorIndex % granularity
		ss[usageIndex].clearUsage(uint16(bitIndex))
	}

	return ss
}

// syncResources loop through all the folder,
// and synchronize the sector data and metadata
// @ Require: the files should not be modified or use at current time
func (sm *storageManager) syncFiles(wg *sync.WaitGroup) {
	defer wg.Done()
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
// @ Require: the files should not be modified or use at current time
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
		fmt.Println(err.Error())
	}

	// rename to rewrite the original config file
	if err = os.Rename(filepath.Join(sm.persistDir, configFileTmp),
		filepath.Join(sm.persistDir, configFile)); err != nil {
		// TODO: log the failure
		fmt.Println(err.Error())
	}
}
