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
	"path/filepath"
	"time"
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

		// disrupter is used only for test
		disrupter *disrupter
	}

	sectorSalt [32]byte
)

// New creates a new storage manager with no disrupter
func New(persistDir string) (sm *storageManager, err error) {
	return newStorageManager(persistDir, newDisrupter())
}

// new create a new storage manager with the disrupter
func newStorageManager(persistDir string, d *disrupter) (sm *storageManager, err error) {
	sm = &storageManager{}
	sm.db, err = openDB(filepath.Join(persistDir, databaseFileName))
	if err != nil {
		return nil, fmt.Errorf("cannot create the storagemanager: %v", err)
	}
	sm.sectorLocks = newSectorLocks()
	sm.log = log.New("module", "storage manager")
	sm.persistDir = persistDir
	// Only initialize the WAL in start
	sm.tm = &threadmanager.ThreadManager{}
	sm.disrupter = d
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
	if sm.folders, err = loadFolderManager(sm.db); err != nil {
		return fmt.Errorf("cannot load folder manager: %v", err)
	}

	// Open the wal
	var txns []*writeaheadlog.Transaction
	sm.wal, txns, err = writeaheadlog.New(filepath.Join(sm.persistDir, walFileName))
	if err != nil {
		return fmt.Errorf("cannot open the wal: %v", err)
	}
	// Create goroutines to process unfinished transactions
	// The txn should be processed in reverse order (all recovered transactions are to be reverted)
	for i := len(txns) - 1; i >= 0; i-- {
		// If the module is stopped, return
		if sm.stopped() {
			return nil
		}
		// Wait for the last update to lock the corresponding resource
		<-time.After(20 * time.Millisecond)
		txn := txns[i]
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
			return err
		}
		// lock the resource for the update
		if err = up.lockResource(sm); err != nil {
			sm.log.Warn("Cannot lock the resource for update", "update", up)
			continue
		}
		// This function shall be called with a background thread. Since the error has been
		// logged in prepareProcessReleaseUpdate, it's safe not to handle the error here.
		go func(up update) {
			_ = sm.prepareProcessReleaseUpdate(up, targetRecoverCommitted)
			sm.tm.Done()
		}(up)
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

	_, err = sm.wal.CloseIncomplete()
	fullErr = common.ErrCompose(fullErr, err)

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
