// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

var (
	// errUpdateAlreadyInProgress is the error when creating a new update, found
	// there is already an update goroutine in progress.
	errUpdateAlreadyInProgress = errors.New("another goroutine is working on metadata update")

	// errStopped is the error that happens when the system stops during the update
	errStopped = errors.New("file system stopped during update")

	// errInterrupted is the error that happens during an update, another update interrupt
	// and redo the update
	errInterrupted = errors.New("file update is interrupted")
)

type (
	// dirMetadataUpdate is a single dirMetadataUpdate for updating a dxdir metadata
	dirMetadataUpdate struct {
		// dxPath is the path relates to the root path of the directory currently being repaired
		dxPath storage.DxPath

		// updateInProgress is the atomic field of whether the current threadedUpdate is on going.
		// There should be at most one threadedUpdate on going
		updateInProgress uint32

		// stop is a channel indicating whether a stop request is received for the update.
		// The source of stop comes from two conditions:
		//  1. the program is shutting down
		//  2. A new thread is trying to update the current DxPath
		stop chan struct{}

		// consecutiveFails is the atomic field for counting the consecutive failed times.
		// When consecutiveFails reaches a certain number, the dirMetadata update is released.
		consecutiveFails uint32

		// walTxn is the wal transaction associated with dirMetadataUpdate.
		// It is stored when first time the goroutine is established.
		walTxn unsafe.Pointer
	}

	// metadataForUpdate contains the least information for dxdir update
	metadataForUpdate struct {
		numFiles            uint64
		totalSize           uint64
		health              uint32
		stuckHealth         uint32
		minRedundancy       uint32
		numStuckSegments    uint32
		timeLastHealthCheck time.Time
	}
)

// InitAndUpdateDirMetadata create the update intent, and then apply the intent.
// The actual metadata update is executed in a thread updateDirMetadata
func (fs *fileSystem) InitAndUpdateDirMetadata(path storage.DxPath) error {
	// Initialize the dirMetadataUpdate, that is, recordDirMetadataUpdate
	txn, err := fs.recordDirMetadataIntent(path)
	if err != nil {
		return fmt.Errorf("cannot update metadata at %v", path.Path)
	}
	// Apply the update
	if err = fs.updateDirMetadata(path, txn); err != nil {
		// tm already closed or update thread already in progress
		if err == errUpdateAlreadyInProgress {
			err = txn.Release()
		}
		return err
	}
	return nil
}

// recordDirMetadataIntent record the dirMetadata intent to the wal
func (fs *fileSystem) recordDirMetadataIntent(path storage.DxPath) (*writeaheadlog.Transaction, error) {
	op, err := createWalOp(path)
	if err != nil {
		return nil, err
	}
	txn, err := fs.updateWal.NewTransaction([]writeaheadlog.Operation{op})
	if err != nil {
		return nil, err
	}

	if <-txn.InitComplete; txn.InitErr != nil {
		return nil, txn.InitErr
	}
	if err = <-txn.Commit(); err != nil {
		return nil, err
	}
	return txn, nil
}

// createWalOp creates a dir metadata update based on the give path
func createWalOp(path storage.DxPath) (writeaheadlog.Operation, error) {
	b, err := rlp.EncodeToBytes(path)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: dirMetadataUpdateName,
		Data: b,
	}, nil
}

// decodeWalOp decode the wal.Operation to DxPath
func decodeWalOp(operation writeaheadlog.Operation) (storage.DxPath, error) {
	if operation.Name != dirMetadataUpdateName {
		return storage.DxPath{}, fmt.Errorf("unknown operation name for updateWal [%s]", operation.Name)
	}
	var s string
	if err := rlp.DecodeBytes(operation.Data, &s); err != nil {
		return storage.DxPath{}, err
	}
	return storage.NewDxPath(s)
}

// applyDirMetadataUpdate creates a new dirMetadataUpdate and initialize a
// goroutine of calculateMetadataAndApply as necessary.
//  1. If the update already exists in fs.unfinishedUpdates, signal the current update thread
//     signalStop
//  2. If there is already an update thread in progress, return
//  3. Else add to fs.tm, update fs.unfinishedUpdates, and start a goroutine to
//     calculateMetadataAndApply
// The returned error type is could be two cases:
//  1. threadmanager already stopped. It would be safe to throw the error during error handling
//  2. there is already a thread updating the metadata. return errUpdateAlreadyInProgress
// The function is called in two places:
//  1. A dxfile is updated and the update is bubble to the root
//  2. A MaintenanceLoop loops over the fs.unfinishedUpdates field for previously failed update.
//     In this case, walTxn should be nil
func (fs *fileSystem) updateDirMetadata(path storage.DxPath, walTxn *writeaheadlog.Transaction) (err error) {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	// If the update already exists in the unfinishedUpdates, signalStop
	update, exist := fs.unfinishedUpdates[path]
	if exist {
		update.signalStop()
	} else {
		update = &dirMetadataUpdate{
			dxPath:           path,
			updateInProgress: 0,
			stop:             make(chan struct{}, 1),
		}
	}

	// Check whether the update has an update thread in progress.
	// Create an update thread only if the updateInProgress is false
	swapped := atomic.CompareAndSwapUint32(&update.updateInProgress, 0, 1)
	if !swapped {
		// Already an update thread in progress, return
		err = errUpdateAlreadyInProgress
		return
	}
	defer func() {
		// If error happened swap back the updateInProgress value
		if err != nil {
			atomic.StoreUint32(&update.updateInProgress, 0)
		}
	}()
	if err = fs.tm.Add(); err != nil {
		err = errStopped
		return
	}
	fs.unfinishedUpdates[path] = update
	// If the input walTxn is not nil, update the walTxn field
	if walTxn != nil {
		atomic.StorePointer(&update.walTxn, unsafe.Pointer(walTxn))
	}
	go fs.calculateMetadataAndApply(update)
	return
}

// signalStop is a helper function that update the redo field and try to fill the stop channel.
// It is triggered when a new update is created while currently a dirMetadataUpdate is in progress.
func (update *dirMetadataUpdate) signalStop() {
	// Try to insert into the channel. If cannot, there is already struct in stop channel.
	select {
	case update.stop <- struct{}{}:
	default:
	}
}

// calculateMetadataAndApply is the threaded function of calculate the metadata
// and save to dxdir. This is the core function of this file
func (fs *fileSystem) calculateMetadataAndApply(update *dirMetadataUpdate) {
	var err error
	// Call cleanUp to clean up the update
	defer func() {
		update.cleanUp(fs, err)
	}()
	// clear the stop channel
	select {
	case <-update.stop:
	default:
	}

	for {
		// store the value of redo as not needed
		if fs.disrupt("cmaa1") {
			err = errDisrupted
			return
		}
		select {
		case <-update.stop:
			continue
		case <-fs.tm.StopChan():
			err = errStopped
			return
		default:
		}
		if fs.disrupt("cmaa2") {
			err = errDisrupted
			return
		}
		md, err := fs.loopDirAndCalculateDirMetadata(update)
		if err == errInterrupted {
			continue
		}
		if err != nil {
			return
		}
		err = fs.applyDxDirMetadata(update.dxPath, md)
		if err != nil {
			return
		}
		if fs.disrupt("cmaa3") {
			err = errDisrupted
			return
		}
		// Termination. Only happens when redo value is redoNotNeeded
		select {
		case <-update.stop:
			continue
		case <-fs.tm.StopChan():
			err = errStopped
			return
		default:
		}
		break
	}
	return
}

// cleanUp is the defer function that is called for the goroutine of calculateMetadataAndApply
// It do the following:
// 1. Set the updateInProgress value to 0, notifying this goroutine is over
// 2. Notify thread manager this goroutine is done.
// 3. Determine whether release is necessary
// 4. If release is needed, delete the entry in fs.unfinishedUpdates.
// 5. If release is needed, Release the transaction
func (update *dirMetadataUpdate) cleanUp(fs *fileSystem, err error) {
	// fs.tm.Done()
	defer fs.tm.Done()

	// If the error is errStopped, do nothing and simply return
	if err == errStopped {
		return
	}
	// Set the updateInProgress value to 0, notifying this goroutine is over
	atomic.StoreUint32(&update.updateInProgress, 0)

	// determine whether release is necessary
	// release could happen either non err or consecutiveFails reaches the limit
	if err != nil {
		atomic.AddUint32(&update.consecutiveFails, 1)
	}
	fails := atomic.LoadUint32(&update.consecutiveFails)

	release := fails >= numConsecutiveFailRelease || err == nil

	// 3. If no error happened, delete the entry in fs.unfinishedUpdates.
	// 4. If no error happened, Release the transaction
	if release {
		txn := (*writeaheadlog.Transaction)(atomic.LoadPointer(&update.walTxn))
		if txn != nil {
			err = common.ErrCompose(err, txn.Release())
		}

		defer func() {
			fs.lock.Lock()
			delete(fs.unfinishedUpdates, update.dxPath)
			fs.lock.Unlock()
		}()
	}

	if err == nil {
		// no error happend. Continue to update parent
		if update.dxPath.IsRoot() {
			// If root check for repairNeeded and stuckFound, and there is no need to further update parent
			d, err := fs.dirSet.Open(update.dxPath)
			if err != nil {
				fs.logger.Warn("cannot open root directory")
				return
			}
			md := d.Metadata()
			if md.Health < dxfile.RepairHealthThreshold {
				select {
				case fs.repairNeeded <- struct{}{}:
				default:
				}
			}
			if md.NumStuckSegments > 0 {
				select {
				case fs.stuckFound <- struct{}{}:
				default:
				}
			}
			if err := d.Close(); err != nil {
				fs.logger.Warn("cannot close root directory", "err", err)
			}
			return
		}
		parent, err := update.dxPath.Parent()
		if err != nil {
			fs.logger.Warn("cannot create parent directory", "path", update.dxPath.Path, "err", err)
			return
		}
		if err = fs.InitAndUpdateDirMetadata(parent); err != nil {
			fs.logger.Warn("cannot update parent directory", "path", update.dxPath.Path, "err", err)
		}
	} else {
		if release {
			// released updates failed more than numConsecutiveFailRelease times
			fs.logger.Error("cannot update the metadata.", "consecutive fails", fails, "err", err)
		} else {
			// unreleased updates
			fs.logger.Warn("cannot update the metadata. Try later", "err", err)
		}
	}
}

// loopDirAndCalculateDirMetadata loops over all files under the DxPath and calculate the updated
// metadata of the update
func (fs *fileSystem) loopDirAndCalculateDirMetadata(update *dirMetadataUpdate) (*dxdir.Metadata, error) {
	// Set default metadata value
	metadata := &dxdir.Metadata{
		NumFiles:            0,
		TotalSize:           0,
		Health:              dxdir.DefaultHealth,
		StuckHealth:         dxdir.DefaultHealth,
		MinRedundancy:       math.MaxUint32,
		TimeLastHealthCheck: uint64(time.Now().Unix()),
		TimeModify:          uint64(time.Now().Unix()),
		NumStuckSegments:    0,
		DxPath:              update.dxPath,
		RootPath:            fs.fileRootDir,
	}
	// Read all files and directories under the path
	fileInfos, err := ioutil.ReadDir(string(fs.fileRootDir.Join(update.dxPath)))
	if err != nil {
		return nil, err
	}
	// Iterate over all files under the directory
	for _, file := range fileInfos {
		// If there is a stop signal, return the error of errStopped
		select {
		case <-update.stop:
			return nil, errInterrupted
		case <-fs.tm.StopChan():
			return nil, errStopped
		default:
		}
		ext := filepath.Ext(file.Name())
		var md *metadataForUpdate
		if ext == storage.DxFileExt {
			// File type DxFile
			md, err = fs.calculateDxFileMetadata(update.dxPath, file.Name())
			if err != nil {
				fs.logger.Warn("cannot calculate the file metadata", "path", update.dxPath.Path, "err", err)
				continue
			}
		} else if file.IsDir() {
			// File type DxDir
			md, err = fs.calculateDxDirMetadata(update.dxPath, file.Name())
			if err == os.ErrExist {
				continue
			}
			if err != nil {
				fs.logger.Warn("cannot calculate the file metadata", "path", update.dxPath.Path, "err", err)
				continue
			}
		} else {
			// Ignore all files other than DxFile and DxDir
			continue
		}
		metadata = applyMetadataForUpdateToMetadata(metadata, md)
	}
	return metadata, nil
}

// calculateDxFileMetadata update, calculate and apply the health related field of a dxfile.
func (fs *fileSystem) calculateDxFileMetadata(path storage.DxPath, filename string) (*metadataForUpdate, error) {
	// Deal with the file names. Input path is the DxPath of the target directory.
	// filename is the system filename of the dxfile.
	filenameNoSuffix := strings.TrimSuffix(filename, storage.DxFileExt)
	fileDxPath, err := path.Join(filenameNoSuffix)
	if err != nil {
		return nil, err
	}
	// Open the DxPath
	file, err := fs.fileSet.Open(fileDxPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open DxPath %v: %v", fileDxPath.Path, err)
	}
	defer file.Close()

	// Get the healthInfoMap, mark all healthy as unstuck, and then calculate the health
	healthInfoTable := fs.contractManager.HostHealthMapByID(file.HostIDs())
	if err = file.MarkAllUnhealthySegmentsAsStuck(healthInfoTable); err != nil {
		return nil, fmt.Errorf("cannot mark stuck segments for file %v: %v", fileDxPath.Path, err)
	}
	if err = file.MarkAllHealthySegmentsAsUnstuck(healthInfoTable); err != nil {
		return nil, fmt.Errorf("cannot mark unstuck segments for file %v: %v", fileDxPath.Path, err)
	}
	health, stuckHealth, numStuckSegments := file.Health(healthInfoTable)
	redundancy := file.Redundancy(healthInfoTable)

	// Update TimeLastHealthCheck
	if err := file.SetTimeLastHealthCheck(time.Now()); err != nil {
		return nil, fmt.Errorf("cannot SetTimeLastHealthCheck for file %v: %v", fileDxPath.Path, err)
	}
	cachedMetadata := dxfile.CachedHealthMetadata{
		Health:      health,
		StuckHealth: stuckHealth,
		Redundancy:  redundancy,
	}
	// apply cached metadata and return
	return &metadataForUpdate{
		numFiles:            1,
		totalSize:           file.FileSize(),
		health:              health,
		stuckHealth:         stuckHealth,
		minRedundancy:       redundancy,
		numStuckSegments:    numStuckSegments,
		timeLastHealthCheck: time.Now(),
	}, file.ApplyCachedHealthMetadata(cachedMetadata)
}

// calculateDxDirMetadata calculate and return the metadata from the .dxdir file
func (fs *fileSystem) calculateDxDirMetadata(path storage.DxPath, filename string) (*metadataForUpdate, error) {
	path, err := path.Join(filename)
	if err != nil {
		return nil, err
	}
	d, err := fs.dirSet.Open(path)
	if os.IsNotExist(err) {
		// The .dxdir not exist. Create a new one
		d, err = fs.dirSet.NewDxDir(path)
		if os.IsExist(err) {
			return nil, os.ErrExist
		}
		if err != nil {
			return nil, fmt.Errorf("cannot create the .dxdir file for file %v: %v", path.Path, err)
		}
	} else if err != nil {
		return nil, err
	}
	defer d.Close()
	// No error, or the dxdir is created.
	rawMetadata := d.Metadata()
	return &metadataForUpdate{
		numFiles:            rawMetadata.NumFiles,
		totalSize:           rawMetadata.TotalSize,
		health:              rawMetadata.Health,
		stuckHealth:         rawMetadata.StuckHealth,
		minRedundancy:       rawMetadata.MinRedundancy,
		numStuckSegments:    rawMetadata.NumStuckSegments,
		timeLastHealthCheck: time.Unix(int64(d.Metadata().TimeLastHealthCheck), 0),
	}, nil
}

// applyDxDirMetadata apply the calculated metadata to the dxdir path
func (fs *fileSystem) applyDxDirMetadata(path storage.DxPath, md *dxdir.Metadata) error {
	var d *dxdir.DirSetEntryWithID
	var err error
	d, err = fs.dirSet.NewDxDir(path)
	if err == os.ErrExist {
		d, err = fs.dirSet.Open(path)
		if err != nil {
			return err
		}

	} else if err != nil {
		return err
	}
	defer d.Close()
	err = d.UpdateMetadata(*md)
	if err != nil {
		return err
	}
	return nil
}

// applyMetadataForUpdateToMetadata apply a metadataForUpdate to dxdir.Metadata
func applyMetadataForUpdateToMetadata(md *dxdir.Metadata, update *metadataForUpdate) *dxdir.Metadata {
	md.NumFiles += update.numFiles
	md.TotalSize += update.totalSize
	// If the update health or stuckHealth has higher priority than md.Health, update
	if dxfile.CmpRepairPriority(update.health, md.Health) > 0 {
		md.Health = update.health
	}
	if dxfile.CmpRepairPriority(update.stuckHealth, md.StuckHealth) > 0 {
		md.StuckHealth = update.stuckHealth
	}
	// Update minRedundancy
	if update.minRedundancy < md.MinRedundancy {
		md.MinRedundancy = update.minRedundancy
	}
	md.NumStuckSegments += update.numStuckSegments
	// update timeLastHealthCheck. TimeLastHealthCheck is the oldest time for health check
	if uint64(update.timeLastHealthCheck.Unix()) < md.TimeLastHealthCheck {
		md.TimeLastHealthCheck = uint64(update.timeLastHealthCheck.Unix())
	}
	return md
}
