// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"fmt"
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
		folders map[folderID]*storageFolder

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
	sm.folders, err = sm.db.loadAllStorageFolders()
	if err != nil {
		return fmt.Errorf("cannot load all storage folders: %v", err)
	}
	// Open the wal
	var txns []*writeaheadlog.Transaction
	sm.wal, txns, err = writeaheadlog.New(filepath.Join(sm.persistDir, walFileName))
	if err != nil {
		return fmt.Errorf("cannot open the wal: %v", err)
	}
	// Create goroutines go process unfinished transactions
	for _, txn := range txns {
		err = sm.tm.Add()
		if err != nil {
			return
		}
		go sm.loopProcessTxn(txn)
	}
	return nil
}

// loopProcessTxn loops processing the transaction, until failed for numConsecutiveFailsRelease times,
// or the processing succeed.
func (sm *storageManager) loopProcessTxn(txn *writeaheadlog.Transaction, target uint8) {
	upErr := newUpdateError()
	// decode the update from transaction
	up, err := decodeFromTransaction(txn)
	// If cannot decode from the transaction, return at once
	if err != nil {
		sm.logError(up, upErr.setPrepareError(err).setReleaseError(txn.Release()))
		return
	}
	// register the defer function
	defer func() {
		sm.cleanUp(up, upErr)
	}()

	// try to process the update for numConsecutiveFailsRelease times
	for numFailedTimes := 0; numFailedTimes != numConsecutiveFailsRelease; numFailedTimes++ {
		select {
		case <-sm.tm.StopChan():
			return
		default:
		}
		err = up.apply(sm)
		// If the storage manager has been stopped. reverse the transaction and
		// return
		if err == errStopped {
			err = up.reverse(sm)
			upErr = upErr.addReverseError(err)
			return
		}
		// If there is no error happened, clear the previous error and return
		if err == nil {
			upErr = newUpdateError()
			return
		}
		// If some other error happened, add the err to the fullErr and then
		// reverse the transaction
		if err != nil {
			upErr = upErr.addProcessError(err).addReverseError(up.reverse(sm))
			continue
		}
		// wait for processInterval and try again
		select {
		case <-sm.tm.StopChan():
			return
		case <-time.After(processInterval):
		}
	}
}

// cleanUp do the post process Transaction work
func (sm *storageManager) cleanUp(up update, err *updateError) {
	sm.logError(up, err.setReleaseError(up.transaction().Release()))
}
