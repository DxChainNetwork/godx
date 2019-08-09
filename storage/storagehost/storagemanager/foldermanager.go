// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"errors"
	"fmt"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

// folderManager is the map from folder id to storage folder
type folderManager struct {
	sfs map[string]*storageFolder
}

// loadFolderManager creates a new storage folders from database and open the data files
func loadFolderManager(db *database) (fm *folderManager, err error) {
	// load the folders from database
	folders, err := db.loadAllStorageFolders()
	if err != nil {
		return
	}
	for _, sf := range folders {
		// load the folder data file
		if err = sf.load(); err != nil {
			err = fmt.Errorf("load folder %v: %v", sf.path, err)
			return
		}
	}
	fm = &folderManager{
		sfs: folders,
	}
	return
}

// close close all files in the storage folders
func (fm *folderManager) close() (err error) {
	for _, sf := range fm.sfs {
		err = common.ErrCompose(err, sf.dataFile.Close())
	}
	return
}

// exist check whether the folder id is in the folderManager.
// The function is not thread safe to use
func (fm *folderManager) exist(path string) (exist bool) {
	_, exist = fm.sfs[path]
	return
}

// get get a storage folder specified by path from the folder manager.
func (fm *folderManager) get(path string) (sf *storageFolder, err error) {
	sf, exist := fm.sfs[path]
	if !exist {
		return nil, errors.New("path not exist")
	}
	return sf, nil
}

// getFolders return the map from folder id to the folders
func (fm *folderManager) getFolders(folderPaths []string) (folders map[folderID]*storageFolder, err error) {
	folders = make(map[folderID]*storageFolder)
	for _, path := range folderPaths {
		sf, exist := fm.sfs[path]
		if !exist {
			return make(map[folderID]*storageFolder), fmt.Errorf("folder not exist")
		}
		folders[sf.id] = sf
	}
	return
}

// delete delete the entry in folder manager
func (fm *folderManager) delete(path string) {
	delete(fm.sfs, path)
}

// size return the size in the folder manager
func (fm *folderManager) size() (size int) {
	size = len(fm.sfs)
	return
}

// add add a storageFolder to the folder manager.
func (fm *folderManager) addFolder(sf *storageFolder) (err error) {
	if _, exist := fm.sfs[sf.path]; exist {
		err = errors.New("path already exist")
	}
	fm.sfs[sf.path] = sf
	return nil
}

// selectFolderToAdd select a folder to add sector. return a storageFolder, the
// index to insert, and error that happened during execution
func (fm *folderManager) selectFolderToAdd() (sf *storageFolder, index uint64, err error) {
	// Loop over the folder manager to check availability
	for _, sf = range fm.sfs {
		if sf.status == folderUnavailable {
			continue
		}
		index, err = sf.freeSectorIndex()
		if err == errFolderAlreadyFull {
			continue
		} else if err != nil {
			return nil, 0, err
		}
		// return the storage folder, the index, and nil error
		return
	}

	// After loop over all folders, still no available slot found
	return nil, 0, errAllFoldersFullOrUsed
}

// selectFolderToAddWithRetry execute selectFolderToAdd retryTimes, If no error, return
func (fm *folderManager) selectFolderToAddWithRetry(retryTimes int) (sf *storageFolder, index uint64, err error) {
	for i := 0; i != retryTimes; i++ {
		sf, index, err = fm.selectFolderToAdd()
		if err == nil {
			return
		}
		<-time.After(100 * time.Millisecond)
	}
	return nil, 0, err
}

// validateShrink validates the shrinkFolderUpdate for whether all the stored sectors
// could be stored in the folders.
func (fm *folderManager) validateShrink(folderPath string, targetNumSector uint64) (err error) {
	freeSectors := uint64(0)
	for path, sf := range fm.sfs {
		if path == folderPath {
			continue
		}
		freeSectors += sf.numSectors - sf.storedSectors
	}
	freeSectors += targetNumSector
	if freeSectors < fm.sfs[folderPath].storedSectors {
		return fmt.Errorf("not enough storage space for shrink")
	}
	return
}
