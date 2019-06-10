package newstoragemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/syndtr/goleveldb/leveldb"
	"io"
	"os"
)

type (
	// addStorageFolderUpdate is the structure used for add storage folder.
	addStorageFolderUpdate struct {
		path string
		size uint64
		txn  *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	// addStorageFolderUpdatePersist is the structure of addStorageFolderUpdate
	// only used for RLP. The fields are set to public, and use this structure
	// only during the RLP encode and decode functions.
	addStorageFolderUpdatePersist struct {
		Path string
		Size uint64
	}
)

// addStorageFolder add a storageFolder. The function could be called with a goroutine
func (sm *storageManager) addStorageFolder(path string, size uint64) (err error) {
	// Register in the thread manager
	if err = sm.tm.Add(); err != nil {
		return
	}
	defer sm.tm.Done()

	// validate the add storage folder
	if err = sm.validateAddStorageFolder(path, size); err != nil {
		return
	}
	// create the update and record the intent
	update := NewAddStorageFolderUpdate(path, size)

	// record the update intent
	if err = update.recordIntent(sm); err != nil {
		err = fmt.Errorf("cannot record the intent for %v: %v", update.str(), err)
	}
	return
}

// validateAddStorageFolder validate the add storage folder request. Return error if validation failed
func (sm *storageManager) validateAddStorageFolder(path string, size uint64) error {
	// Check numSectors
	numSectors := sizeToNumSectors(size)
	if numSectors < minSectorsPerFolder {
		return fmt.Errorf("size too small")
	}
	if numSectors > maxSectorsPerFolder {
		return fmt.Errorf("size too large")
	}
	// check whether the folder path already exists
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return fmt.Errorf("folder already exist: %v", path)
	}
	// check whether the folders has exceed limit
	if size := sm.folders.size(); size >= maxNumFolders {
		return fmt.Errorf("too many folders to manager")
	}
	return nil
}

// NewAddStorageFolderUpdate create a new addStorageFolderUpdate
// Note the size in the update is not the same as the input
func NewAddStorageFolderUpdate(path string, size uint64) (update *addStorageFolderUpdate) {
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
func (update *addStorageFolderUpdate) DecodeRLP(st *rlp.Stream) (err error){
	var pUpdate addStorageFolderUpdatePersist
	if err = st.Decode(&pUpdate); err != nil {
		return
	}
	update.path, update.size= pUpdate.Path, pUpdate.Size
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
// For addStorageFolderUpdate, the prepare stage is defined by prepareNormal, prepareCommitted,
// prepareUncommitted based on the target.
func (update *addStorageFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
	// In the prepare stage, the folder in the folder manager should be locked and removed.
	// Create the database batch, and write new data or delete data in the database
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
	case targetRecoverCommitted:
	case targetRecoverUncommitted:
	}
	return
}

// prepareNormal defines the prepare stage behaviour with the targetNormal.
// In this scenario,
// 1. a new folder should be written to the batch
// 2. Create a new locked folder and insert to the folder map
// 3. The underlying transaction is committed.
func (update *addStorageFolderUpdate) prepareNormal(manager *storageManager) (err error) {
	// Check the existence in the database
	exist, err := manager.db.hasStorageFolder(update.path)
	if err != nil {
		return fmt.Errorf("during check db existence for the folder: %v", err)
	}
	if exist {
		return fmt.Errorf("")
	}
	sf := &storageFolder{
		path: update.path,
		usage: EmptyUsage(update.size),
		numSectors: sizeToNumSectors(update.size),
		lock: common.NewTryLock(),
	}
	sf.lock.Lock()
}

func (update *addStorageFolderUpdate) process(manager *storageManager, target uint8) (err error) {
	return
}

func (update *addStorageFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	return
}
