// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/log"
	"os"
	"path/filepath"
)

type (
	storageManager struct {
		// sectorSalt is the salt used to generate the sector id with merkle root
		sectorSalt sectorSalt

		// database is the db that wraps leveldb. Folders and Sectors metadata info are
		// stored in database
		db *database

		// folders is a in-memory map of the folder
		folders *folderManager

		// sectorLocks is the map from sector id to the sectorLock
		sectorLocks *sectorLocks

		// utility field
		log        log.Logger
		persistDir string
		wal        *writeaheadlog.Wal
		tm         *threadmanager.ThreadManager
	}

	sectorSalt [32]byte
)

// TODO: Test new start close routine.

// New create a new storage manager
func New(persistDir string) (sm *storageManager, err error) {
	sm.db, err = openDB(filepath.Join(persistDir, databaseFileName))
	if err != nil {
		return nil, fmt.Errorf("cannot create the storagemanager: %v", err)
	}
	sm.sectorLocks = newSectorLocks()
	sm.log = log.New("module", "storage manager")
	sm.persistDir = persistDir
	// Only initialize the WAL in start
	sm.tm = &threadmanager.ThreadManager{}
	return
}

// Start start the storage manager
func (sm *storageManager) Start() (err error) {
	// generate or get the sector salt. The sector salt is constant across host's lifetime
	sm.sectorSalt, err = sm.db.getOrCreateSectorSalt()
	if err != nil {
		return fmt.Errorf("cannot get or create the sector salt: %v", err)
	}
	// load folders metadata from the db
	if err = sm.loadFolderManager(); err != nil {

	}

	// Open the wal
	var txns []*writeaheadlog.Transaction
	sm.wal, txns, err = writeaheadlog.New(filepath.Join(sm.persistDir, walFileName))
	if err != nil {
		return fmt.Errorf("cannot open the wal: %v", err)
	}
	// Create goroutines go process unfinished transactions
	for _, txn := range txns {
		// If the module is stopped, return
		if sm.stopped() {
			return nil
		}
		// define the process target
		var target uint8
		if txn.Committed() {
			target = targetRecoverCommitted
		} else {
			target = targetRecoverUncommitted
		}
		// decode the update
		up, err := decodeFromTransaction(txn)
		if err != nil {
			if len(txn.Operations) > 0 {
				sm.log.Warn("Cannot decode transaction", "update", txn.Operations[0].Name)
			} else {
				sm.log.Warn("Cannot decode transaction. wal might be corrupted")
			}
			continue
		}
		// start a thread to process
		err = sm.tm.Add()
		if err != nil {
			return
		}
		go sm.prepareProcessReleaseUpdate(up, target)
	}
	return nil
}

// Close close the storage manager
func (sm *storageManager) Close() (fullErr error) {
	// Stop the thread manager
	err := sm.tm.Stop()
	fullErr = common.ErrCompose(fullErr, err)

	// Close db
	sm.db.close()

	// Close storage folder
	err = sm.folders.close()
	fullErr = common.ErrCompose(fullErr, err)
}

// loadFolderManager load storage folders from database and open the data files
func (sm *storageManager) loadFolderManager() (err error) {
	// load the folders from database
	folders, err := sm.db.loadAllStorageFolders()
	if err != nil {
		return
	}
	for _, sf := range folders {
		// load the folder data file
		if err = sf.load(); err != nil {
			return fmt.Errorf("load folder %v: %v", sf.path, err)
		}
	}
	sm.folders = &folderManager{
		sfs: folders,
	}
	return
}

// stopped return whether the current storage manager is stopped
func (sm *storageManager) stopped() bool {
	select {
	case <-sm.tm.StopChan():
		return true
	default:
	}
	return false
}
