package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/pkg/errors"
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

		// The index to insert
		index uint64

		// After the update, the count of the index
		count uint64

		// transaction is the transaction the update associated with
		txn *writeaheadlog.Transaction

		// batch is the in memory database operation set
		batch *leveldb.Batch
	}

	// addSectorInitPersist is the initial persist part for add sector update
	addSectorInitPersist struct {
		ID   sectorID
		Data []byte
	}

	// addSectorAppendPersist is the append part for add sector update
	addSectorAppendPersist struct {
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
	update.txn.Release()
	update.folder.lock.Unlock()
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
	manager.folders.selectFolderToAdd()
	return
}

func (update *addSectorUpdate) processNormal(manager *storageManager) (err error) {
	return
}
