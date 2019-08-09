// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/syndtr/goleveldb/leveldb"
)

type (
	// expandFolderUpdate is the update to increase the folder size
	expandFolderUpdate struct {
		folderPath string

		prevNumSectors uint64

		targetNumSectors uint64

		folder *storageFolder

		txn *writeaheadlog.Transaction

		batch *leveldb.Batch
	}

	expandFolderUpdatePersist struct {
		FolderPath string

		PrevNumSectors uint64

		TargetNumSectors uint64
	}
)

// expandFolder expand the folder to target Num Sectors size
func (sm *storageManager) expandFolder(folderPath string, size uint64) (err error) {
	// keep the storage manager lock until finished
	sm.lock.Lock()
	defer sm.lock.Unlock()
	// Create and process the request
	update := sm.createExpandFolderUpdate(folderPath, size)
	if err = update.recordIntent(sm); err != nil {
		return
	}
	if err = sm.prepareProcessReleaseUpdate(update, targetNormal); err != nil {
		upErr := err.(*updateError)
		if !upErr.isNil() {
			sm.logError(update, upErr)
		} else {
			err = nil
		}
		return
	}
	return
}

// createExpandFolderUpdate create the expand folder update
func (sm *storageManager) createExpandFolderUpdate(folderPath string, size uint64) (update *expandFolderUpdate) {
	update = &expandFolderUpdate{
		folderPath:       folderPath,
		targetNumSectors: sizeToNumSectors(size),
	}
	return
}

func (update *expandFolderUpdate) str() (s string) {
	s = fmt.Sprintf("expand folder [%v] to %v bytes", update.folderPath, update.targetNumSectors*storage.SectorSize)
	return
}

// record intent record the expand folder intent
func (update *expandFolderUpdate) recordIntent(manager *storageManager) (err error) {
	// Open the storage folder to get the prevNumSectors
	update.folder, err = manager.folders.get(update.folderPath)
	if err != nil {
		return
	}
	update.prevNumSectors = update.folder.numSectors
	// Record the intent to the wal
	persist := expandFolderUpdatePersist{
		FolderPath:       update.folderPath,
		TargetNumSectors: update.targetNumSectors,
		PrevNumSectors:   update.prevNumSectors,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return
	}
	op := writeaheadlog.Operation{
		Name: opNameExpandFolder,
		Data: b,
	}
	if update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op}); err != nil {
		return err
	}
	return
}

// prepare is the function to be called during prepare stage
func (update *expandFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
		if manager.disruptor.disrupt("expand folder prepare normal") {
			return errDisrupted
		}
		if manager.disruptor.disrupt("expand folder prepare normal stop") {
			return errStopped
		}
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

// prepareNormal prepares for the update as normal update
func (update *expandFolderUpdate) prepareNormal(manager *storageManager) (err error) {
	// Update the memory folder
	update.folder.numSectors = update.targetNumSectors
	update.folder.usage = expandUsage(update.folder.usage, update.targetNumSectors)
	// update the db batch
	if update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, update.folder); err != nil {
		return err
	}
	// Finished initialization of the transaction
	if <-update.txn.InitComplete; update.txn.InitErr != nil {
		return update.txn.InitErr
	}
	return nil
}

func (update *expandFolderUpdate) prepareCommitted(manager *storageManager) (err error) {
	// get the folder
	if update.folder, err = manager.folders.get(update.folderPath); err != nil {
		update.folder = nil
		return err
	}
	return
}

func (update *expandFolderUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
		if manager.disruptor.disrupt("expand folder process normal") {
			return errDisrupted
		}
		if manager.disruptor.disrupt("expand folder process normal stop") {
			return errStopped
		}
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

func (update *expandFolderUpdate) processNormal(manager *storageManager) (err error) {
	// Commit the transaction
	if err = <-update.txn.Commit(); err != nil {
		return err
	}
	// truncate the related file
	if err = update.folder.dataFile.Truncate(int64(numSectorsToSize(update.targetNumSectors))); err != nil {
		return err
	}
	// apply the batch
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	return
}

func (update *expandFolderUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}

func (update *expandFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	// If no error happened, release the transaction
	if upErr == nil || upErr.isNil() {
		err = update.txn.Release()
		return
	}
	if upErr.hasErrStopped() {
		upErr.processErr = nil
		upErr.prepareErr = nil
		return
	}
	if upErr.prepareErr != nil {
		// If error happened at prepare stage, revert the memory
		update.folder.numSectors = update.prevNumSectors
		update.folder.usage = shrinkUsage(update.folder.usage, update.prevNumSectors)
		// commit and release the transaction
		if <-update.txn.InitComplete; update.txn.InitErr != nil {
			update.txn = nil
			err = update.txn.InitErr
			return
		}
		newErr := <-update.txn.Commit()
		err = common.ErrCompose(err, newErr)

		newErr = update.txn.Release()
		err = common.ErrCompose(err, newErr)
		return
	}
	// If process error
	// revert the memory
	update.folder.numSectors = update.prevNumSectors
	update.folder.usage = shrinkUsage(update.folder.usage, update.prevNumSectors)
	// revert the database
	batch := manager.db.newBatch()
	batch, newErr := manager.db.saveStorageFolderToBatch(batch, update.folder)
	err = common.ErrCompose(err, newErr)
	// apply the batch
	newErr = manager.db.writeBatch(batch)
	err = common.ErrCompose(err, newErr)
	// revert the file data
	newErr = update.folder.dataFile.Truncate(int64(numSectorsToSize(update.prevNumSectors)))
	err = common.ErrCompose(err, newErr)
	// release the transaction
	newErr = update.txn.Release()
	err = common.ErrCompose(err, newErr)
	return
}

func decodeExpandFolderUpdate(txn *writeaheadlog.Transaction) (update *expandFolderUpdate, err error) {
	var persist expandFolderUpdatePersist
	if err = rlp.DecodeBytes(txn.Operations[0].Data, &persist); err != nil {
		return nil, fmt.Errorf("cannot decode init persist: %v", err)
	}
	update = &expandFolderUpdate{
		folderPath:       persist.FolderPath,
		prevNumSectors:   persist.PrevNumSectors,
		targetNumSectors: persist.TargetNumSectors,
		txn:              txn,
	}
	return
}
