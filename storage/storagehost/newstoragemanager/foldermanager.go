package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"sync"
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
		err = common.ErrCompose(err, sf.dataFile.Close())
	}
	return
}

// exist check whether the folder id is in the folderManager
func (fm *folderManager) exist(path string) (exist bool) {
	fm.lock.Lock()
	defer fm.lock.Unlock()

	_, exist = fm.sfs[path]
	return
}

func (fm *folderManager) size() (size int) {
	fm.lock.Lock()
	defer fm.lock.Unlock()

	size = len(fm.sfs)
	return
}
