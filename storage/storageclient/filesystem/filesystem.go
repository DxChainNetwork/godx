// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"io/ioutil"
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

// ErrNoRepairNeeded is the error that no repair is needed
var ErrNoRepairNeeded = errors.New("no repair needed")

// fileSystem is the structure for a file system that include a fileSet and a dirSet
type fileSystem struct {
	// fileRootDir is the root directory where the files locates
	fileRootDir storage.SysPath

	// persistDir is the directory of containing the persist files
	persistDir storage.SysPath

	// fileSet is the fileSet from module dxfile
	fileSet *dxfile.FileSet

	// dirSet is the dirSet from module dxdir
	dirSet *dxdir.DirSet

	// contractManager is the contractManager used to give health info for the file system
	contractManager contractManager

	// fileWal is the wal responsible for storage.InsertUpdate / storage.DeleteUpdate
	// that is used in dxfile and dxdir
	fileWal *writeaheadlog.Wal

	// updateWal is the wal responsible for
	updateWal *writeaheadlog.Wal

	// tm is the thread manager for manage the threads in fileSystem
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

	// repairNeeded is the channel to signal a repair is needed
	repairNeeded chan struct{}

	// stuckFound is the channel to signal a stuck segment is found
	stuckFound chan struct{}
}

// newFileSystem creates a new file system with the standardDisrupter
func newFileSystem(persistDir string, contractor contractManager, disrupter disrupter) *fileSystem {
	// create the fileSystem
	return &fileSystem{
		fileRootDir:       storage.SysPath(filepath.Join(persistDir, filesDirectory)),
		persistDir:        storage.SysPath(persistDir),
		contractManager:   contractor,
		tm:                &threadmanager.ThreadManager{},
		logger:            log.New("module", "filesystem"),
		disrupter:         disrupter,
		unfinishedUpdates: make(map[storage.DxPath]*dirMetadataUpdate),
		repairNeeded:      make(chan struct{}, 1),
		stuckFound:        make(chan struct{}, 1),
	}
}

// Start is the function that is called for starting the file system service.
// It open the wals, apply all transactions, and start the thread loopRepairUnfinishedDirMetadataUpdate
func (fs *fileSystem) Start() error {
	// open the fileWal
	if err := fs.loadFileWal(); err != nil {
		return fmt.Errorf("cannot start the file system: %v", err)
	}
	// load fs.dirSet
	var err error
	if fs.dirSet, err = dxdir.NewDirSet(fs.fileRootDir, fs.fileWal); err != nil {
		return fmt.Errorf("cannot start the file system dirSet: %v", err)
	}
	fs.fileSet = dxfile.NewFileSet(fs.fileRootDir, fs.fileWal)
	// open the updateWal
	if err := fs.loadUpdateWal(); err != nil {
		return fmt.Errorf("cannot start the file system: %v", err)
	}
	// Start the repair loop
	go fs.loopRepairUnfinishedDirMetadataUpdate()
	return nil
}

// Close will terminate all threads opened by file system
func (fs *fileSystem) Close() error {
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

// RootDir returns the root directory for the files
func (fs *fileSystem) RootDir() storage.SysPath {
	return fs.fileRootDir
}

// PersistDir returns the persist path of the file system
func (fs *fileSystem) PersistDir() storage.SysPath {
	return fs.persistDir
}

// NewDxFile creates a new dxfile in the file system
func (fs *fileSystem) NewDxFile(dxPath storage.DxPath, sourcePath storage.SysPath, force bool, erasureCode erasurecode.ErasureCoder, cipherKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*dxfile.FileSetEntryWithID, error) {
	return fs.fileSet.NewDxFile(dxPath, sourcePath, force, erasureCode, cipherKey, fileSize, fileMode)
}

// OpenDxFile opens the DxFile specified by the path
func (fs *fileSystem) OpenDxFile(path storage.DxPath) (*dxfile.FileSetEntryWithID, error) {
	return fs.fileSet.Open(path)
}

// Delete delete the dxfile from the file system
func (fs *fileSystem) DeleteDxFile(dxPath storage.DxPath) error {
	return fs.fileSet.Delete(dxPath)
}

// RenameDxFile rename the dxfile from prevPath to newPath
func (fs *fileSystem) RenameDxFile(prevPath, newPath storage.DxPath) error {
	return fs.fileSet.Rename(prevPath, newPath)
}

// NewDxDir creates a new dxdir specified by path
func (fs *fileSystem) NewDxDir(path storage.DxPath) (*dxdir.DirSetEntryWithID, error) {
	return fs.dirSet.NewDxDir(path)
}

// OpenDxDir opens the dxdir in the file system
func (fs *fileSystem) OpenDxDir(path storage.DxPath) (*dxdir.DirSetEntryWithID, error) {
	return fs.dirSet.Open(path)
}

// SelectDxFileToFix selects a file with the health of highest priority to repair
func (fs *fileSystem) SelectDxFileToFix() (*dxfile.FileSetEntryWithID, error) {
	curDir, err := fs.dirSet.Open(storage.RootDxPath())
	if err != nil {
		return nil, err
	}
	defer func() {
		curDir.Close()
	}()
LOOP:
	for {
		select {
		case <-fs.tm.StopChan():
			return nil, errStopped
		default:
		}
		health := curDir.Metadata().Health
		if err = curDir.Close(); err != nil {
			return nil, err
		}
		// If the health is larger than the threshold, no repair is needed
		if dxfile.CmpRepairPriority(health, dxfile.RepairHealthThreshold) <= 0 {
			return nil, ErrNoRepairNeeded
		}
		// Get dirs and files o the directory
		dirs, files, err := fs.dirsAndFiles(curDir.DxPath())
		if err != nil {
			return nil, err
		}
		// Loop over files and compare the health
		for file := range files {
			select {
			case <-fs.tm.StopChan():
				return nil, errStopped
			default:
			}
			df, err := fs.OpenDxFile(file)
			if err != nil {
				fs.logger.Warn("file system open file", "path", file, "err", err)
				continue
			}
			fHealth := df.GetHealth()
			if dxfile.CmpRepairPriority(fHealth, health) >= 0 {
				// This is the file we want to repair
				return df, nil
			}
			df.Close()
		}
		// Loop over dirs and compare with the health
		for dir := range dirs {
			select {
			case <-fs.tm.StopChan():
				return nil, errStopped
			default:
			}
			d, err := fs.dirSet.Open(dir)
			if err != nil {
				fs.logger.Warn("file system open curDir", "path", dir, "err", err)
				continue
			}
			dHealth := d.Metadata().Health
			if dxfile.CmpRepairPriority(dHealth, health) >= 0 {
				if err = curDir.Close(); err != nil {
					return nil, common.ErrCompose(err, d.Close())
				}
				curDir = d
				goto LOOP
			}
		}
		// Loops over. No file founded in the directory
		return nil, ErrNoRepairNeeded
	}
}

// RandomStuckDirectory randomly pick a stuck directory to fix. The possibility to pick
// is proportion to the value of numStuckSegments
func (fs *fileSystem) RandomStuckDirectory() (*dxdir.DirSetEntryWithID, error) {
	path := storage.RootDxPath()
	curDir, err := fs.dirSet.Open(path)
	if err != nil {
		return nil, err
	}
	// create the random index
	numStuckSegments := curDir.Metadata().NumStuckSegments
	if numStuckSegments == 0 {
		return nil, ErrNoRepairNeeded
	}
	index := randomUint32() % numStuckSegments
	// permanent loop to find the directory
	for {
	LOOP:
		select {
		case <-fs.tm.StopChan():
			return nil, errStopped
		default:
		}
		dirs, _, err := fs.dirsAndFiles(curDir.DxPath())
		if err != nil {
			return nil, err
		}
		for dirPath := range dirs {
			d, err := fs.dirSet.Open(dirPath)
			if err != nil {
				continue
			}
			dNumStuckSegments := d.Metadata().NumStuckSegments
			if index < dNumStuckSegments {
				// This is the directory to go deep into
				curDir.Close()
				curDir = d
				goto LOOP
			} else {
				index -= dNumStuckSegments
				d.Close()
			}
		}
		// All curDir passed, still not found the directory, return the current directory
		return curDir, nil
	}
}

// OldestLastTimeHealthCheck find the dxpath of the directory with the oldest lastTimeHealthCheck
func (fs *fileSystem) OldestLastTimeHealthCheck() (storage.DxPath, time.Time, error) {
	path := storage.RootDxPath()
	dir, err := fs.dirSet.Open(path)
	if err != nil {
		return storage.DxPath{}, time.Time{}, err
	}
	md := dir.Metadata()
	if err = dir.Close(); err != nil {
		return storage.DxPath{}, time.Time{}, err
	}

	for time.Since(time.Unix(int64(md.TimeLastHealthCheck), 0)) > healthCheckInterval {
		// check whether the file system has closed
		select {
		case <-fs.tm.StopChan():
			return storage.DxPath{}, time.Time{}, errStopped
		default:
		}
		subDirs, _, err := fs.dirsAndFiles(path)
		if err != nil {
			return storage.DxPath{}, time.Time{}, err
		}
		// If no more directories to go deep into, return
		if len(subDirs) == 0 {
			return path, time.Unix(int64(md.TimeLastHealthCheck), 0), nil
		}
		// Loop through the subDirs
		updated := false
		for subDir := range subDirs {
			dir, err := fs.dirSet.Open(subDir)
			if err != nil {
				return storage.DxPath{}, time.Time{}, err
			}
			subMd := dir.Metadata()
			if err = dir.Close(); err != nil {
				return storage.DxPath{}, time.Time{}, err
			}
			// If subdirectory has older timestamp than the parent directory, continue to next
			// subdir
			if subMd.TimeLastHealthCheck > md.TimeLastHealthCheck {
				continue
			}
			updated = true
			md = subMd
			path = subDir
		}
		// After loop over dirs, not updated, return current directory
		if !updated {
			return path, time.Unix(int64(md.TimeLastHealthCheck), 0), nil
		}
	}
	return path, time.Unix(int64(md.TimeLastHealthCheck), 0), nil
}

// RepairNeededChan return a channel that signals a repair is needed
func (fs *fileSystem) RepairNeededChan() chan struct{} {
	return fs.repairNeeded
}

// StuckFoundChan returns a channel that signals a stuck segment is found
func (fs *fileSystem) StuckFoundChan() chan struct{} {
	return fs.stuckFound
}

// dirsAndFiles return the dxdirs and dxfiles under the path. return DxPath for DxDir and DxFiles, and errors
// The returned type map is to add the randomness in file selection
func (fs *fileSystem) dirsAndFiles(path storage.DxPath) (map[storage.DxPath]struct{}, map[storage.DxPath]struct{}, error) {
	fileInfos, err := ioutil.ReadDir(string(fs.fileRootDir.Join(path)))
	if err != nil {
		return nil, nil, err
	}
	dirs, files := make(map[storage.DxPath]struct{}), make(map[storage.DxPath]struct{})
	// iterate over all files
	for _, file := range fileInfos {
		select {
		case <-fs.tm.StopChan():
			return nil, nil, errStopped
		default:
		}
		ext := filepath.Ext(file.Name())
		if ext == storage.DxFileExt {
			filenameNoSuffix := strings.TrimSuffix(file.Name(), storage.DxFileExt)
			fileDxPath, err := path.Join(filenameNoSuffix)
			if err != nil {
				fs.logger.Warn("invalid DxPath name", "name", filenameNoSuffix)
				continue
			}
			files[fileDxPath] = struct{}{}
		} else if file.IsDir() {
			dirDxPath, err := path.Join(file.Name())
			if err != nil {
				fs.logger.Warn("invalid DxPath name", "name", file.Name())
				continue
			}
			dirs[dirDxPath] = struct{}{}
		} else {
			// Unrecognized file type
			continue
		}
	}
	return dirs, files, nil
}

// loadFileWal read the fileWal
func (fs *fileSystem) loadFileWal() error {
	fileWalPath := filepath.Join(string(fs.persistDir), fileWalName)
	fileWal, unappliedTxns, err := writeaheadlog.New(fileWalPath)
	if err != nil {
		return fmt.Errorf("cannot start load system fileWal: %v", err)
	}
	for i, txn := range unappliedTxns {
		err = storage.ApplyOperations(txn.Operations)
		if err != nil {
			fs.logger.Warn("cannot apply the operation of file transaction", "index", i, "error", err)
		}
		err = txn.Release()
		if err != nil {
			fs.logger.Warn("cannot release the operation of file transaction", "index", i, "error", err)
		}
	}
	fs.fileWal = fileWal
	return nil
}

// loadUpdateWal load the update Wal from disk, and apply unfinished updates
func (fs *fileSystem) loadUpdateWal() error {
	updateWalPath := filepath.Join(string(fs.persistDir), updateWalName)
	updateWal, unappliedTxns, err := writeaheadlog.New(updateWalPath)
	if err != nil {
		return fmt.Errorf("cannot start file system updateWal: %v", err)
	}
	for i, txn := range unappliedTxns {
		for j, op := range txn.Operations {
			path, err := decodeWalOp(op)
			if err != nil {
				fs.logger.Warn(fmt.Sprintf("cannot decode txn[%d].operation[%d]", i, j), "error", err)
				continue
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

// loopRepairUnfinishedDirMetadataUpdate is the permanent loop for repairing the unfinished
// dirMetadataUpdate.
func (fs *fileSystem) loopRepairUnfinishedDirMetadataUpdate() {
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
func (fs *fileSystem) repairUnfinishedDirMetadataUpdate() error {
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
func (fs *fileSystem) disrupt(s string) bool {
	return fs.disrupter.disrupt(s)
}

// fileList returns a brief file info list
func (fs *fileSystem) fileList() ([]storage.FileBriefInfo, error) {
	if err := fs.tm.Add(); err != nil {
		return []storage.FileBriefInfo{}, err
	}
	defer fs.tm.Done()

	var fileList []storage.FileBriefInfo
	healthInfoTable := fs.contractManager.HostHealthMap()
	err := filepath.Walk(string(fs.fileRootDir), func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != storage.DxFileExt {
			return nil
		}
		str := strings.TrimSuffix(strings.TrimPrefix(path, string(fs.fileRootDir)), storage.DxFileExt)
		dxPath, err := storage.NewDxPath(str)
		if err != nil {
			return err
		}
		fileInfo, err := fs.fileBriefInfo(dxPath, healthInfoTable)
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
// If the input table is empty, the code the query the contractManager for health info
func (fs *fileSystem) fileDetailedInfo(path storage.DxPath, table storage.HostHealthInfoTable) (storage.FileInfo, error) {
	file, err := fs.fileSet.Open(path)
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
		table = fs.contractManager.HostHealthMapByID(file.HostIDs())
	}
	status := fileStatus(file, table)
	redundancy := file.Redundancy(table)

	info := storage.FileInfo{
		DxPath:         path.Path,
		Status:         status,
		SourcePath:     string(file.LocalPath()),
		FileSize:       file.FileSize(),
		Redundancy:     redundancy,
		StoredOnDisk:   onDisk,
		UploadProgress: file.UploadProgress(),
	}
	return info, nil
}

// fileBriefInfo returns the brief info about a file specified by the path
// If the input table is empty, the code the query the contractManager for health info
func (fs *fileSystem) fileBriefInfo(path storage.DxPath, table storage.HostHealthInfoTable) (storage.FileBriefInfo, error) {
	file, err := fs.fileSet.Open(path)
	if err != nil {
		return storage.FileBriefInfo{}, err
	}
	defer file.Close()
	if len(table) == 0 {
		table = fs.contractManager.HostHealthMapByID(file.HostIDs())
	}

	info := storage.FileBriefInfo{
		Path:           path.Path,
		UploadProgress: file.UploadProgress(),
		Status:         fileStatus(file, table),
	}
	return info, nil
}

// getLogger returns the logger of the file system
func (fs *fileSystem) getLogger() log.Logger {
	return fs.logger
}

// fileStatus return the human readable status
func fileStatus(file *dxfile.FileSetEntryWithID, table storage.HostHealthInfoTable) string {
	health, _, numStuckSegments := file.Health(table)
	if numStuckSegments > 0 {
		return statusUnrecoverableStr
	}
	return humanReadableHealth(health)
}

// humanReadableHealth convert the health to human readable string
func humanReadableHealth(health uint32) string {
	if health > healthyThreshold {
		return statusHealthyStr
	}
	if health > recoverableThreshold {
		return statusRecoverableStr
	}
	if health > inDangerThreshold {
		return statusInDangerStr
	}
	return statusUnrecoverableStr
}

// randomUint32 create a random number uint32
func randomUint32() uint32 {
	b := make([]byte, 4)
	rand.Read(b)
	return binary.LittleEndian.Uint32(b)
}
