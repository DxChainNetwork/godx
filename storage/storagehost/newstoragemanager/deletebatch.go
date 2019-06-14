// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"errors"
	"strings"

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

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	// deleteBatchInitPersist is the persist to recorded in record intent
	deleteBatchInitPersist struct {
		ids []sectorID
	}

	// deleteVritualBatchPersist is the persist for appended operations for deleting a virtual sector
	deleteVritualSectorPersist struct {
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
	if len(roots) == 0 {
		return
	}
	return
}

func (sm *storageManager) createDeleteSectorBatchUpdate(roots []common.Hash) (update *deleteSectorBatchUpdate) {
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

func (update *deleteSectorBatchUpdate) recordIntent(manager *storageManager) (err error) {
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

func (update *deleteSectorBatchUpdate) prepareNormal(manager *storageManager) (err error) {
	return
}

func (update *deleteSectorBatchUpdate) processNormal(manager *storageManager) (err error) {
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
