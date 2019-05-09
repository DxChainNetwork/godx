package storagemanager

import (
	"errors"
	"os"
	"path/filepath"
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
		path:  path,
		usage: make([]BitVector, sectors/granularity),
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

	// extract the config for the folder and write into log
	sfConfig := sf.extractFolder()
	err = sm.wal.writeEntry(logEntry{PrepareAddStorageFolder: []folderPersist{sfConfig}})
	if err != nil {
		return nil, errors.New("cannot write wal")
	}

	// manage map the folder
	sm.folderLock.Lock()
	defer sm.folderLock.Unlock()

	// TODO: if this is map. how to handle when sector need to write data to this folder
	sm.folders[sf.index] = sf

	return sf, nil
}

// TODO: revert add storage folder
// TODO: create would redo the already existing file

// processAddStorageFolder start to create metadata files and sector file
// for the folder object. Also include truncating
func (sm *storageManager) processAddStorageFolder(sf *storageFolder) error {
	sf.lock.Lock()
	defer sf.lock.Unlock()

	var err error

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

	// lock wal to write entry
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// extract the folder information and write to entry
	sfConfig := sf.extractFolder()
	err = sm.wal.writeEntry(logEntry{ProcessedAddStorageFolder: []folderPersist{sfConfig}})
	if err != nil {
		return errors.New("cannot write wal, consider crashing in future")
	}

	// re update the folder mapping
	sm.folderLock.Lock()
	defer sm.folderLock.Unlock()

	sm.folders[sf.index] = sf

	return nil
}

func findUnprocessedFolderAddition(entries []logEntry) []folderPersist {
	entryMap := make(map[uint16]folderPersist)

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

		// TODO: delete the folder in removal stage
	}

	var folders []folderPersist
	for _, sf := range entryMap {
		folders = append(folders, sf)
	}
	return folders
}
