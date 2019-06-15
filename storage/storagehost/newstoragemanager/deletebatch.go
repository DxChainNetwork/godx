// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
	"strings"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
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
	if err = sm.tm.Add(); err != nil {
		return errStopped
	}
	defer sm.tm.Done()

	sm.lock.RLock()
	defer sm.lock.RUnlock()

	if len(roots) == 0 {
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
	return
}

func (update *deleteSectorBatchUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

func (update *deleteSectorBatchUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

func (update *deleteSectorBatchUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	return
}

// prepareNormal prepares for the normal update
func (update *deleteSectorBatchUpdate) prepareNormal(manager *storageManager) (err error) {
	// load and lock sectors and folders.
	if err = update.loadSectorsAndFolders(manager); err != nil {
		return fmt.Errorf("cannot load sectors and folders: %v", err)
	}
	var once sync.Once
	// update entries and write to database and wal
	for _, sector := range update.sectors {
		var op writeaheadlog.Operation
		if sector.count <= 1 {
			// Update memory
			sector.count = 0
			sf, exist := update.folders[sector.folderID]
			if !exist {
				return fmt.Errorf("folder [%v] for sector [%x] not found", sector.folderID, sector.id)
			}
			if err = sf.setFreeSectorSlot(sector.index); err != nil {
				return fmt.Errorf("set free sector for [%x] failed", sector.id)
			}
			// update batch. Delete the sector, delete folder Sector, update folder
			update.batch = manager.db.deleteSectorToBatch(update.batch, sector.id)
			update.batch = manager.db.deleteFolderSectorToBatch(update.batch, sector.folderID, sector.id)
			update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, sf)
			if err != nil {
				return fmt.Errorf("save storage folder to batch error: %v", err)
			}
			// Create the operation
			persist := deletePhysicalSectorPersist {
				ID: sector.id,
				FolderID: sector.folderID,
				Index: sector.index,
				Count: sector.count,
			}
			b, err := rlp.EncodeToBytes(persist)
			if err != nil {
				return err
			}
			op = writeaheadlog.Operation{
				Name: opNameDeletePhysicalSector,
				Data: b,
			}
		} else {
			// Delete the virtual sector
			sector.count--
			// Write the batch
			update.batch, err = manager.db.saveSectorToBatch(update.batch, sector, false)
			if err != nil {
				return err
			}
			persist := deleteVirtualSectorPersist{
				ID: sector.id,
				FolderID: sector.folderID,
				Index: sector.index,
				Count: sector.count,
			}
			b, err := rlp.EncodeToBytes(persist)
			if err != nil {
				return err
			}
			op = writeaheadlog.Operation{
				Name: opNameDeleteVirtualSector,
				Data: b,
			}
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
	manager.sectorLocks.lockSectors(update.ids)
	folderPaths := make([]string, 0)
	// Get all sectors and get related folder paths
	for _, id := range update.ids {
		s, err := manager.db.getSector(id)
		if err != nil {
			return
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

func (update *deleteSectorBatchUpdate) processNormal(manager *storageManager) (err error) {
	if err = <- update.txn.Commit(); err != nil {
		return err
	}
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	return
}

func decodeDeleteSectorBatchUpdate(txn *writeaheadlog.Transaction) (update *deleteSectorBatchUpdate, err error) {
	return
}

func (update *deleteSectorBatchUpdate) lockResource(manager *storageManager) (err error) {
	return
}

func (update *deleteSectorBatchUpdate) prepareCommitted(manager *storageManager) (err error) {
	return
}

func (update *deleteSectorBatchUpdate) processCommitted(manager *storageManager) (err error) {
	return
}
