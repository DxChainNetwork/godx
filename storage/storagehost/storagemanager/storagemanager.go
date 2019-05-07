package storagemanager

import (
	"sync"

	tm "github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/log"
)

type storageManager struct {
	persistDir string

	// TODO: currently use bytes
	sectorSalt [32]byte
	folders    map[uint16]*storageFolder
	sectors    map[sectorID]*storageSector
	log        log.Logger

	folderLock sync.Mutex // this lock manage to lock the mapping of the folder

	tm  *tm.ThreadManager
	wal *writeAheadLog
}

func New(persistDir string, mode ...int) (*storageManager, error) {
	if mode != nil && len(mode) != 0 {
		buildSetting(mode[0])
	}
	return newStorageManager(persistDir)
}

// use the thread manager to close all the related process
func (cm *storageManager) Close() error {
	return cm.tm.Stop()
}

// AddStorageFolder add the a storage folder by given path and size
// make sure the size is between the range and is multiple of granularity
func (sm *storageManager) AddStorageFolder(path string, size uint64) error {
	var err error

	// prepare for adding the folder, which manage to make sure if the
	// input is valid: size in range, duplicate path, and etc check
	// log the addition as preparation stage
	sf, err := sm.prepareAddStorageFolder(path, size)

	// TODO: in multi threading, the steps prepareAddStorageFolder and processAddStorageFolder
	//  may happen at the same time, may need a channel for waiting all the thread complete the addition
	//  before go into the actual process

	// if the error is not nil, cannot continue the next process stage
	if err != nil {
		return err
	}
	err = sm.processAddStorageFolder(sf)

	return err
}
