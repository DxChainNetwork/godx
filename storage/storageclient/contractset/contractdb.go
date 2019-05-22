package contractset

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type DB struct {
	lvl *leveldb.DB
}

// OpenDB will create a new level db. If the db already existed,
// it will open the db instead
func OpenDB(path string) (db *DB, err error) {
	if path == "" {
		err = errors.New("db persistDir cannot be empty")
	}
	return newPersistentDB(path)
}

// Close will close the level db, therefore, another process
// can open it again
func (db *DB) Close() {
	db.lvl.Close()
}

// StoreContractHeader will stored the contract header information into the database
func (db *DB) StoreContractHeader(ch ContractHeader) (err error) {

	// get the header key and rlp encode the header
	headerKey, err := makeKey(ch.ID, dbContractHeader)
	if err != nil {
		return
	}
	blob, err := rlp.EncodeToBytes(ch)
	if err != nil {
		return
	}

	// store the header information
	if err = db.lvl.Put(headerKey, blob, nil); err != nil {
		return
	}

	return
}

// StoreContractRoots will store the contract roots information into the database
func (db *DB) StoreContractRoots(id storage.ContractID, roots []common.Hash) (err error) {
	// get the roots key and rlp encode contract roots
	rootsKey, err := makeKey(id, dbContractRoot)
	if err != nil {
		return
	}
	blob, err := rlp.EncodeToBytes(roots)
	if err != nil {
		return
	}

	// store the contract root information
	if err = db.lvl.Put(rootsKey, blob, nil); err != nil {
		return
	}
	return
}

// FetchContractHeader will retrieve the contract header information based on the
// contract id provided
func (db *DB) FetchContractHeader(id storage.ContractID) (ch ContractHeader, err error) {
	// generate the key based on the storage contract ID
	key, err := makeKey(id, dbContractHeader)
	if err != nil {
		return
	}

	// try to get the information from the database
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return
	}

	// rlp decode the data
	if err = rlp.DecodeBytes(blob, &ch); err != nil {
		return
	}

	return
}

// FetchContractRoots will retrieve the contract roots information based on the contract
// id provided
func (db *DB) FetchContractRoots(id storage.ContractID) (roots []common.Hash, err error) {
	// generate the key based on the storage contract ID
	key, err := makeKey(id, dbContractRoot)
	if err != nil {
		return
	}

	// try to get the information from the database
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return
	}

	// rlp decode the data
	if err = rlp.DecodeBytes(blob, &roots); err != nil {
		return
	}
	return
}

// newPersistentDB will initialize a new DB object which is used
// to store storage contract information
func newPersistentDB(path string) (db *DB, err error) {

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
	db = &DB{
		lvl: lvl,
	}
	return
}

// makeKey will generate a DB key for different type of entries
func makeKey(id storage.ContractID, field string) (key []byte, err error) {
	// validate the contract id
	if len(id) == 0 {
		err = errors.New("the storage contract id cannot be empty")
		return
	}

	// make the key
	key = append(key, append(id[:], field...)...)
	return
}
