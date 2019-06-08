// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"crypto/rand"
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

// saveStorageFolder save the storage folder to the database.
// Note the storage folder should be locked before calling this function
func (db *database) saveStorageFolder(sf *storageFolder) (err error) {
	// make key-value pair
	folderIndex := int(sf.id)
	folderKey := makeKey(prefixFolder, strconv.Itoa(folderIndex))
	folderData, err := rlp.EncodeToBytes(sf)
	if err != nil {
		return err
	}
	// save
	err = db.lvl.Put(folderKey, folderData, nil)
	return
}

// loadStorageFolder get the storage folder with the index from db
func (db *database) loadStorageFolder(index folderID) (sf *storageFolder, err error) {
	// make the folder key
	folderKey := makeKey(prefixFolder, strconv.Itoa(int(index)))
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

// loadAllStorageFolders load all storage folders from database
func (db *database) loadAllStorageFolders() (folders map[folderID]*storageFolder, fullErr error) {
	folders = make(map[folderID]*storageFolder)
	// iterate over all entries start with the prefixFolder
	iter := db.lvl.NewIterator(util.BytesPrefix([]byte(prefixFolder+"_")), nil)
	for iter.Next() {
		// get the folder index from key
		key := string(iter.Key())
		folderIndexStr := strings.TrimPrefix(key, prefixFolder+"_")
		// get the folder content
		sfByte := iter.Value()
		var sf *storageFolder
		if err := rlp.DecodeBytes(sfByte, &sf); err != nil {
			// If error happened, log the error in return value and skip to next item
			fullErr = common.ErrCompose(fullErr, fmt.Errorf("cannot load folder %s: %v", key, err))
			continue
		}
		id, err := strconv.Atoi(folderIndexStr)
		if err != nil {
			fullErr = common.ErrCompose(fullErr, fmt.Errorf("cannot load folder %s: %v", key, err))
			continue
		}
		sf.id = folderID(id)
		// Add the folder to map
		folders[folderID(id)] = sf
	}
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
