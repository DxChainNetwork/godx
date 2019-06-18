package newstoragemanager

import (
	"os"
	"path/filepath"
)

// ResizeFolder resize the folder to specified size
func (sm *storageManager) ResizeFolder(folderPath string, size uint64) (err error) {
	// Read the folder numSectors
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sf, err := sm.folders.getWithoutLock(folderPath)
	if err != nil {
		return err
	}
	targetNumSectors := sizeToNumSectors(size)
	if targetNumSectors == sf.numSectors {
		// No need to resize
		return nil
	} else if targetNumSectors > sf.numSectors {
		// expand the folder
		return sm.expandFolder(folderPath, size)
	} else {
		// shrink the folder
		return sm.shrinkFolder(folderPath, size)
	}
}

// DeleteFolder delete the folder
func (sm *storageManager) DeleteFolder(folderPath string) (err error) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sf, err := sm.folders.getWithoutLock(folderPath)
	if err != nil {
		return err
	}
	if sf.numSectors == 0 {
		return nil
	}

	var haveErr bool
	if err = sm.shrinkFolder(folderPath, uint64(0)); err != nil {
		upErr, ok := err.(*updateError)
		if !ok {
			haveErr = true
		} else {
			haveErr = !upErr.isNil()
		}
	}
	if haveErr {
		return err
	}
	// Delete the file and the folder
	if err = sm.db.deleteStorageFolder(sf); err != nil {
		return err
	}
	if err = sf.dataFile.Close(); err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(sf.path, dataFileName)); err != nil {
		return err
	}
	return nil
}
