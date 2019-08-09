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
	// addSectorBatchUpdate is the update from function AddSectorBatch
	addSectorBatchUpdate struct {
		// ids is the function arguments
		ids []sectorID

		// sectors is the in memory log for the sector batch
		sectors []*sector

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	// addSectorBatchInitPersist is the initial persist
	addSectorBatchInitPersist struct {
		IDs []sectorID
	}

	// addSectorBatchAppendPersist is the persist for appended operations
	addSectorBatchAppendPersist struct {
		ID       sectorID
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// AddSectorBatch add the sectors to the storage manager. The sectors are assumed to exist in
// storage manager before calling this function. The operation is atomic, that is, if any error
// happened during the update, all the operation will not be performed.
//
// Note:
//   1. The added sectors must be previously stored in storage manager, else return an error
//   2. Each two of the added sectors must not share the same root, else the function will
//      be blocked permanently.
func (sm *storageManager) AddSectorBatch(roots []common.Hash) (err error) {
	// If no root input, no need to update. Simply return a nil error
	if len(roots) == 0 {
		return
	}
	// Lock the storage Manager
	sm.lock.Lock()
	defer sm.lock.Unlock()
	// create the update and record the intent
	update := sm.createAddSectorBatchUpdate(roots)
	if err = update.recordIntent(sm); err != nil {
		return err
	}
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

// createAddSectorBatchUpdate creates an addSectorBatchUpdate
func (sm *storageManager) createAddSectorBatchUpdate(roots []common.Hash) (update *addSectorBatchUpdate) {
	// copy the ids
	update = &addSectorBatchUpdate{
		ids: make([]sectorID, 0, len(roots)),
	}
	for _, root := range roots {
		id := sm.calculateSectorID(root)
		update.ids = append(update.ids, id)
	}
	return
}

// str defines the string representation of the update
func (update *addSectorBatchUpdate) str() (s string) {
	s = "Add sector batch\n[\n\t"
	idStrs := make([]string, 0, len(update.ids))
	for _, id := range update.ids {
		idStrs = append(idStrs, common.Bytes2Hex(id[:]))
	}
	s += strings.Join(idStrs, ",\n\t")
	s += "\n]"
	return
}

// recordIntent records the intent for an addSectorBatch update
func (update *addSectorBatchUpdate) recordIntent(manager *storageManager) (err error) {
	persist := addSectorBatchInitPersist{
		IDs: update.ids,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return
	}
	op := writeaheadlog.Operation{
		Name: opNameAddSectorBatch,
		Data: b,
	}
	update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op})
	if err != nil {
		update.txn = nil
		return fmt.Errorf("cannot create trnasaction: %v", err)
	}
	return
}

// prepare is the function to be called during prepare stage
func (update *addSectorBatchUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
		if manager.disruptor.disrupt("add batch prepare") {
			return errDisrupted
		}
		if manager.disruptor.disrupt("add batch prepare stop") {
			return errStopped
		}
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

// prepare is the function to be called during process stage
func (update *addSectorBatchUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
		if manager.disruptor.disrupt("add batch process stop") {
			return errStopped
		}
		if manager.disruptor.disrupt("add batch process") {
			return errDisrupted
		}
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

// release is to release the update and do error handling
// Checks for the existence of all sectors and append to the transaction
func (update *addSectorBatchUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	// release all sector locks
	// If no error happened, release the transaction
	if upErr == nil || upErr.isNil() {
		err = update.txn.Release()
		return
	}
	// If have errStopped, do nothing and return
	if upErr.hasErrStopped() {
		upErr.processErr = nil
		upErr.prepareErr = nil
		return
	}
	// If some error happened at prepare stage, no need to do anything, just release the transaction
	if upErr.prepareErr != nil {
		// revert the memory
		for _, s := range update.sectors {
			if s.count > 1 {
				s.count--
			}
		}
		// release the transaction
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
	// If some error happened at process stage, we need to revert the db sectors.
	// And then release the transaction
	batch := manager.db.newBatch()
	for _, s := range update.sectors {
		// s.count == 1 and s.count == 0 shall never happen.
		// The following check is redundancy in code to avoid integer overflow
		if s.count > 1 {
			s.count--
		}
		var newErr error
		batch, newErr = manager.db.saveSectorToBatch(batch, s, false)
		if newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}
	if newErr := manager.db.writeBatch(batch); newErr != nil {
		err = common.ErrCompose(err, newErr)
	}
	if newErr := update.txn.Release(); newErr != nil {
		err = common.ErrCompose(err, newErr)
	}
	return
}

// prepareNormal prepare for the normal execution
func (update *addSectorBatchUpdate) prepareNormal(manager *storageManager) (err error) {
	var once sync.Once
	for _, id := range update.ids {
		s, err := manager.db.getSector(id)
		if err != nil {
			return fmt.Errorf("get sector [%x]: %v", id, err)
		}
		// increment the count field
		s.count++
		// Write to memory
		update.sectors = append(update.sectors, s)
		// Write to batch
		update.batch, err = manager.db.saveSectorToBatch(update.batch, s, false)
		if err != nil {
			return fmt.Errorf("cannot save sector [%v]: %v", id, err)
		}
		// Write to wal transaction
		op, err := createAddBatchAppendOperation(id, s.folderID, s.index, s.count)
		if err != nil {
			return err
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
		// append the prepared op to transaction
		if err = <-update.txn.Append([]writeaheadlog.Operation{op}); err != nil {
			return err
		}
	}
	return
}

// createAddBatchAppendPersist is the helper function to create an append operation
func createAddBatchAppendOperation(id sectorID, folderID folderID, index, count uint64) (op writeaheadlog.Operation, err error) {
	persist := addSectorBatchAppendPersist{
		ID:       id,
		FolderID: folderID,
		Index:    index,
		Count:    count,
	}
	walBytes, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	op = writeaheadlog.Operation{
		Name: opNameAddSectorBatchAppend,
		Data: walBytes,
	}
	return op, nil
}

// processNormal process for the normal execution
func (update *addSectorBatchUpdate) processNormal(manager *storageManager) (err error) {
	if err = <-update.txn.Commit(); err != nil {
		return
	}
	if err = manager.db.writeBatch(update.batch); err != nil {
		return
	}
	return
}

// decodeAddSectorBatchUpdate decode the addSectorBatchUpdate from the transaction
func decodeAddSectorBatchUpdate(txn *writeaheadlog.Transaction) (update *addSectorBatchUpdate, err error) {
	if len(txn.Operations) == 0 {
		return nil, errors.New("empty transaction")
	}
	// The first operation is addSectorBatchInitPersist
	initBytes := txn.Operations[0].Data
	var initPersist addSectorBatchInitPersist
	if err = rlp.DecodeBytes(initBytes, &initPersist); err != nil {
		return
	}
	update = &addSectorBatchUpdate{
		ids: initPersist.IDs,
		txn: txn,
	}
	if len(txn.Operations) == 1 {
		return
	}
	// Decode the append operations
	for _, appendOp := range txn.Operations[1:] {
		if appendOp.Name != opNameAddSectorBatchAppend {
			return nil, errors.New("for addSectorBatchUpdate, operation name not expected")
		}
		var persist addSectorBatchAppendPersist
		if err = rlp.DecodeBytes(appendOp.Data, &persist); err != nil {
			return nil, err
		}
		s := &sector{
			id:       persist.ID,
			folderID: persist.FolderID,
			index:    persist.Index,
			count:    persist.Count,
		}
		update.sectors = append(update.sectors, s)
	}
	return update, nil
}

// prepareCommitted prepare for the recovery
func (update *addSectorBatchUpdate) prepareCommitted(manager *storageManager) (err error) {
	return nil
}

// processCommitted process for the recovery
func (update *addSectorBatchUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}
