package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"os"
)

type (
	addStorageFolderUpdate struct {
		path string
		size uint64
	}
)

// addStorageFolder add a storageFolder. The function could be called with a goroutine
func (sm *storageManager) addStorageFolder(path string, size uint64) (err error) {
	// Register in the thread manager
	if err = sm.tm.Add(); err != nil {
		return
	}
	defer sm.tm.Done()

	// validate the add storage folder
	if err = sm.validateAddStorageFolder(path, size); err != nil {
		return
	}
	// create the update and record the intent
	update := NewAddStorageFolderUpdate(path, size)

	// record the update intent
	if err = update.recordIntent(sm); err != nil {
		err = fmt.Errorf("cannot record the intent for %v: %v", update.str(), err)
	}
	return
}

// validateAddStorageFolder validate the add storage folder request. Return error if validation failed
func (sm *storageManager) validateAddStorageFolder(path string, size uint64) error {
	// Check numSectors
	numSectors := sizeToNumSectors(size)
	if numSectors < minSectorsPerFolder {
		return fmt.Errorf("size too small")
	}
	if numSectors > maxSectorsPerFolder {
		return fmt.Errorf("size too large")
	}
	// check whether the folder path already exists
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return fmt.Errorf("folder already exist: %v", path)
	}
	// check whether the folders has exceed limit
	if size := sm.folders.size(); size >= maxNumFolders {
		return fmt.Errorf("too many folders to manager")
	}
	return nil
}

// NewAddStorageFolderUpdate create a new addStorageFolderUpdate
// Note the size in the update is not the same as the input
func NewAddStorageFolderUpdate(path string, size uint64) (update *addStorageFolderUpdate) {
	numSectors := sizeToNumSectors(size)
	update = &addStorageFolderUpdate{
		path: path,
		size: numSectors * storage.SectorSize,
	}
	return
}

// str defines the user friendly string of the update
func (update *addStorageFolderUpdate) str() (s string) {
	// TODO: user friendly formatted print for size
	s = fmt.Sprintf("Add storage folder [%v] of %v byte", update.path, update.size)
	return
}

func (update *addStorageFolderUpdate) recordIntent(manager *storageManager) (err error) {

	return
}

func (update *addStorageFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
	return
}

func (update *addStorageFolderUpdate) process(manager *storageManager, target uint8) (err error) {
	return
}

func (update *addStorageFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	return
}
