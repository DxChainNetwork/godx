// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"strconv"
	"strings"
)

type database struct {
	lvl *leveldb.DB
}

// openDB will create a new level db. If the db already existed,
// it will open the db instead
func openDB(path string) (db *database, err error) {
	if path == "" {
		err = errors.New("db persistDir cannot be empty")
	}
	return newPersistentDB(path)
}

// newPersistentDB open or create a new level db and change the type to database
func newPersistentDB(path string) (db *database, err error) {
	// open / create a new db
	lvl, err := leveldb.OpenFile(path, &opt.Options{})

	// if the db file already existed, check if the file is corrupted
	if _, isCorrupted := err.(*errors.ErrCorrupted); isCorrupted {
		lvl, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return
	}

	// initialize DB object
	db = &database{lvl}
	return
}

// close will close the level db, therefore, another process
// can open it again
func (db *database) close() {
	db.lvl.Close()
}

// newBatch create a new batch within the underlying level db
func (db *database) newBatch() *leveldb.Batch {
	return new(leveldb.Batch)
}

// writeBatch write the batch to the database
func (db *database) writeBatch(batch *leveldb.Batch) (err error) {
	err = db.lvl.Write(batch, nil)
	return
}

// getOrCreateSectorSalt return the sector salt and return.
// If previously the sector salt is not stored, create a new one and return
func (db *database) getOrCreateSectorSalt() (salt sectorSalt, err error) {
	key := makeKey(sectorSaltKey)
	var exist bool
	if exist, err = db.lvl.Has(key, nil); !exist || err != nil {
		// create a new random salt
		if _, err = rand.Read(salt[:]); err != nil {
			return
		}
		if err = db.lvl.Put(key, salt[:], nil); err != nil {
			return
		}
		return
	} else {
		var saltByte []byte
		saltByte, err = db.lvl.Get([]byte(sectorSaltKey), nil)
		if err != nil {
			return
		}
		copy(salt[:], saltByte)
		return
	}
}

// hasStorageFolder returns the result of whether the database has the key of a
// folder specified by a path
func (db *database) hasStorageFolder(path string) (exist bool, err error) {
	folderKey := makeKey(prefixFolder, path)
	exist, err = db.lvl.Has(folderKey, nil)
	return
}

// saveStorageFolder save the storage folder to the database.
// Note the storage folder should be locked before calling this function
func (db *database) saveStorageFolder(sf *storageFolder) (err error) {
	// make a new batch
	batch, err := db.saveStorageFolderBatch(sf)
	if err != nil {
		return err
	}
	if err = db.writeBatch(batch); err != nil {
		return
	}
	return
}

// saveStorageFolderBatch create the level db batch for save storage folder
// The storage folder should be locked before calling this function
func (db *database) saveStorageFolderBatch(sf *storageFolder) (batch *leveldb.Batch, err error) {
	// make a new batch
	batch = db.newBatch()
	// write folder data update to batch
	folderKey := makeKey(prefixFolder, sf.path)
	folderData, err := rlp.EncodeToBytes(sf)
	if err != nil {
		return nil, err
	}
	batch.Put(folderKey, folderData)
	// write id to path mapping to batch
	folderIDToPathKey := makeKey(prefixFolderIDToPath, strconv.FormatUint(uint64(sf.id), 10))
	batch.Put(folderIDToPathKey, []byte(sf.path))

	return batch, nil
}

// loadStorageFolder get the storage folder with the index from db
func (db *database) loadStorageFolder(path string) (sf *storageFolder, err error) {
	// make the folder key
	folderKey := makeKey(prefixFolder, path)
	folderBytes, err := db.lvl.Get(folderKey, nil)
	if err != nil {
		return
	}
	if err = rlp.DecodeBytes(folderBytes, &sf); err != nil {
		sf = nil
		return
	}
	return
}

// deleteStorageFolder delete the storage folder entry specified with the path
// WARN: this action will also remove all folder_sector map entries associated
// with the folder. Be sure that all sectors are placed safe before this
// function is called.
func (db *database) deleteStorageFolder(sf *storageFolder) (err error) {
	batch := new(leveldb.Batch)

	folderKey := makeKey(prefixFolder, sf.path)
	batch.Delete(folderKey)

	// when folder's id is 0, consider it as invalid. Skip delete the folderIDToPath
	if sf.id != 0 {
		folderIDToPathKey := makeKey(prefixFolderIDToPath, strconv.FormatUint(uint64(sf.id), 10))
		batch.Delete(folderIDToPathKey)
	}

	// Remove all entries in the iterator for folder to sector entries
	iter := db.lvl.NewIterator(util.BytesPrefix([]byte(makeKey(prefixFolderSector, sf.path))), nil)
	for iter.Next() {
		batch.Delete(iter.Key())
	}
	if err = db.writeBatch(batch); err != nil {
		return
	}
	return
}

// loadAllStorageFolders load all storage folders from database
func (db *database) loadAllStorageFolders() (folders map[string]*storageFolder, fullErr error) {
	folders = make(map[string]*storageFolder)
	// iterate over all entries start with the prefixFolder
	iter := db.lvl.NewIterator(util.BytesPrefix([]byte(prefixFolder+"_")), nil)
	for iter.Next() {
		// get the folder index from key
		key := string(iter.Key())
		path := strings.TrimPrefix(key, prefixFolder+"_")
		// get the folder content
		sfByte := iter.Value()
		var sf *storageFolder
		if err := rlp.DecodeBytes(sfByte, &sf); err != nil {
			// If error happened, log the error in return value and skip to next item
			fullErr = common.ErrCompose(fullErr, fmt.Errorf("cannot load folder %s: %v", key, err))
			continue
		}
		// Add the folder to map
		folders[path] = sf
	}
	return
}

// loadStorageFolderByID load the storage folder by id
func (db *database) loadStorageFolderByID(id folderID) (sf *storageFolder, err error) {
	folderIDKey := makeKey(prefixFolderIDToPath, strconv.FormatUint(uint64(id), 10))
	b, err := db.lvl.Get(folderIDKey, nil)
	if err != nil {
		return
	}
	path := string(b)
	sf, err = db.loadStorageFolder(path)
	return
}

// makeKey create the key. Add _ in each of the arguments
func makeKey(ss ...string) (key []byte) {
	if len(ss) == 0 {
		return
	}
	s := strings.Join(ss, "_")
	key = []byte(s)
	return
}

// randomFolderID create a random folder id that does not exist in database.
// After the function execution, the folderID is already stored in database to avoid other
// randomFolderID calls to use the same id
func (db *database) randomFolderID() (id folderID, err error) {
	b := make([]byte, 4)
	for i := 0; i != maxCreateFolderIDReties; i++ {
		rand.Read(b)
		id = folderID(binary.LittleEndian.Uint32(b))
		if id == 0 {
			continue
		}
		key := makeKey(prefixFolderIDToPath, strconv.FormatUint(uint64(id), 10))
		if exist, err := db.lvl.Has(key, nil); exist || err != nil {
			continue
		}
		// The key is ok to use
		err = db.lvl.Put(key, []byte{}, nil)
		if err != nil {
			// this key might be invalid. Continue to the next loop to find
			// another available key.
			continue
		}
		return
	}
	err = errors.New("create random folder id maximum retries reached.")
	return
}

// hasSector checks whether the sector is in the database
func (db *database) hasSector(id sectorID) (exist bool, err error) {
	key := makeKey(prefixSector, common.Bytes2Hex(id[:]))
	exist, err = db.lvl.Has(key, nil)
	return
}

// getSector get the sector from database with specified id.
// If the key does not exist in database, return ErrNotFound
func (db *database) getSector(id sectorID) (s *sector, err error) {
	key := makeKey(prefixSector, common.Bytes2Hex(id[:]))
	b, err := db.lvl.Get(key, nil)
	if err != nil {
		return
	}
	if err = rlp.DecodeBytes(b, &s); err != nil {
		return
	}
	return
}
