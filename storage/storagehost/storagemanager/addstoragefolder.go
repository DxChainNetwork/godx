package storagemanager

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"os"
	"path/filepath"
	"sync/atomic"
)

// prepareAddStorageFolder manage the first step for adding a folder
func (sm *storageManager) prepareAddStorageFolder(path string, size uint64) (*storageFolder, error) {
	var err error

	// check the number of sectors
	sectors := size / SectorSize

	// check if the number of sector is less than maximum limitation
	if sectors > MaxSectorPerFolder {
		return nil, errors.New("folder size is too large")
	}

	// check if the number of sector is more than minimum limitation
	if sectors < MinSectorPerFolder {
		return nil, errors.New("folder size is too small")
	}

	// check if sectors could be map to each granularity exactly
	if sectors%granularity != 0 {
		return nil, errors.New("folder size should be a multiple of 64")
	}

	// if the mode is not standard mode, a relative path is allowed to use
	// for simplify the testing
	if Mode == STD && !filepath.IsAbs(path) {
		return nil, errors.New("given path is not an absolute path")
	}

	// create a folder object
	sf := &storageFolder{
		path:        path,
		usage:       make([]BitVector, sectors/granularity),
		freeSectors: make(map[sectorID]uint32),
		folderLock:  new(folderLock),
	}

	// in order to prevent duplicate add
	// or create a folder which adding in progress use the wal
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// check if the number of folder does not exceed the limitation
	// this number must fits the folder which is adding in progress
	if uint64(len(sm.folders)) >= MaxStorageFolders {
		return nil, errors.New("reach the limitation of maximum number of folder")
	}

	// loop through the folders in memory, if the folder intend to
	// override a existing storage folder, return an error
	for _, f := range sm.folders {
		if path == f.path {
			return nil, ErrFolderAlreadyExist
		}
	}

	//find a random index for folder
	sf.index, err = randomFolderIndex(sm.folders)
	if err != nil {
		return nil, err
	}

	// lock the folder, mark as cannot use until the folder addition been processed
	sf.folderLock.Lock()
	sm.folders[sf.index] = sf

	// extract the config for the folder and write into log
	sfConfig := extractFolder(*sf)
	err = sm.wal.writeEntry(logEntry{PrepareAddStorageFolder: []folderPersist{sfConfig}})
	if err != nil {
		return nil, errors.New("cannot write wal")
	}

	return sf, nil
}

// processAddStorageFolder start to create metadata files and sector file
// for the folder object. Also include truncating
func (sm *storageManager) processAddStorageFolder(sf *storageFolder) error {
	// at the end of the operation, the folder can be used, do unlock
	defer sf.folderLock.Unlock()
	var err error

	// defer the error handling process
	defer func(sf *storageFolder) {
		if err == nil {
			return
		}
		// if the error is caused by the existing of sector or meta file
		// just cancel the operation
		if err == ErrDataFileAlreadyExist {
			err = common.ErrCompose(err, sm.cancelAddStorageFolder(sf))
			return
		}

		// if the error is caused by other error, need to revert the operation
		err = common.ErrCompose(err, sm.revertAddStorageFolder(sf))
	}(sf)

	// this step is to make sure the metadata and sector data not exist
	// int the folder, to avoid the override of data
	_, metaErr := os.Stat(filepath.Join(sf.path, sectorMetaFileName))
	_, dataErr := os.Stat(filepath.Join(sf.path, sectorDataFileName))
	err = common.ErrCompose(metaErr, dataErr)
	if err == nil {
		// if any of them can be checked without error,
		// means the sector and metadata exist in the folder
		err = ErrDataFileAlreadyExist
		return err
	} else if !os.IsNotExist(metaErr) || !os.IsNotExist(dataErr) {
		// if the error is not caused by non existing of file
		// TODO: log warning or crash
	}

	// create all the directory
	if err = os.MkdirAll(sf.path, 0700); err != nil {
		return err
	}

	// create the metadata file
	sf.sectorMeta, err = os.Create(filepath.Join(sf.path, sectorMetaFileName))
	if err != nil {
		return err
	}

	// create the sector file
	sf.sectorData, err = os.Create(filepath.Join(sf.path, sectorDataFileName))
	if err != nil {
		return err
	}

	if Mode == TST && MockFails["ADD_FAIL"] {
		err = ErrMock
		return err
	}

	// try to truncate the sector file
	err = sf.sectorData.Truncate(int64(len(sf.usage) * granularity * int(SectorSize)))
	if err != nil {
		return err
	}

	// try to truncate the meta file
	err = sf.sectorMeta.Truncate(int64(len(sf.usage) * granularity * int(SectorMetaSize)))
	if err != nil {
		return err
	}

	// lock the wal
	sm.wal.lock.Lock()

	// re update the folder mapping
	sm.folders[sf.index] = sf

	if Mode == TST && MockFails["ADD_EXIT"] {
		sm.wal.lock.Unlock()
		sm.stopChan <- struct{}{}
		return nil
	}

	// extract the folder information and write to entry
	sfConfig := extractFolder(*sf)
	err = sm.wal.writeEntry(logEntry{ProcessedAddStorageFolder: []folderPersist{sfConfig}})
	if err != nil {
		sm.wal.lock.Unlock()
		return errors.New("cannot write wal, consider crashing in future")
	}

	sm.wal.lock.Unlock()

	return nil
}

// revertAddStorageFolder revert the change of the storage folder
func (sm *storageManager) revertAddStorageFolder(sf *storageFolder) error {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	var err error

	// delete the folder from the mapping
	delete(sm.folders, sf.index)

	// extract the information of folder to be removed
	// and record the folder information to the log
	folder := extractFolder(*sf)
	err = sm.wal.writeEntry(logEntry{RevertAddStorageFolder: []folderPersist{folder}})
	if err != nil {
		// TODO : consider just crashing the system to prevent corruption
		return errors.New("cannot write wal, consider crashing in the future")
	}

	// close and remove the sector file, metadata file

	err = common.ErrCompose(err, sf.sectorData.Close())
	err = common.ErrCompose(err, sf.sectorMeta.Close())
	// using of removeAll to simplify the case that the sector or meta file have not been create yet
	err = common.ErrCompose(err, os.RemoveAll(filepath.Join(sf.path, sectorDataFileName)))
	err = common.ErrCompose(err, os.RemoveAll(filepath.Join(sf.path, sectorMetaFileName)))

	return err
}

func (sm *storageManager) cancelAddStorageFolder(sf *storageFolder) error {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// TODO: dangerous operation, may clean out an already mapped memory
	// delete the folder from the mapping
	delete(sm.folders, sf.index)

	// extract the information of folder to be removed
	// and record the folder information to the log
	folder := extractFolder(*sf)
	err := sm.wal.writeEntry(logEntry{CancelAddStorageFolder: []folderPersist{folder}})
	if err != nil {
		//TODO: consider crashing the system to prevent corruption
		return err
	}
	return nil
}

// find the already processed folder
func findProcessedFolderAddition(entries []logEntry) []folderPersist {
	entryMap := make(map[uint16]folderPersist)
	// loop through the entry and find the processed addition
	for _, entry := range entries {
		for _, sf := range entry.ProcessedAddStorageFolder {
			entryMap[sf.Index] = sf
		}
	}
	// convert the map to array
	var folders []folderPersist
	for _, sf := range entryMap {
		folders = append(folders, sf)
	}
	return folders
}

// find all the unprocessed folder addition, return a array of folder
// which have not start to do operation yet
func findUnprocessedFolderAddition(entries []logEntry) []folderPersist {
	entryMap := make(map[uint16]folderPersist)

	// loop through the entries
	for _, entry := range entries {
		// add all the folder in prepare stage
		for _, sf := range entry.PrepareAddStorageFolder {
			entryMap[sf.Index] = sf
		}

		// delete all the folder processed add operation
		for _, sf := range entry.ProcessedAddStorageFolder {
			delete(entryMap, sf.Index)
		}

		// delete all the folder in revert state
		for _, sf := range entry.RevertAddStorageFolder {
			delete(entryMap, sf.Index)
		}

		for _, sf := range entry.CancelAddStorageFolder {
			delete(entryMap, sf.Index)
		}

		// TODO: delete the folder in removal stage
	}

	// convert map to array
	var folders []folderPersist
	for _, sf := range entryMap {
		folders = append(folders, sf)
	}
	return folders
}

// if the prepare stage is logged, which means there is no duplication in folders,
// so no worry about delete data already exist
func (sm *storageManager) clearUnprocessedAddition(unProcessedAddition []folderPersist) {
	var err error
	// check through all unprocessed additions
	for _, sf := range unProcessedAddition {
		f, exists := sm.folders[sf.Index]

		// if the folder not exist
		if !exists {
			if err := os.RemoveAll(sf.Path); err != nil {
				// TODO: log: consider if this is an dangerous operation or not
			}
			continue
		}
		if f.sectorData != nil {
			// close sector metadata file
			if err = f.sectorMeta.Close(); err != nil {
				// TODO: log
			}
			// remove the meta data
			if err = os.RemoveAll(filepath.Join(f.path, sectorMetaFileName)); err != nil {
				// TODO: log
			}
			// close sector data file
			if err = f.sectorData.Close(); err != nil {
				// TODO: log
			}
			// remove the sector data file
			if err = os.RemoveAll(filepath.Join(f.path, sectorDataFileName)); err != nil {
				// TODO: log
			}
		}
		// delete the record from memory
		delete(sm.folders, sf.Index)
	}
}

// commit the processed additions, mainly do create the folder object and load into the memory
func (sm *storageManager) commitProcessedAddition(processedAddition []folderPersist) {
	// loop through the processed additions
	for _, sf := range processedAddition {
		// check if is recorded in the memory
		f, exists := sm.folders[sf.Index]
		// if the folder exist in memory, close the files,
		// and will be reopen after this block in order to get refresh
		if exists {
			// if folder object exist in the memory
			if f.sectorMeta != nil {
				// availability handle below
				_ = f.sectorMeta.Close()
			}

			if f.sectorData != nil {
				// availability handle below
				_ = f.sectorData.Close()
			}
		}

		f = &storageFolder{
			index: sf.Index,
			path:  sf.Path,
			usage: sf.Usage,
		}

		atomic.StoreUint64(&f.atomicUnavailable, 0)

		var err error

		// only the folder could be open can be recorded
		f.sectorMeta, err = os.OpenFile(filepath.Join(sf.Path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			// if there is error, not load the things to memory
			continue
		}

		f.sectorData, err = os.OpenFile(filepath.Join(sf.Path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			// if there is error, not load the things to memory
			continue
		}

		// load the folder to memory
		sm.folders[f.index] = f
	}
}
