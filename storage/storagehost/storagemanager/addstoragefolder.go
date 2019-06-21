// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"

	"github.com/syndtr/goleveldb/leveldb"
)

type (
	// addStorageFolderUpdate is the structure used for add storage folder.
	addStorageFolderUpdate struct {
		path   string
		size   uint64
		txn    *writeaheadlog.Transaction
		batch  *leveldb.Batch
		folder *storageFolder
	}

	// addStorageFolderUpdatePersist is the structure of addStorageFolderUpdate
	// only used for RLP. The fields are set to public, and use this structure
	// only during the RLP encode and decode functions.
	addStorageFolderUpdatePersist struct {
		Path string
		Size uint64
	}
)

// AddStorageFolder add a storageFolder. The function could be called with a goroutine
func (sm *storageManager) AddStorageFolder(path string, size uint64) (err error) {
	// Change the folderPath to absolute path
	if path, err = absolutePath(path); err != nil {
		return
	}
	// Register in the thread manager
	if err = sm.tm.Add(); err != nil {
		return errStopped
	}
	defer sm.tm.Done()

	// validate the add storage folder
	if err = sm.validateAddStorageFolder(path, size); err != nil {
		return
	}
	// create the update and record the intent
	update := newAddStorageFolderUpdate(path, size)

	// record the update intent
	if err = update.recordIntent(sm); err != nil {
		err = fmt.Errorf("cannot record the intent for %v: %v", update.str(), err)
		return
	}
	// prepare, process, and release the update
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

// validateAddStorageFolder validate the add storage folder request. Return error if validation failed
func (sm *storageManager) validateAddStorageFolder(path string, size uint64) (err error) {
	sm.folders.lock.Lock()
	defer func() {
		if err != nil {
			sm.folders.lock.Unlock()
		}
	}()
	// Check numSectors
	numSectors := sizeToNumSectors(size)
	if numSectors < minSectorsPerFolder {
		err = fmt.Errorf("size too small")
		return
	}
	if numSectors > maxSectorsPerFolder {
		err = fmt.Errorf("size too large")
		return
	}
	// check whether the folder path already exists
	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		err = fmt.Errorf("folder already exists: %v", path)
		return
	}
	// check whether the folders has exceed limit
	if size := sm.folders.size(); size >= maxNumFolders {
		err = fmt.Errorf("too many folders to manager")
		return
	}
	// Check the existence of the folder in database
	exist, err := sm.db.hasStorageFolder(path)
	if err != nil {
		err = fmt.Errorf("check existence error: %v", err)
		return
	}
	if exist {
		err = fmt.Errorf("folder already exist in database")
		return
	}
	// Check the existence of the folder in folder manager
	if sm.folders.exist(path) {
		err = fmt.Errorf("folder already exist in memory")
		return
	}
	return nil
}

// newAddStorageFolderUpdate create a new addStorageFolderUpdate
// Note the size in the update is not the same as the input
func newAddStorageFolderUpdate(path string, size uint64) (update *addStorageFolderUpdate) {
	numSectors := sizeToNumSectors(size)
	update = &addStorageFolderUpdate{
		path: path,
		size: numSectors * storage.SectorSize,
	}
	return
}

// EncodeRLP defines the rlp rule of the addStorageFolderUpdate
func (update *addStorageFolderUpdate) EncodeRLP(w io.Writer) (err error) {
	pUpdate := addStorageFolderUpdatePersist{
		Path: update.path,
		Size: update.size,
	}
	return rlp.Encode(w, pUpdate)
}

// DecodeRLP defines the rlp decode rule of the addStorageFolderUpdate
func (update *addStorageFolderUpdate) DecodeRLP(st *rlp.Stream) (err error) {
	var pUpdate addStorageFolderUpdatePersist
	if err = st.Decode(&pUpdate); err != nil {
		return
	}
	update.path, update.size = pUpdate.Path, pUpdate.Size
	return nil
}

// str defines the user friendly string of the update
func (update *addStorageFolderUpdate) str() (s string) {
	// TODO: user friendly formatted print for size
	s = fmt.Sprintf("Add storage folder [%v] of %v byte", update.path, update.size)
	return
}

// recordIntent record the intent to wal to record the add folder intent
func (update *addStorageFolderUpdate) recordIntent(manager *storageManager) (err error) {
	manager.lock.RLock()
	defer func() {
		if err != nil {
			manager.lock.RUnlock()
		}
	}()

	data, err := rlp.EncodeToBytes(update)
	if err != nil {
		return
	}
	op := writeaheadlog.Operation{
		Name: opNameAddStorageFolder,
		Data: data,
	}
	if update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op}); err != nil {
		return
	}
	return
}

// prepare is the interface function defined by update, which prepares an update.
// For addStorageFolderUpdate, the prepare stage is defined by prepareNormal, prepareUncommitted,
// prepareUncommitted based on the target.
func (update *addStorageFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
	// In the prepare stage, the folder in the folder manager should be locked and removed.
	// Create the database batch, and write new data or delete data in the database
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("unknown target")
	}
	return
}

// process process the update with specified target
func (update *addStorageFolderUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("unknown target")
	}
	return
}

// release handle all errors and release the transaction
func (update *addStorageFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	// After all release operation completes, unlock the folder and the folder manager
	defer func() {
		manager.folders.lock.Unlock()
		if update.folder != nil {
			update.folder.status = folderAvailable
			update.folder.lock.Unlock()
		}
		manager.lock.RUnlock()
	}()

	// If no error happened during update, release the transaction and return
	if upErr == nil || upErr.isNil() {
		err = update.txn.Release()
		return
	}
	if upErr.hasErrStopped() {
		// If the storage manager has been stopped, simply do nothing and return
		// hand it to the next open
		upErr.processErr = nil
		upErr.prepareErr = nil
		return
	}
	// delete folder in memory
	// The folders is locked before prepare. So it shall be safe to delete the entry
	delete(manager.folders.sfs, update.path)
	// If update failed at update stage, revert the memory and commit release the transaction
	if upErr.prepareErr != nil {
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
	// Close the folder datafile
	if update.folder != nil {
		if newErr := update.folder.dataFile.Close(); newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
		// Delete the entry in database
		if newErr := manager.db.deleteStorageFolder(update.folder); newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}
	// If the processErr is os.ErrExist, which means that the file not exist during validation,
	// but during process, some other program (or user) created a file in the path, keep that
	// file, which might be useful to other programs. So delete the file only if the processErr
	// is not os.ErrExist
	if upErr.processErr != os.ErrExist {
		if newErr := os.Remove(filepath.Join(update.path, dataFileName)); newErr != nil {
			err = common.ErrCompose(err, newErr)
		}
	}

	// release the transaction
	if upErr.processErr != nil && update.txn != nil {
		err = common.ErrCompose(err, update.txn.Release())
	}
	return
}

// prepareNormal defines the prepare stage behaviour with the targetNormal.
// In this scenario,
// 1. lock the folders and check for whether the folder exist in folders. if exist, return error
// 2. a new folder should be written to the batch
// 3. Create a new locked folder and insert to the folder map
// 4. The underlying transaction is committed.
func (update *addStorageFolderUpdate) prepareNormal(manager *storageManager) (err error) {
	// construct the storage folder, lock the folder and register to manager.folders
	id, err := manager.db.randomFolderID()
	if err != nil {
		return fmt.Errorf("cannot create id: %v", err)
	}
	sf := &storageFolder{
		id:         id,
		status:     folderUnavailable,
		path:       update.path,
		usage:      emptyUsage(update.size),
		numSectors: sizeToNumSectors(update.size),
	}
	sf.lock.Lock()
	// For normal execution, the folders has already been locked. And the folder is also locked.
	if err = manager.folders.addFolder(sf); err != nil {
		err = fmt.Errorf("folder cannot register to storageManager: %v", err)
		return
	}
	update.folder = sf
	// Put the storageFolder in the batch
	update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, sf)
	if err != nil {
		return fmt.Errorf("cannot create save storage folder batch: %v", err)
	}
	if <-update.txn.InitComplete; update.txn.InitErr != nil {
		return fmt.Errorf("cannot initialize the trnasaction: %v", update.txn.InitErr)
	}
	return
}

// processNormal process the update as normal, which will
// 1. create the folder in location specified by path
// 2. Write the batch to db
// Note in this function, if file exist will return os.ErrExist, which should be handled in
// release
func (update *addStorageFolderUpdate) processNormal(manager *storageManager) (err error) {
	if err = <-update.txn.Commit(); err != nil {
		return fmt.Errorf("cannot commit the transaction: %v", err)
	}
	// check again whether the folder exists
	if _, err := os.Stat(filepath.Join(update.path)); !os.IsNotExist(err) {
		return os.ErrExist
	}
	// create the directory
	if err = os.MkdirAll(update.path, 0700); err != nil {
		return err
	}
	// create the data file
	update.folder.dataFile, err = os.Create(filepath.Join(update.path, dataFileName))
	if err != nil {
		return
	}
	// truncate the data file
	if err = update.folder.dataFile.Truncate(int64(update.size)); err != nil {
		return err
	}
	// write the batch to database
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	return
}

// decodeAddStorageFolderUpdate decode the transaction to the addStorageFolderUpdate
func decodeAddStorageFolderUpdate(txn *writeaheadlog.Transaction) (update *addStorageFolderUpdate, err error) {
	// only the data in the first operation is needed
	if len(txn.Operations) < 1 {
		return nil, fmt.Errorf("add storage folder transaction not having operation")
	}
	b := txn.Operations[0].Data
	if err = rlp.DecodeBytes(b, &update); err != nil {
		return
	}
	update.txn = txn
	return
}

// lockResource locks the resource during recover
func (update *addStorageFolderUpdate) lockResource(manager *storageManager) (err error) {
	manager.lock.RLock()
	// lock the folders until release
	manager.folders.lock.Lock()
	defer func() {
		if err != nil {
			manager.lock.RUnlock()
			manager.folders.lock.Unlock()
		}
	}()
	update.folder, err = manager.folders.get(update.path)
	if err != nil {
		return err
	}
	return nil
}

// prepareCommitted is the function called in prepare stage as preparing committed updates
func (update *addStorageFolderUpdate) prepareCommitted(manager *storageManager) (err error) {
	return
}

// processCommitted is the function called in process stage as processing committed an uncommitted updates
func (update *addStorageFolderUpdate) processCommitted(manager *storageManager) (err error) {
	// Do nothing, just return errRevert.
	return errRevert
}
