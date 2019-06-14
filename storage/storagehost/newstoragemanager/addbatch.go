// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

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

func (sm *storageManager) AddSectorBatch(roots []common.Hash) (err error) {
	// If no root input, no need to update. Simply return a nil error
	if len(roots) == 0 {
		return
	}
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
	if err = manager.tm.Add(); err != nil {
		return errStopped
	}
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
	fmt.Println("prepare")
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

// prepare is the function to be called during process stage
func (update *addSectorBatchUpdate) process(manager *storageManager, target uint8) (err error) {
	fmt.Println("process")
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

// release is to release the update and do error handling
// Checks for the existence of all sectors and append to the transaction
func (update *addSectorBatchUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	fmt.Println("lalalal", upErr)
	// release all sector locks
	defer func() {
		// release all locks
		for _, s := range update.sectors {
			//fmt.Printf("try unlocking %x\n", s.id)
			manager.sectorLocks.unlockSector(s.id)
			fmt.Printf("unlocked %x\n", s.id)
		}
	}()
	//fmt.Println("length", len(update.ids))
	//for _, id := range update.ids {
	//fmt.Printf("releasing %x\n", id)
	//}
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
		err = update.txn.Release()
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
	fmt.Println(1)
	for _, id := range update.ids {
		// lock the sector
		fmt.Printf("try locking %x\n", id)
		manager.sectorLocks.lockSector(id)
		fmt.Printf("locked %x\n", id)
		s, err := manager.db.getSector(id)
		if err != nil {
			manager.sectorLocks.unlockSector(id)
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
	fmt.Println(2)
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
	fmt.Println(3)
	if err = <-update.txn.Commit(); err != nil {
		return
	}
	if err = manager.db.writeBatch(update.batch); err != nil {
		return
	}
	fmt.Println(4)
	return
}

// prepareCommitted prepare for the recovery
func (update *addSectorBatchUpdate) prepareCommitted(manager *storageManager) (err error) {
	return nil
}

// processCommitted process for the recovery
func (update *addSectorBatchUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}
