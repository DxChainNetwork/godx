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

		// transaction is the transaction the update associated with
		txn *writeaheadlog.Transaction

		// batch is the in memory database operation set
		batch *leveldb.Batch

		// cached field for sector
		sector *sector

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
	addVirtualSectorAppendPersist struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// addSector add the sector to host manager
// whether the data has merkle root root is not validated here, and assumed valid
func (sm *storageManager) addSector(root common.Hash, data []byte) (err error) {
	if err = validateAddSector(root, data); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}
	//update := sm.createAddSectorUpdate(root, data)
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
	update.batch = new(leveldb.Batch)
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
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
	default:
		err = errors.New("invalid target")
	}
	return
}

// release release the update
func (update *addSectorUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	if update.txn != nil {
		// update.txn might be nil if the transaction have init error during prepare
		update.txn.Release()
	}

	if update.folder != nil {
		update.folder.lock.Unlock()
	}
	manager.sectorLocks.unlockSector(update.id)
	return
}

// decodeAddSectorUpdate decode the transaction to an addSectorUpdate
func decodeAddSectorUpdate(txn *writeaheadlog.Transaction) (update *addSectorUpdate, err error) {
	return
}

// prepareNormal execute the normal prepare process for addSectorUpdate
// In the prepare stage, find a folder with empty slot to insert the data
func (update *addSectorUpdate) prepareNormal(manager *storageManager) (err error) {
	// Try to find the sector in the database. If found, simply increment the count field
	manager.sectorLocks.lockSector(update.id)
	s, err := manager.db.getSector(update.id)
	var op writeaheadlog.Operation
	if err != nil && err != leveldb.ErrNotFound {
		// something unexpect error happened, return the error
		return
	} else if err == nil {
		// entry is found in database. This shall be a virtual sector update. Update the database
		// entry and there is no need to access the folder.
		update.physical = false
		update.sector = s
		update.sector.count += 1
		update.batch, err = manager.db.saveSectorToBatch(update.batch, update.sector, true)
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
		sf, index, err := manager.folders.selectFolderToAddWithRetry(maxFolderSelectionRetries)
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
			folderID: sf.id,
			index:    index,
			count:    1,
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
	if err = <-update.txn.Commit(); err != nil {
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

func (update *addSectorUpdate) processNormal(manager *storageManager) (err error) {
	return
}
