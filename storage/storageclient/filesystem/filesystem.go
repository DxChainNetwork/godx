// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// FileSystem is the structure for a file system that include a FileSet and a DirSet
type FileSystem struct {
	// rootDir is the root directory where the files locates
	rootDir storage.SysPath

	// persistDir is the directory of containing the persist files
	persistDir storage.SysPath

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

	// standardDisrupter is the standardDisrupter used for test cases. In production environment,
	// it should always be an empty standardDisrupter
	disrupter disrupter
}

// New is the public function used for creating a production FileSystem
func New(persistDir string, contractor contractor) *FileSystem {
	d := newStandardDisrupter()
	return newFileSystem(persistDir, contractor, d)
}

// newFileSystem creates a new file system with the standardDisrupter
func newFileSystem(persistDir string, contractor contractor, disrupter disrupter) *FileSystem {
	// create the FileSystem
	return &FileSystem{
		rootDir:           storage.SysPath(filepath.Join(persistDir, filesDirectory)),
		persistDir:        storage.SysPath(persistDir),
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
	// load fs.DirSet
	var err error
	if fs.DirSet, err = dxdir.NewDirSet(fs.rootDir, fs.fileWal); err != nil {
		return fmt.Errorf("cannot start the file system DirSet: %v", err)
	}
	fs.FileSet = dxfile.NewFileSet(fs.rootDir, fs.fileWal)
	// open the updateWal
	if err := fs.loadUpdateWal(); err != nil {
		return fmt.Errorf("cannot start the file system: %v", err)
	}
	// Start the repair loop
	go fs.loopRepairUnfinishedDirMetadataUpdate()
	return nil
}

func (fs *FileSystem) OpenFile(path storage.DxPath) (*dxfile.FileSetEntryWithID, error) {
	return fs.FileSet.Open(path)
}

// loadFileWal read the fileWal
func (fs *FileSystem) loadFileWal() error {
	fileWalPath := filepath.Join(string(fs.persistDir), fileWalName)
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
	updateWalPath := filepath.Join(string(fs.persistDir), updateWalName)
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
		if err != nil && err != errStopped && err != errUpdateAlreadyInProgress {
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
			return errStopped
		default:
		}
		// Check whether need to init and update the dirMetadata
		updateInProgress := atomic.LoadUint32(&update.updateInProgress)
		if updateInProgress != 0 {
			continue
		}
		// InitAndUpdate all unfinished dirMetadataUpdates
		err = common.ErrCompose(err, fs.updateDirMetadata(path, nil))
	}
	return err
}

// disrupt is the wrapper to disrupt with fs.standardDisrupter
func (fs *FileSystem) disrupt(s string) bool {
	return fs.disrupter.disrupt(s)
}

// fileList returns a brief file info list
func (fs *FileSystem) fileList() ([]storage.FileBriefInfo, error) {
	if err := fs.tm.Add(); err != nil {
		return []storage.FileBriefInfo{}, err
	}
	defer fs.tm.Done()

	// TODO: Call contractor.HostUtilsMap here to avoid calculating the map again and
	// 	again for each file
	var fileList []storage.FileBriefInfo
	err := filepath.Walk(string(fs.rootDir), func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != storage.DxFileExt {
			return nil
		}
		str := strings.TrimSuffix(strings.TrimPrefix(path, string(fs.rootDir)), storage.DxFileExt)
		dxPath, err := storage.NewDxPath(str)
		if err != nil {
			return err
		}
		fileInfo, err := fs.fileBriefInfo(dxPath, make(storage.HostHealthInfoTable))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		fileList = append(fileList, fileInfo)
		return nil
	})
	return fileList, err
}

// fileDetailedInfo returns detailed information for a file specified by the path
// If the input table is empty, the code the query the contractor for health info
func (fs *FileSystem) fileDetailedInfo(path storage.DxPath, table storage.HostHealthInfoTable) (storage.FileInfo, error) {
	file, err := fs.FileSet.Open(path)
	if err != nil {
		return storage.FileInfo{}, err
	}
	defer file.Close()

	var onDisk bool
	localPath := string(file.LocalPath())
	if localPath != "" {
		_, err = os.Stat(localPath)
		onDisk = err == nil
	}
	if len(table) == 0 {
		table = fs.contractor.HostHealthMapByID(file.HostIDs())
	}
	redundancy := file.Redundancy(table)
	health, stuckHealth, numStuckSegments := file.Health(table)
	info := storage.FileInfo{
		Accessible:       redundancy >= 100 || onDisk,
		FileSize:         file.FileSize(),
		Health:           health,
		StuckHealth:      stuckHealth,
		NumStuckSegments: numStuckSegments,
		Redundancy:       redundancy,
		StoredOnDisk:     onDisk,
		Recoverable:      redundancy >= 100,
		DxPath:           path.Path,
		Stuck:            numStuckSegments > 0,
		UploadProgress:   file.UploadProgress(),
	}
	return info, nil
}

// fileBriefInfo returns the brief info about a file specified by the path
// If the input table is empty, the code the query the contractor for health info
func (fs *FileSystem) fileBriefInfo(path storage.DxPath, table storage.HostHealthInfoTable) (storage.FileBriefInfo, error) {
	file, err := fs.FileSet.Open(path)
	if err != nil {
		return storage.FileBriefInfo{}, err
	}
	defer file.Close()
	if len(table) == 0 {
		table = fs.contractor.HostHealthMapByID(file.HostIDs())
	}
	recoverable := file.Redundancy(table) >= 100

	info := storage.FileBriefInfo{
		Path:           path.Path,
		UploadProgress: file.UploadProgress(),
		Recoverable:    recoverable,
	}
	return info, nil
}
