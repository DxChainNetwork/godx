package newstoragemanager

import (
	"errors"
	"fmt"
	"sync"

	"github.com/DxChainNetwork/godx/common"
)

// folderManager is the map from folder id to storage folder
type folderManager struct {
	sfs  map[string]*storageFolder
	lock sync.RWMutex
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
	fm.lock.Lock()
	defer fm.lock.Unlock()

	for _, sf := range fm.sfs {
		sf.lock.Lock()
		err = common.ErrCompose(err, sf.dataFile.Close())
		sf.lock.Unlock()
	}
	return
}

// exist check whether the folder id is in the folderManager.
// The function is not thread safe to use
func (fm *folderManager) exist(path string) (exist bool) {
	_, exist = fm.sfs[path]
	return
}

// get get a already locked storage folder specified by path from the folder manager.
// Please make sure the storage folder is unlocked when finished using.
// Also make sure the folder manager is locked before calling this function
func (fm *folderManager) get(path string) (sf *storageFolder, err error) {
	sf, exist := fm.sfs[path]
	if !exist {
		return nil, errors.New("path not exist")
	}
	sf.lock.Lock()
	return sf, nil
}

// delete delete the entry in folder manager
// Note this function is not thread safe
func (fm *folderManager) delete(path string) {
	delete(fm.sfs, path)
}

// size return the size in the folder manager
func (fm *folderManager) size() (size int) {
	size = len(fm.sfs)
	return
}

// add add a storageFolder to the folder manager.
// Note this function is totally not thread safe either for folder manager or the folder itself.
// Make sure both the folderManager and storageFolder are locked before the function is called
func (fm *folderManager) addFolder(sf *storageFolder) (err error) {
	if _, exist := fm.sfs[sf.path]; exist {
		err = errors.New("path already exist")
	}
	fm.sfs[sf.path] = sf
	return nil
}
