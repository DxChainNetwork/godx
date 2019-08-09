// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"

	"github.com/syndtr/goleveldb/leveldb"
)

type (
	// deleteSectorBatchUpdate defines an update to delete batches
	deleteSectorBatchUpdate struct {
		// ids is the function call params
		ids []sectorID

		// sectors is the updated sector fields
		sectors []*sector

		// folders are the mapping from folder id to storage folder
		folders map[folderID]*storageFolder

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	// deleteBatchInitPersist is the persist to recorded in record intent
	deleteBatchInitPersist struct {
		IDs []sectorID
	}

	// deleteVritualBatchPersist is the persist for appended operations for deleting a virtual sector
	deleteVirtualSectorPersist struct {
		ID       sectorID
		FolderID folderID
		Index    uint64
		Count    uint64
	}

	// deletePhysicalSectorPersist is the persist for appended operations for deleting
	// a physical sector
	deletePhysicalSectorPersist struct {
		ID       sectorID
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// DeleteSector delete a sector from storage manager. It calls DeleteSectorBatch
func (sm *storageManager) DeleteSector(root common.Hash) (err error) {
	return sm.DeleteSectorBatch([]common.Hash{root})
}

// DeleteSectorBatch delete sectors in batch
func (sm *storageManager) DeleteSectorBatch(roots []common.Hash) (err error) {
	// Check the size of the roots. If 0, return immediately
	if len(roots) == 0 {
		return
	}
	// Lock the storage manager
	sm.lock.Lock()
	defer sm.lock.Unlock()
	// create the update and record the intent
	update := sm.createDeleteSectorBatchUpdate(roots)
	if err = update.recordIntent(sm); err != nil {
		return err
	}
	// prepare process, and release the update
	if err = sm.prepareProcessReleaseUpdate(update, targetNormal); err != nil {
		if upErr := err.(*updateError); !upErr.isNil() {
			sm.logError(update, upErr)
		} else {
			err = nil
		}
		return
	}
	return
}

// createDeleteSectorBatchUpdate create the deleteSectorBatchUpdate
func (sm *storageManager) createDeleteSectorBatchUpdate(roots []common.Hash) (update *deleteSectorBatchUpdate) {
	// copy the ids
	update = &deleteSectorBatchUpdate{
		ids: make([]sectorID, 0, len(roots)),
	}
	for _, root := range roots {
		id := sm.calculateSectorID(root)
		update.ids = append(update.ids, id)
	}
	return
}

// str defines the string representation of the update
func (update *deleteSectorBatchUpdate) str() (s string) {
	s = "Delete sector batch\n[\n\t"
	idStrs := make([]string, 0, len(update.ids))
	for _, id := range update.ids {
		idStrs = append(idStrs, common.Bytes2Hex(id[:]))
	}
	s += strings.Join(idStrs, ",\n\t")
	s += "\n]"
	return
}

// recordIntent record the intent of deleteSectorBatchUpdate
func (update *deleteSectorBatchUpdate) recordIntent(manager *storageManager) (err error) {
	persist := deleteBatchInitPersist{
		IDs: update.ids,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return
	}
	op := writeaheadlog.Operation{
		Name: opNameDeleteSectorBatch,
		Data: b,
	}
	update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op})
	if err != nil {
		update.txn = nil
		return fmt.Errorf("cannot create transaction: %v", err)
	}
	// load and lock sectors and folders.
	if err = update.loadSectorsAndFolders(manager); err != nil {
		return fmt.Errorf("cannot load sectors and folders: %v", err)
	}
	return
}

// prepare prepares for the deleteSectorBatchUpdate at specified target
func (update *deleteSectorBatchUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
		if manager.disruptor.disrupt("delete batch prepare normal") {
			return errDisrupted
		}
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	if manager.disruptor.disrupt("delete batch prepare stop") {
		return errStopped
	}
	return
}

// process process for the deleteSectorBatchUpdate at specified target
func (update *deleteSectorBatchUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
		if manager.disruptor.disrupt("delete batch process normal") {
			return errDisrupted
		}
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	if manager.disruptor.disrupt("delete batch process stop") {
		return errStopped
	}
	return
}

// prepareNormal prepares for the normal update
func (update *deleteSectorBatchUpdate) prepareNormal(manager *storageManager) (err error) {
	var once sync.Once
	// update entries and write to database and wal
	for _, s := range update.sectors {
		var op writeaheadlog.Operation
		if s.count <= 1 {
			// The sector need to be deleted physically
			op, err = update.preparePhysicalSector(manager, s)
		} else {
			// The sector is deleted virtually
			op, err = update.prepareVirtualSector(manager, s)
		}
		// Wait for init to complete just once
		once.Do(func() {
			if <-update.txn.InitComplete; update.txn.InitErr != nil {
				update.txn = nil
				err = update.txn.InitErr
			}
		})
		if err != nil {
			return fmt.Errorf("wal init error: %v", err)
		}
		if err = <-update.txn.Append([]writeaheadlog.Operation{op}); err != nil {
			return err
		}
	}
	return
}

// loadSectorsAndFolders load the sectors and folders from database and memory.
// Also the locks for the sectors and folders are already locked
func (update *deleteSectorBatchUpdate) loadSectorsAndFolders(manager *storageManager) (err error) {
	// lock all sectors
	folderPaths := make([]string, 0)
	// Get all sectors and get related folder paths
	for _, id := range update.ids {
		s, err := manager.db.getSector(id)
		if err != nil {
			return err
		}
		if s.count > 1 {
			update.sectors = append(update.sectors, s)
		} else {
			// Need to delete the sector. The folder is effected
			update.sectors = append(update.sectors, s)
			if _, exist := update.folders[s.folderID]; !exist {
				path, err := manager.db.getFolderPath(s.folderID)
				if err != nil {
					return err
				}
				folderPaths = append(folderPaths, path)
			}
		}
	}
	update.folders, err = manager.folders.getFolders(folderPaths)
	if err != nil {
		return err
	}
	return
}

// preparePhysicalSector do all the prepare work for deleting the physical sector.
// It add the related updates to update.batch, then create and return the operation
func (update *deleteSectorBatchUpdate) preparePhysicalSector(manager *storageManager, s *sector) (op writeaheadlog.Operation, err error) {
	// Update memory
	s.count = 0
	sf, exist := update.folders[s.folderID]
	if !exist {
		return writeaheadlog.Operation{}, fmt.Errorf("folder [%v] for sector [%x] not found", s.folderID, s.id)
	}
	if err = sf.setFreeSectorSlot(s.index); err != nil {
		return writeaheadlog.Operation{}, fmt.Errorf("set free sector for [%x] failed", s.id)
	}
	// update batch. Delete the sector, delete folder Sector, update folder
	update.batch = manager.db.deleteSectorToBatch(update.batch, s.id)
	update.batch = manager.db.deleteFolderSectorToBatch(update.batch, s.folderID, s.id)
	update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, sf)
	if err != nil {
		return writeaheadlog.Operation{}, fmt.Errorf("save storage folder to batch error: %v", err)
	}
	// Create the operation
	persist := deletePhysicalSectorPersist{
		ID:       s.id,
		FolderID: s.folderID,
		Index:    s.index,
		Count:    s.count,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	op = writeaheadlog.Operation{
		Name: opNameDeletePhysicalSector,
		Data: b,
	}
	return
}

// prepareVirtualSector prepares for the virtual sector
// It write to the update.batch, create and return the operation
func (update *deleteSectorBatchUpdate) prepareVirtualSector(manager *storageManager, s *sector) (op writeaheadlog.Operation, err error) {
	// Delete the virtual sector
	s.count--
	// Write the batch
	update.batch, err = manager.db.saveSectorToBatch(update.batch, s, false)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	persist := deleteVirtualSectorPersist{
		ID:       s.id,
		FolderID: s.folderID,
		Index:    s.index,
		Count:    s.count,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	op = writeaheadlog.Operation{
		Name: opNameDeleteVirtualSector,
		Data: b,
	}
	return
}

// processNormal process for the normal update. It simply commit the transaction and write
// the batch prepared in prepare stage.
func (update *deleteSectorBatchUpdate) processNormal(manager *storageManager) (err error) {
	if err = <-update.txn.Commit(); err != nil {
		return err
	}
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	return
}

// release release the deleteSectorBatchUpdate.
func (update *deleteSectorBatchUpdate) release(manager *storageManager, upErr *updateError) (err error) {
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
	// If there are some errors in preparing, revert the memory.
	if upErr.prepareErr != nil {
		_, err = update.revert(manager, false)
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
	// If there are some errors in processing, revert the memory and the database
	batch, newErr := update.revert(manager, true)
	err = common.ErrCompose(err, newErr)
	// Apply the revert batch
	if batch != nil {
		newErr = manager.db.writeBatch(batch)
		err = common.ErrCompose(err, newErr)
	}

	// Finally, release the transaction
	newErr = update.txn.Release()
	err = common.ErrCompose(err, newErr)
	return
}

// revert revert a deleteSectorBatchUpdate. if the revertDB flag is set to false, only memory
// will be reverted to previous state. If the revertDB flag is set to true, the related revert
// transaction will also be write to batch and be returned at the first argument.
func (update *deleteSectorBatchUpdate) revert(manager *storageManager, revertDB bool) (batch *leveldb.Batch, err error) {
	batch = manager.db.newBatch()
	// The order does not matter since the sectors are different and are place at
	// different locations
	for _, s := range update.sectors {
		if s.count == 0 {
			// The sector is physically deleted. In this case, the folder data in memory has to be
			// reverted.
			s.count = 1
			folder, exist := update.folders[s.folderID]
			if !exist {
				// This shall never happen
				return nil, fmt.Errorf("folder [%v] not exist", s.folderID)
			}
			_ = folder.setUsedSectorSlot(s.index)
			// if memory only flag is false, need to write the data to batch
			if revertDB {
				if batch, err = manager.db.saveSectorToBatch(batch, s, true); err != nil {
					return nil, fmt.Errorf("cannot save sector to batch")
				}
				if batch, err = manager.db.saveStorageFolderToBatch(batch, folder); err != nil {
					return nil, fmt.Errorf("cannot save the storage folder to batch")
				}
			}
		} else {
			// the sector is virtually deleted. The sector data need to be updated and flushed to
			// database. The folders needs not to be changed
			s.count++
			if revertDB {
				if batch, err = manager.db.saveSectorToBatch(batch, s, false); err != nil {
					return nil, fmt.Errorf("cannot save sector to batch")
				}
			}
		}
	}
	return batch, nil
}

// decodeDeleteSectorBatchUpdate decode the transaction to a deleteSectorBatchUpdate
func decodeDeleteSectorBatchUpdate(txn *writeaheadlog.Transaction) (update *deleteSectorBatchUpdate, err error) {
	if len(txn.Operations) == 0 {
		return nil, fmt.Errorf("transaction have operation length 0")
	}
	// decode the init persist
	var initPersist deleteBatchInitPersist
	if err = rlp.DecodeBytes(txn.Operations[0].Data, &initPersist); err != nil {
		return nil, fmt.Errorf("cannot decode init persist: %v", err)
	}
	update = &deleteSectorBatchUpdate{
		ids: initPersist.IDs,
		txn: txn,
	}
	return
}

func (update *deleteSectorBatchUpdate) prepareCommitted(manager *storageManager) (err error) {
	// folderPaths are the path to lock together
	var folderPaths []string
	for _, op := range update.txn.Operations[1:] {
		switch op.Name {
		case opNameDeletePhysicalSector:
			var persist deletePhysicalSectorPersist
			if err = rlp.DecodeBytes(op.Data, &persist); err != nil {
				err = fmt.Errorf("cannot decode appended persist: %v", err)
				return
			}
			s := &sector{
				id:       persist.ID,
				folderID: persist.FolderID,
				index:    persist.Index,
				count:    persist.Count,
			}
			update.sectors = append(update.sectors, s)
			// Find the folder path
			if _, exist := update.folders[s.folderID]; !exist {
				path, err := manager.db.getFolderPath(s.folderID)
				if err != nil {
					return err
				}
				folderPaths = append(folderPaths, path)
			}
		case opNameDeleteVirtualSector:
			var persist deleteVirtualSectorPersist
			if err = rlp.DecodeBytes(op.Data, &persist); err != nil {
				err = fmt.Errorf("cannot decode appended persist: %v", err)
				return
			}
			update.sectors = append(update.sectors, &sector{
				id:       persist.ID,
				folderID: persist.FolderID,
				index:    persist.Index,
				count:    persist.Count,
			})
		default:
			err = fmt.Errorf("unknown opName for %v: %v", opNameDeleteSectorBatch, op.Name)
			return
		}
	}
	// Lock and load all folders
	update.folders, err = manager.folders.getFolders(folderPaths)
	if err != nil {
		return err
	}
	return
}

func (update *deleteSectorBatchUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}
