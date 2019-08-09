// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"errors"
	"fmt"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"

	"github.com/syndtr/goleveldb/leveldb"
)

// shrinkFolderUpdate shrinks the folder to the target size.
// The processing of shrinkFolderUpdates acquires an exclusive lock from the module,
// so no worry about locks in this update. The update will relocate the sectors that
// needs to be relocated because the folder shrinks.
type (
	shrinkFolderUpdate struct {
		folderPath string

		// numSectors before the update
		prevNumSectors uint64

		// numSectors after the update
		targetNumSectors uint64

		// The folder to shrink
		targetFolder *storageFolder

		// entries of relocates
		relocates []sectorRelocation

		// related storage folders as a map
		folders map[folderID]*storageFolder

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	shrinkFolderInitPersist struct {
		FolderPath       string
		PrevNumSectors   uint64
		TargetNumSectors uint64
	}

	sectorRelocation struct {
		ID           sectorID
		PrevLocation sectorLocation
		NewLocation  sectorLocation
	}

	sectorLocation struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// shrinkFolder shrink the folder to the target size
func (sm *storageManager) shrinkFolder(folderPath string, targetSize uint64) (err error) {
	update := createShrinkFolderUpdate(folderPath, targetSize)
	if err = update.recordIntent(sm); err != nil {
		return err
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

// createShrinkFolderUpdate create the shrink folder update
func createShrinkFolderUpdate(folderPath string, targetSize uint64) (update *shrinkFolderUpdate) {
	update = &shrinkFolderUpdate{
		folderPath:       folderPath,
		targetNumSectors: sizeToNumSectors(targetSize),
		folders:          make(map[folderID]*storageFolder),
	}
	return update
}

// str defines the string representation fo the shrinkFolderUpdate
func (update *shrinkFolderUpdate) str() (s string) {
	s = fmt.Sprintf("shrink folder [%v] to %v bytes", update.folderPath, numSectorsToSize(update.targetNumSectors))
	return
}

// recordIntent record the intent to shrink the folder
func (update *shrinkFolderUpdate) recordIntent(manager *storageManager) (err error) {
	// validate the shrink update. Checks for after the shrink, whether the rest of the folders could be stored
	if err = manager.folders.validateShrink(update.folderPath, update.targetNumSectors); err != nil {
		return
	}
	// get the storage folder from folders
	update.targetFolder, err = manager.folders.get(update.folderPath)
	if err != nil {
		return err
	}
	update.prevNumSectors = update.targetFolder.numSectors

	// record the intent
	persist := shrinkFolderInitPersist{
		FolderPath:       update.folderPath,
		PrevNumSectors:   update.targetFolder.numSectors,
		TargetNumSectors: update.targetNumSectors,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return err
	}
	op := writeaheadlog.Operation{
		Name: opNameShrinkFolder,
		Data: b,
	}
	if update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op}); err != nil {
		return err
	}
	return
}

// prepare prepares for the shrink folder update
func (update *shrinkFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
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

// process process for the shrink folder update
func (update *shrinkFolderUpdate) process(manager *storageManager, target uint8) (err error) {
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

// prepareNormal prepares for the shrinkFolderFolder update as normal execution
func (update *shrinkFolderUpdate) prepareNormal(manager *storageManager) (err error) {
	var once sync.Once
	update.targetFolder.status = folderUnavailable
	update.targetFolder.numSectors = update.targetNumSectors
	update.folders[update.targetFolder.id] = update.targetFolder

	// get all related sectors
	ids := manager.db.getAllSectorsIDsFromFolder(update.targetFolder.id)
	for _, id := range ids {
		oldSector, err := manager.db.getSector(id)
		if err != nil {
			return err
		}
		if oldSector.index < update.targetNumSectors {
			// No need to update the sector
			continue
		}
		// oldSector needs to be relocated. First try to relocate the oldSector in the same folder
		// If the folder is full, then try to relocate the oldSector to other folders
		relocate, err := update.relocateSector(manager, oldSector)
		if err != nil {
			return err
		}
		update.relocates = append(update.relocates, relocate)
		// Append the transaction
		once.Do(func() {
			if <-update.txn.InitComplete; update.txn.InitErr != nil {
				err = update.txn.InitErr
				return
			}
		})
		if err != nil {
			return err
		}
		b, err := rlp.EncodeToBytes(relocate)
		if err != nil {
			return err
		}
		op := writeaheadlog.Operation{
			Name: opNameRelocateSector,
			Data: b,
		}
		if err = <-update.txn.Append([]writeaheadlog.Operation{op}); err != nil {
			return err
		}
		// Append the database batch
		newSector := &sector{
			id:       relocate.ID,
			folderID: relocate.NewLocation.FolderID,
			index:    relocate.NewLocation.Index,
			count:    relocate.NewLocation.Count,
		}
		update.batch, err = manager.db.saveSectorToBatch(update.batch, newSector, true)
		if err != nil {
			return err
		}
		if relocate.NewLocation.FolderID != relocate.PrevLocation.FolderID {
			update.batch = manager.db.deleteFolderSectorToBatch(update.batch, oldSector.folderID, oldSector.id)
			update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, update.folders[relocate.NewLocation.FolderID])
			if err != nil {
				return err
			}
		}
	}
	// Finally, shrink the folder, and add to batch
	update.targetFolder.usage = shrinkUsage(update.targetFolder.usage, update.prevNumSectors)
	update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, update.targetFolder)
	if err != nil {
		return err
	}
	if manager.disruptor.disrupt("shrink folder prepare normal") {
		return errDisrupted
	}
	if manager.disruptor.disrupt("shrink folder prepare normal stop") {
		return errStopped
	}
	return
}

// relocateSector relocate the sector. First try to relocate in the same folder, then
// find other folders to relocate
func (update *shrinkFolderUpdate) relocateSector(manager *storageManager, s *sector) (relocate sectorRelocation, err error) {
	index, err := update.targetFolder.freeSectorIndex()
	var relocatedFolder *storageFolder
	if err == nil {
		// the s can be filled in
		relocatedFolder = update.targetFolder
	} else if err == errFolderAlreadyFull {
		relocatedFolder, index, err = manager.folders.selectFolderToAdd()
		if err != nil {
			return sectorRelocation{}, err
		}
	} else {
		return sectorRelocation{}, fmt.Errorf("cannot get free s index")
	}
	if _, exist := update.folders[relocatedFolder.id]; !exist {
		update.folders[relocatedFolder.id] = relocatedFolder
	}
	// Update the memory
	if err = update.targetFolder.setFreeSectorSlot(s.index); err != nil {
		return sectorRelocation{}, err
	}
	if err = relocatedFolder.setUsedSectorSlot(index); err != nil {
		_ = update.targetFolder.setFreeSectorSlot(s.index)
		return sectorRelocation{}, err
	}
	relocate = sectorRelocation{
		ID: s.id,
		PrevLocation: sectorLocation{
			s.folderID, s.index, s.count,
		},
		NewLocation: sectorLocation{
			relocatedFolder.id, index, s.count,
		},
	}
	return relocate, nil
}

// prepare committed is to prepare for txn recover. It loads folders (old folders and
// target folders) to update
func (update *shrinkFolderUpdate) prepareCommitted(manager *storageManager) (err error) {
	// load all folders to update
	sf, err := manager.folders.get(update.folderPath)
	if err != nil {
		return err
	}
	update.targetFolder = sf
	update.folders[sf.id] = sf
	for _, relocate := range update.relocates {
		path, err := manager.db.getFolderPath(relocate.NewLocation.FolderID)
		if err != nil {
			return err
		}
		sf, err = manager.folders.get(path)
		if err != nil {
			return err
		}
		update.folders[sf.id] = sf
	}
	return
}

// processNormal process for normal execution of the update
func (update *shrinkFolderUpdate) processNormal(manager *storageManager) (err error) {
	// commit the transaction
	if err = <-update.txn.Commit(); err != nil {
		return err
	}
	// write the data from prevLocation to afterLocation
	b := make([]byte, storage.SectorSize)
	for _, relocate := range update.relocates {
		// read data
		prevIndex := relocate.PrevLocation.Index
		n, err := update.targetFolder.dataFile.ReadAt(b, int64(prevIndex*storage.SectorSize))
		if err != nil || uint64(n) != storage.SectorSize {
			return fmt.Errorf("not read full sector")
		}
		// write data
		targetFolder, exist := update.folders[relocate.NewLocation.FolderID]
		if !exist {
			return fmt.Errorf("folder not in folders")
		}
		newIndex := relocate.NewLocation.Index
		n, err = targetFolder.dataFile.WriteAt(b, int64(newIndex*storage.SectorSize))
		if err != nil || n != int(storage.SectorSize) {
			return fmt.Errorf("not full write")
		}
	}
	// write the db batch
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	if manager.disruptor.disrupt("shrink folder process normal") {
		return errDisrupted
	}
	if manager.disruptor.disrupt("shrink folder process normal stop") {
		return errStopped
	}
	if err = update.targetFolder.dataFile.Truncate(int64(numSectorsToSize(update.targetNumSectors))); err != nil {
		return err
	}
	return
}

// processCommitted process for recovered transaction. It simply return an error
func (update *shrinkFolderUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}

// release releases the shrinkFolderUpdate based on the error
func (update *shrinkFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	defer func() {
		if err == nil {
			update.targetFolder.status = folderAvailable
		}
	}()
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
		// revert memory
		err = update.revert(manager, true)
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
	// Check whether the file has been truncated
	info, newErr := update.targetFolder.dataFile.Stat()
	err = common.ErrCompose(err, newErr)
	if newErr == nil && info.Size() != int64(numSectorsToSize(update.prevNumSectors)) {
		// the folder has been truncated. Only truncate the file to previous size, and
		// revert the folder db info. The sectors can reside in new locations
		update.targetFolder.numSectors = update.prevNumSectors
		update.targetFolder.usage = expandUsage(update.targetFolder.usage, update.targetNumSectors)
		newErr = update.targetFolder.dataFile.Truncate(int64(numSectorsToSize(update.targetNumSectors)))
		err = common.ErrCompose(err, newErr)
		newErr = manager.db.saveStorageFolder(update.targetFolder)
		err = common.ErrCompose(err, newErr)
		// release the transaction
		newErr = update.txn.Release()
		err = common.ErrCompose(err, newErr)
		return
	}
	// The file is not truncated. So all the data are still safely stored on the original file.
	// It would be safe to revert all the relocates.
	newErr = update.revert(manager, false)
	err = common.ErrCompose(err, newErr)
	// release the transaction
	newErr = update.txn.Release()
	err = common.ErrCompose(err, newErr)
	return
}

// revert will revert the updates in the shrinkFolderUpdate
func (update *shrinkFolderUpdate) revert(manager *storageManager, memoryOnly bool) (err error) {
	batch := manager.db.newBatch()
	var newErr error
	// first grow the folder to prevSize
	if update.targetFolder.numSectors != update.prevNumSectors {
		update.targetFolder.numSectors = update.prevNumSectors
		update.targetFolder.usage = expandUsage(update.targetFolder.usage, update.prevNumSectors)
	}
	// Then update relocates
	for _, relocate := range update.relocates {
		prevLocation := relocate.PrevLocation
		newLocation := relocate.NewLocation
		_ = update.folders[prevLocation.FolderID].setUsedSectorSlot(prevLocation.Index)
		_ = update.folders[newLocation.FolderID].setFreeSectorSlot(newLocation.Index)
		if !memoryOnly {
			s := &sector{
				id:       relocate.ID,
				folderID: prevLocation.FolderID,
				index:    prevLocation.Index,
				count:    prevLocation.Count,
			}
			batch, newErr = manager.db.saveSectorToBatch(batch, s, true)
			if newErr != nil {
				err = common.ErrCompose(err, newErr)
				continue
			}
			if prevLocation.FolderID == newLocation.FolderID {
				// No further update needed
				continue
			}
			// sector is moved to a new folder. Revert this
			batch, newErr = manager.db.saveStorageFolderToBatch(batch, update.folders[newLocation.FolderID])
			if newErr != nil {
				err = common.ErrCompose(err, newErr)
				continue
			}
			batch = manager.db.deleteFolderSectorToBatch(batch, newLocation.FolderID, relocate.ID)
		}
	}
	if !memoryOnly {
		batch, newErr = manager.db.saveStorageFolderToBatch(batch, update.targetFolder)
		err = common.ErrCompose(err, newErr)
	}
	if newErr = manager.db.writeBatch(batch); newErr != nil {
		err = common.ErrCompose(err, newErr)
		return
	}
	return
}

// decodeShrinkFolderUpdate decode the shrinkFolderUpdate
func decodeShrinkFolderUpdate(txn *writeaheadlog.Transaction) (update *shrinkFolderUpdate, err error) {
	var initPersist shrinkFolderInitPersist
	if err = rlp.DecodeBytes(txn.Operations[0].Data, &initPersist); err != nil {
		return nil, err
	}
	update = &shrinkFolderUpdate{
		folderPath:       initPersist.FolderPath,
		prevNumSectors:   initPersist.PrevNumSectors,
		targetNumSectors: initPersist.TargetNumSectors,
		folders:          make(map[folderID]*storageFolder),
	}
	// decode the rest
	for _, op := range txn.Operations[1:] {
		if op.Name != opNameRelocateSector {
			return nil, fmt.Errorf("invalid op name: %v", op.Name)
		}
		var relocate sectorRelocation
		if err = rlp.DecodeBytes(op.Data, &relocate); err != nil {
			return nil, err
		}
		update.relocates = append(update.relocates, relocate)
	}
	update.txn = txn
	return
}
