// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
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

// makeKey create the key. Add _ in each of the arguments
func makeKey(ss ...string) (key []byte, err error) {
	if len(ss) == 0 {
		err = errors.New("key cannot be empty")
		return
	}
	s := strings.Join(ss, "_")
	key = []byte(s)
	return
}
