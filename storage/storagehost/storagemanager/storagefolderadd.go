package storagemanager

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage/storagehost/storagemanager/bitvector"
)

// TODO: don't directly return the err
// TODO: add this stuff in commit stage
// TODO: what if the folder is in progress but not commit: use wal lock

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

	// if is in the standard mode, no worry is not the absolute path, but
	// we do need the folder be absolute path in standard mode
	if Mode == STD && !filepath.IsAbs(path) {
		return nil, errors.New("given path is not an absolute path")
	}

	// make the dir. If the dir is already exist or the status is not health
	// the circumstance are already be handled in the MkdirAll function,
	// TODO: test what happen if the meta, sector already exist in the folder
	if err = os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	// create a folder object
	sf := &storageFolder{
		path:       path,
		usage:      make([]bitvector.BitVector, sectors/granularity),
		freeSector: make(map[sectorID]uint32),
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

	// TODO: 什么情况下folder没有被记录到config中，可能导致data被重写? 假设已经有folder没有被映射

	// check if the folder is not a duplicate one
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

	// persist the change to the json file
	// log the prepare stage of the folder addition
	persist := sf.extractFolder()
	err = sm.wal.writeEntry(logEntry{PrepareAddStorageFolder: []folderPersist{persist}})
	if err != nil {
		// TODO : consider just crashing the system to prevent corruption
		return nil, errors.New("cannot write wal, consider crashing in the future")
	}

	// this step make change to the storage manager on the
	// mapping of folders
	sm.folderLock.Lock()
	defer sm.folderLock.Unlock()

	// map the new folder to the folder map
	sm.folders[sf.index] = sf

	return sf, nil
}

func (sm *storageManager) processAddStorageFolder(sf *storageFolder) error {
	var err error

	// if at the end, there is error, revert all the things done
	defer func() {
		if err != nil {
			err = common.ErrCompose(err, sm.revertAddStorageFolder(sf))
		}
	}()

	// TODO: if the folder exist but no record, the Create of sector file
	//  and metadata file are possible to clear the original one, dangerous operation,
	//  but seems this is handled by record the formation of folder when
	//  checking duplicate path.

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

	// TODO: synchronize the files

	// if all things work as expected, record the change
	sm.wal.lock.Lock()
	persist := sf.extractFolder()
	err = sm.wal.writeEntry(logEntry{ProcessedAddStorageFolder: []folderPersist{persist}})
	if err != nil {
		// TODO : consider just crashing the system to prevent corruption
		return errors.New("cannot write wal, consider crashing in the future")
	}
	sm.wal.lock.Unlock()

	// TODO: refresh the mapping even though it have already been added?
	sm.folderLock.Lock()
	defer sm.folderLock.Unlock()
	sm.folders[sf.index] = sf

	return nil
}

// revertAddStorageFolder revert the change of the storage folder
func (sm *storageManager) revertAddStorageFolder(sf *storageFolder) error {
	var err error

	// extract the information of folder to be removed
	// and record the folder information to the log
	persist := sf.extractFolder()
	err = sm.wal.writeEntry(logEntry{RevertAddStorageFolder: []folderPersist{persist}})
	if err != nil {
		// TODO : consider just crashing the system to prevent corruption
		return errors.New("cannot write wal, consider crashing in the future")
	}
	sm.folderLock.Lock()
	defer sm.folderLock.Unlock()

	// delete the folder from the mapping
	delete(sm.folders, sf.index)

	// close and remove the sector file, metadata file

	err = common.ErrCompose(err, sf.sectorData.Close())
	err = common.ErrCompose(err, sf.sectorMeta.Close())
	// using of removeAll to simplify the case that the sector or meta file have not been create yet
	err = common.ErrCompose(err, os.RemoveAll(filepath.Join(sf.path, sectorDataFileName)))
	err = common.ErrCompose(err, os.RemoveAll(filepath.Join(sf.path, sectorMetaFileName)))

	return err
}
