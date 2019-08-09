// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
)

type (
	// StorageManager is the interface to manager storage which will be provided to
	// upper function calls. Supported methods are mutually exclusive
	StorageManager interface {
		Start() error
		Close() error
		// Functions for download and storage responsibilities
		AddSectorBatch(sectorRoots []common.Hash) error
		AddSector(sectorRoot common.Hash, sectorData []byte) error
		DeleteSector(sectorRoot common.Hash) error
		DeleteSectorBatch(sectorRoots []common.Hash) error
		ReadSector(sectorRoot common.Hash) ([]byte, error)
		// Functions from user calls
		AddStorageFolder(path string, size uint64) error
		DeleteFolder(folderPath string) error
		ResizeFolder(folderPath string, size uint64) error
		// Status check
		Folders() []storage.HostFolder
		AvailableSpace() storage.HostSpace
	}

	storageManager struct {
		// sectorSalt is the salt used to generate the sector id with merkle root
		sectorSalt sectorSalt

		// database is the db that wraps leveldb. Folders and Sectors metadata info are
		// stored in database
		db *database

		// folders is a in-memory map of the folder
		folders *folderManager

		// utility field
		log        log.Logger
		persistDir string
		wal        *writeaheadlog.Wal
		tm         *threadmanager.ThreadManager

		// All methods provided are mutually exclusive
		lock sync.RWMutex

		// disruptor is used only for test
		disruptor *disruptor
	}

	sectorSalt [32]byte
)

// New creates a new storage manager with no disruptor
func New(persistDir string) (sm StorageManager, err error) {
	return newStorageManager(persistDir, newDisruptor())
}

// new create a new storage manager with the disruptor
func newStorageManager(persistDir string, d *disruptor) (sm *storageManager, err error) {
	sm = &storageManager{}
	sm.db, err = openDB(filepath.Join(persistDir, databaseFileName))
	if err != nil {
		return nil, fmt.Errorf("cannot create the storagemanager: %v", err)
	}
	sm.log = log.New("module", "storage manager")
	sm.persistDir = persistDir
	// Only initialize the WAL in start
	sm.tm = &threadmanager.ThreadManager{}
	sm.disruptor = d
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
		// start a thread to process. If cannot be added, module has been stopped. Return nil
		err = sm.tm.Add()
		if err != nil {
			return nil
		}
		// This function shall be called with a background thread.
		go func(up update) {
			sm.lock.Lock()
			defer func() {
				sm.lock.Unlock()
				sm.tm.Done()
			}()
			// Since the error has been handled in prepareProcessReleaseUpdate, it's safe not to
			// handle the error here.
			_ = sm.prepareProcessReleaseUpdate(up, targetRecoverCommitted)
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

// ResizeFolder resize the folder to specified size
func (sm *storageManager) ResizeFolder(folderPath string, size uint64) (err error) {
	// Change the folderPath to absolute path
	if folderPath, err = absolutePath(folderPath); err != nil {
		return
	}
	// Read the folder numSectors
	if sizeToNumSectors(size) > maxSectorsPerFolder {
		return fmt.Errorf("folder size too large")
	}
	if sizeToNumSectors(size) < minSectorsPerFolder {
		return fmt.Errorf("folder size too small")
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sf, err := sm.folders.get(folderPath)
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
	// Change the folderPath to absolute path
	if folderPath, err = absolutePath(folderPath); err != nil {
		return
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	sf, err := sm.folders.get(folderPath)
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
	sm.folders.delete(folderPath)
	if err = sf.dataFile.Close(); err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(sf.path, dataFileName)); err != nil {
		return err
	}
	return nil
}

// Folders return all used folders
func (sm *storageManager) Folders() []storage.HostFolder {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	var folders []storage.HostFolder
	for _, sf := range sm.folders.sfs {
		folders = append(folders, storage.HostFolder{
			Path:         sf.path,
			TotalSectors: sf.numSectors,
			UsedSectors:  sf.storedSectors,
		})
	}
	return folders
}

// AvailableSpace return the host storage space infos
func (sm *storageManager) AvailableSpace() storage.HostSpace {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	var totalSectors, usedSectors, freeSectors uint64
	for _, sf := range sm.folders.sfs {
		totalSectors += sf.numSectors
		usedSectors += sf.storedSectors
		freeSectors += sf.numSectors - sf.storedSectors
	}
	return storage.HostSpace{
		TotalSectors: totalSectors,
		UsedSectors:  usedSectors,
		FreeSectors:  freeSectors,
	}
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

// absolutePath convert the path to abs path
func absolutePath(path string) (absPath string, err error) {
	usr, _ := user.Current()
	dir := usr.HomeDir

	if path == "~" {
		// Apply user home directory
		absPath = dir
	} else if strings.HasPrefix(path, "~/") {
		// Join user home directory with path
		absPath = filepath.Join(dir, path[2:])
	} else {
		// use the absolute path
		absPath, err = filepath.Abs(path)
		if err != nil {
			return
		}
	}
	return
}
