// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

const (
	// repairUnfinishedLoopInterval is the interval between two repairUnfinishedDirMetadataUpdate
	repairUnfinishedLoopInterval = time.Second

	// fileWalName is the fileName for the fileWal
	fileWalName = "file.wal"

	// updateWalName is the fileName for the updateWal
	updateWalName = "update.wal"
)

// FileSystem is the structure for a file system that include a FileSet and a DirSet
type FileSystem struct {
	// rootDir is the root directory of the file system
	rootDir storage.SysPath

	// FileSet is the FileSet from module dxfile
	FileSet *dxfile.FileSet

	// DirSet is the DirSet from module dxdir
	DirSet *dxdir.DirSet

	// contractor is the contractor used to give health info for the file system
	contractor contractor

	// fileWal is the wal responsible for storage.InsertUpdate / storage.DeleteUpdate
	// that is used in dxfile and dxdir
	fileWal *writeaheadlog.Wal

	// updateWal is the wal responsible for
	updateWal *writeaheadlog.Wal

	// tm is the thread manager for manage the threads in FileSystem
	tm *threadmanager.ThreadManager

	// unfinishedUpdates is the field for the mapping from DxPath to the directory to be
	// updated
	unfinishedUpdates map[storage.DxPath]*dirMetadataUpdate

	// lock is meant to protect the map unfinishedUpdates
	lock sync.Mutex

	// log is the logger used for file system
	logger log.Logger

	// disrupter is the disrupter used for test cases. In production environment,
	// it should always be an empty disrupter
	disrupter disrupter
}

// NewFileSystem is the public function used for creating a production FileSystem
func New(rootDir storage.SysPath, contractor contractor) *FileSystem {
	d := make(disrupter)
	return newFileSystem(rootDir, contractor, d)
}

// newFileSystem creates a new file system with the disrupter
func newFileSystem(rootDir storage.SysPath, contractor contractor, disrupter disrupter) *FileSystem {
	// create the FileSystem
	return &FileSystem{
		rootDir:           rootDir,
		contractor:        contractor,
		tm:                &threadmanager.ThreadManager{},
		logger:            log.New("filesystem"),
		disrupter:         disrupter,
		unfinishedUpdates: make(map[storage.DxPath]*dirMetadataUpdate),
	}
}

// Start is the function that is called for starting the file system service.
// It open the wals, apply all transactions, and start the thread loopRepairUnfinishedDirMetadataUpdate
func (fs *FileSystem) Start() error {
	// open the fileWal
	if err := fs.loadFileWal(); err != nil {
		return fmt.Errorf("cannot start the file system: %v", err)
	}
	// open the updateWal
	if err := fs.loadUpdateWal(); err != nil {
		return fmt.Errorf("cannot start the file system: %v", err)
	}
	// load fs.DirSet
	var err error
	if fs.DirSet, err = dxdir.NewDirSet(fs.rootDir, fs.fileWal); err != nil {
		return fmt.Errorf("cannot start the file system DirSet: %v", err)
	}
	fs.FileSet = dxfile.NewFileSet(fs.rootDir, fs.fileWal)
	// Start the repair loop
	go fs.loopRepairUnfinishedDirMetadataUpdate()
	return nil
}

// loadFileWal read the fileWal
func (fs *FileSystem) loadFileWal() error {
	fileWalPath := filepath.Join(string(fs.rootDir), fileWalName)
	fileWal, unappliedTxns, err := writeaheadlog.New(fileWalPath)
	if err != nil {
		return fmt.Errorf("cannot start load system fileWal: %v", err)
	}
	for i, txn := range unappliedTxns {
		err = storage.ApplyOperations(txn.Operations)
		if err != nil {
			fs.logger.Warn("cannot apply the operation of file transaction index %d: %v", i, err)
		}
		err = txn.Release()
		if err != nil {
			fs.logger.Warn("cannot release the operation of file transaction index %d: %v", i, err)
		}
	}
	fs.fileWal = fileWal
	return nil
}

// loadUpdateWal load the update Wal from disk, and apply unfinished updates
func (fs *FileSystem) loadUpdateWal() error {
	updateWalPath := filepath.Join(string(fs.rootDir), updateWalName)
	updateWal, unappliedTxns, err := writeaheadlog.New(updateWalPath)
	if err != nil {
		return fmt.Errorf("cannot start file system updateWal: %v", err)
	}
	for i, txn := range unappliedTxns {
		for j, op := range txn.Operations {
			path, err := decodeWalOp(op)
			if err != nil {
				fs.logger.Warn("cannot decode txn[%d].operation[%d]: %v", i, j, err)
			}
			// if error happened: already in progress
			// release the transaction and continue to the next transaction
			if err = fs.updateDirMetadata(path, txn); err == errUpdateAlreadyInProgress {
				txn.Release()
				break
			}
		}
	}
	fs.updateWal = updateWal
	return nil
}

// Close will terminate all threads opened by file system
func (fs *FileSystem) Close() error {
	var fullErr error
	fs.lock.Lock()
	defer fs.lock.Unlock()
	for _, update := range fs.unfinishedUpdates {
		close(update.stop)
	}
	// close wal
	err := fs.fileWal.Close()
	if err != nil {
		fullErr = common.ErrCompose(fullErr, err)
	}
	err = fs.updateWal.Close()
	if err != nil {
		fullErr = common.ErrCompose(fullErr, err)
	}
	return common.ErrCompose(fullErr, fs.tm.Stop())
}

// loopRepairUnfinishedDirMetadataUpdate is the permanent loop for repairing the unfinished
// dirMetadataUpdate.
func (fs *FileSystem) loopRepairUnfinishedDirMetadataUpdate() {
	err := fs.tm.Add()
	if err != nil {
		return
	}
	defer fs.tm.Done()

	for {
		// Stop when dxchain is stopped. Start when interval repairUnfinishedLoopInterval
		// reached
		select {
		case <-fs.tm.StopChan():
			return
		case <-time.After(repairUnfinishedLoopInterval):
		}
		err := fs.repairUnfinishedDirMetadataUpdate()
		if err != nil {
			fs.logger.Warn("loop repair error", "err", err)
		}
	}
}

// repairUnfinishedDirMetadataUpdate Initialize and update all
func (fs *FileSystem) repairUnfinishedDirMetadataUpdate() error {
	// make a copy of the unfinishedUpdates
	unfinishedUpdates := make(map[storage.DxPath]*dirMetadataUpdate)
	fs.lock.Lock()
	for path, update := range fs.unfinishedUpdates {
		unfinishedUpdates[path] = update
	}
	fs.lock.Unlock()

	var err error
	for path, update := range unfinishedUpdates {
		// If the program already stopped, return
		select {
		case <-fs.tm.StopChan():
			return err
		default:
		}
		// Check whether need to init and update the dirMetadata
		updateInProgress := atomic.LoadUint32(&update.updateInProgress)
		if updateInProgress != 0 {
			continue
		}
		// InitAndUpdate all unfinished dirMetadataUpdates
		err = common.ErrCompose(err, fs.InitAndUpdateDirMetadata(path))
	}
	return err
}

// disrupt is the wrapper to disrupt with fs.disrupter
func (fs *FileSystem) disrupt(s string) bool {
	return fs.disrupter.disrupt(s)
}
