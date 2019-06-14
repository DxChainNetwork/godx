// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

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
	// addSectorUpdate is the update to add a sector
	addSectorUpdate struct {
		// user input fields
		id   sectorID
		data []byte

		// The folder to add sector
		folder *storageFolder

		// sector to add
		sector *sector

		// transaction is the transaction the update associated with
		txn *writeaheadlog.Transaction

		// batch is the in memory database operation set
		batch *leveldb.Batch

		// physical is the flag for whether this update is to add a physical sector or not
		physical bool
	}

	// addSectorInitPersist is the initial persist part for add sector update
	addSectorInitPersist struct {
		ID   sectorID
		Data []byte
	}

	// addPhysicalSectorAppendPersist is the append part for add physical sector update
	addPhysicalSectorAppendPersist struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}

	// addVirtualSectorAppendPersist is the append part for adding virtual sector update
	// Note this structure happens to be the same as addPhysicalSectorAppendPersist.
	// Do not refactor this because of the consideration of extensibility
	addVirtualSectorAppendPersist struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// AddSector add the sector to host manager
// whether the data has merkle root root is not validated here, and assumed valid
func (sm *storageManager) AddSector(root common.Hash, data []byte) (err error) {
	if err = sm.tm.Add(); err != nil {
		return errStopped
	}
	defer sm.tm.Done()
	// validate the add sector request
	if err = validateAddSector(root, data); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}
	// create the update
	update := sm.createAddSectorUpdate(root, data)
	// record the add sector intent
	if err = update.recordIntent(sm); err != nil {
		return
	}
	// prepare, process and release the update
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

// validateAddSector validate the input of add sector request
// It checks whether the input data size is larger than the sector size
func validateAddSector(root common.Hash, data []byte) (err error) {
	if len(data) > int(storage.SectorSize) {
		return fmt.Errorf("add sector give data length exceed limit: %v > %v", len(data), storage.SectorSize)
	}
	return nil
}

// createAddSectorUpdate create a addSectorUpdate
func (sm *storageManager) createAddSectorUpdate(root common.Hash, data []byte) (update *addSectorUpdate) {
	sectorID := sm.calculateSectorID(root)
	// copy the data
	dataCpy := make([]byte, storage.SectorSize)
	copy(dataCpy, data)
	// create an update with copied data
	update = &addSectorUpdate{
		id:   sectorID,
		data: dataCpy,
	}
	return
}

// str define the string representation of the update
func (update *addSectorUpdate) str() (s string) {
	return fmt.Sprintf("Add sector with id [%x]", update.id)
}

// recordIntent record the intent for the update
// 1. Create the update
// 2. write the initial transaction to wal
func (update *addSectorUpdate) recordIntent(manager *storageManager) (err error) {
	pUpdate := addSectorInitPersist{
		ID:   update.id,
		Data: update.data,
	}
	b, err := rlp.EncodeToBytes(pUpdate)
	if err != nil {
		return
	}
	op := writeaheadlog.Operation{
		Name: opNameAddSector,
		Data: b,
	}
	update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op})
	if err != nil {
		update.txn = nil
		return fmt.Errorf("cannot create transaction: %v", err)
	}
	return
}

// prepare prepares for the update
func (update *addSectorUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
		if !update.physical && manager.disrupter.disrupt("virtual prepare normal") {
			return errDisrupted
		}
		if update.physical && manager.disrupter.disrupt("physical prepare normal") {
			return errDisrupted
		}
		if !update.physical && manager.disrupter.disrupt("virtual prepare normal stop") {
			return errStopped
		}
		if update.physical && manager.disrupter.disrupt("physical prepare normal stop") {
			return errStopped
		}
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

// process process the update
func (update *addSectorUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
		if !update.physical && manager.disrupter.disrupt("virtual process normal") {
			return errDisrupted
		}
		if update.physical && manager.disrupter.disrupt("physical process normal") {
			return errDisrupted
		}
		if !update.physical && manager.disrupter.disrupt("virtual process normal stop") {
			return errStopped
		}
		if update.physical && manager.disrupter.disrupt("physical process normal stop") {
			return errStopped
		}
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

// release release the update
func (update *addSectorUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	defer func() {
		if update.folder != nil {
			update.folder.lock.Unlock()
		}
		manager.sectorLocks.unlockSector(update.id)
	}()
	// If no error happened, simply release the transaction
	if upErr == nil || upErr.isNil() {
		err = update.txn.Release()
		return
	}
	// If storage manager has been stopped, no release, do nothing and return
	if upErr.hasErrStopped() {
		upErr.processErr = nil
		upErr.prepareErr = nil
		return
	}
	// There is some error happened, revert the memory update.
	// For recovery situations, the update might not be stored in database. So
	// error might happen as a normal situation. Simply skip the error
	if update.folder != nil && update.physical {
		_ = update.folder.setFreeSectorSlot(update.sector.index)
	}
	// If prepare process has error, release the transaction and return
	if upErr.prepareErr != nil {
		return
	}
	batch := manager.db.newBatch()
	// If process has error, need to reverse the batch update
	if update.physical {
		// Need to delete the sector and folder id to sector mapping
		if update.sector != nil {
			batch.Delete(makeSectorKey(update.sector.id))
		}
		// Need to delete the mapping from folder id to sector id
		if update.folder != nil {
			batch.Delete(makeFolderSectorKey(update.folder.id, update.sector.id))
		}
	} else {
		// Need to put the previous sector data to database
		var newErr error
		update.sector.count--
		batch, newErr = manager.db.saveSectorToBatch(batch, update.sector, false)
		if newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}
	// Update the folder
	if update.folder != nil {
		var newErr error
		batch, newErr = manager.db.saveStorageFolderToBatch(batch, update.folder)
		if newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}
	if newErr := manager.db.writeBatch(batch); newErr != nil {
		err = common.ErrCompose(err, newErr)
	}
	// release the transaction
	if update.txn != nil {
		if newErr := update.txn.Release(); newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}
	return
}

// prepareNormal execute the normal prepare process for addSectorUpdate
// In the prepare stage, find a folder with empty slot to insert the data
func (update *addSectorUpdate) prepareNormal(manager *storageManager) (err error) {
	// Try to find the sector in the database. If found, simply increment the count field
	manager.sectorLocks.lockSector(update.id)
	s, existErr := manager.db.getSector(update.id)
	var op writeaheadlog.Operation
	if existErr != nil && existErr != leveldb.ErrNotFound {
		// something unexpect error happened, return the error
		return
	} else if existErr == nil {
		// entry is found in database. This shall be a virtual sector update. Update the database
		// entry and there is no need to access the folder.
		update.physical = false
		update.sector = s
		update.sector.count += 1
		update.batch, err = manager.db.saveSectorToBatch(update.batch, update.sector, false)
		if err != nil {
			return
		}
		op, err = update.createVirtualSectorAppendOperation()
		if err != nil {
			return
		}
	} else {
		// This is the case to add a new physical sector.
		// The appended operation should have the name opNamePhysicalSector
		update.physical = true
		var sf *storageFolder
		var index uint64
		sf, index, err = manager.folders.selectFolderToAddWithRetry(maxFolderSelectionRetries)
		if err != nil {
			// If there is error, it can only be errAllFoldersFullOrUsed.
			// In this case, return the err
			return
		}
		if err = sf.setUsedSectorSlot(index); err != nil {
			return
		}
		update.folder = sf
		update.sector = &sector{
			id:       update.id,
			folderID: sf.id,
			index:    index,
			count:    1,
		}
		// Apply the sector update to batch
		update.batch, err = manager.db.saveSectorToBatch(update.batch, update.sector, true)
		if err != nil {
			return
		}
		update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, sf)
		if err != nil {
			return
		}
		// create the operation
		op, err = update.createPhysicalSectorAppendOperation()
		if err != nil {
			return
		}
	}
	// Wait for the initialization of the transaction to complete and append the transaction
	if <-update.txn.InitComplete; update.txn.InitErr != nil {
		update.txn = nil
		err = update.txn.InitErr
		return
	}
	err = <-update.txn.Append([]writeaheadlog.Operation{op})
	if err != nil {
		return
	}
	return
}

// createVirtualSectorAppendOperation create an operation to append for add virtual sector
func (update *addSectorUpdate) createVirtualSectorAppendOperation() (op writeaheadlog.Operation, err error) {
	persist := addVirtualSectorAppendPersist{
		FolderID: update.sector.folderID,
		Index:    update.sector.index,
		Count:    update.sector.count,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: opNameAddVirtualSector,
		Data: b,
	}, nil
}

// createPhysicalSectorAppendOperation create an operation to append for adding physical sector
func (update *addSectorUpdate) createPhysicalSectorAppendOperation() (op writeaheadlog.Operation, err error) {
	persist := addPhysicalSectorAppendPersist{
		FolderID: update.sector.folderID,
		Index:    update.sector.index,
		Count:    update.sector.count,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return writeaheadlog.Operation{}, err
	}
	return writeaheadlog.Operation{
		Name: opNameAddPhysicalSector,
		Data: b,
	}, nil
}

// processNormal is to process normally for add sector update
// 1. If is a physical update, insert the data to the folder file
// 2. Apply the database update
func (update *addSectorUpdate) processNormal(manager *storageManager) (err error) {
	if err = <-update.txn.Commit(); err != nil {
		return
	}
	if update.physical {
		_, err = update.folder.dataFile.WriteAt(update.data, int64(update.sector.index*storage.SectorSize))
		if err != nil {
			return
		}
	}
	if err = manager.db.writeBatch(update.batch); err != nil {
		return
	}
	return
}

// decodeAddSectorUpdate decode the transaction to an addSectorUpdate
func decodeAddSectorUpdate(txn *writeaheadlog.Transaction) (update *addSectorUpdate, err error) {
	if len(txn.Operations) == 0 {
		return nil, errors.New("empty transaction")
	}
	// The first operation must have addSectorInitPersist
	initBytes := txn.Operations[0].Data
	var initPersist addSectorInitPersist
	if err = rlp.DecodeBytes(initBytes, &initPersist); err != nil {
		return
	}
	update = &addSectorUpdate{
		id:   initPersist.ID,
		data: initPersist.Data,
		txn:  txn,
	}
	// Not appended
	if len(txn.Operations) == 1 {
		return
	}
	// Depend on the name of the second operation, assign value to update.sector
	appendOp := txn.Operations[1]
	switch appendOp.Name {
	case opNameAddPhysicalSector:
		update.physical = true
		var persist addPhysicalSectorAppendPersist
		if err = rlp.DecodeBytes(appendOp.Data, &persist); err != nil {
			return
		}
		update.sector = &sector{
			id:       update.id,
			folderID: persist.FolderID,
			index:    persist.Index,
			count:    persist.Count,
		}
	case opNameAddVirtualSector:
		update.physical = false
		var persist addVirtualSectorAppendPersist
		if err = rlp.DecodeBytes(appendOp.Data, &persist); err != nil {
			return
		}
		update.sector = &sector{
			id:       update.id,
			folderID: persist.FolderID,
			index:    persist.Index,
			count:    persist.Count,
		}
	default:
		err = fmt.Errorf("unknown operation name: %v", txn.Operations[1].Name)
	}
	return
}

// lockResource locks the resource during recover
func (update *addSectorUpdate) lockResource(manager *storageManager) (err error) {
	manager.sectorLocks.lockSector(update.id)
	folderPath, err := manager.db.getFolderPath(update.sector.folderID)
	if err != nil {
		manager.sectorLocks.unlockSector(update.id)
		return
	}
	manager.folders.lock.RLock()
	update.folder, err = manager.folders.get(folderPath)
	manager.folders.lock.RUnlock()
	if err != nil {
		return err
	}
	return
}

// prepareCommitted prepare for committed transaction
func (update *addSectorUpdate) prepareCommitted(manager *storageManager) (err error) {
	return
}

// processCommitted process for committed transaction
func (update *addSectorUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}
