package contractset

import (
	"bytes"
	"encoding/json"
	"github.com/DxChainNetwork/godx/common"
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

// StoreHeaderAndRoots will store both contract header and contract roots information into
// the contract set database
func (db *DB) StoreHeaderAndRoots(ch ContractHeader, roots []common.Hash) (err error) {
	// store contract header information
	if err = db.StoreContractHeader(ch); err != nil {
		return
	}

	// store contract roots information
	if err = db.StoreMerkleRoots(ch.ID, roots); err != nil {
		return
	}

	return
}

// FetchHeaderAndRoots will fetch both contract header and contract roots information from the
// contract set database
func (db *DB) FetchHeaderAndRoots(id storage.ContractID) (ch ContractHeader, roots []common.Hash, err error) {
	// fetch contract header information from the database
	if ch, err = db.FetchContractHeader(id); err != nil {
		return
	}

	// fetch contract roots information from the database
	if roots, err = db.FetchMerkleRoots(id); err != nil {
		return
	}

	return
}

// DeleteHeaderAndRoots will delete both contract header and contract roots information from the
// contract set database
func (db *DB) DeleteHeaderAndRoots(id storage.ContractID) (err error) {
	// delete contract header information from the database
	if err = db.DeleteContractHeader(id); err != nil {
		return
	}

	// delete contract roots information from the database
	if err = db.DeleteMerkleRoots(id); err != nil {
		return
	}

	return
}

// FetchAllContractID will fetch all contract ID stored in the contract set db
func (db *DB) FetchAllContractID() (ids []storage.ContractID) {
	iter := db.lvl.NewIterator(nil, nil)
	for iter.Next() {
		if bytes.HasSuffix(iter.Key(), []byte(dbMerkleRoot)) {
			continue
		}

		id, _ := splitKey(iter.Key())
		ids = append(ids, id)
	}

	return
}

// FetchAll will fetch all contract header and merkle roots information
func (db *DB) FetchAll() (chs []ContractHeader, allRoots [][]common.Hash, err error) {
	iter := db.lvl.NewIterator(nil, nil)
	for iter.Next() {
		var ch ContractHeader
		var roots []common.Hash

		// get the contract header value
		if bytes.HasSuffix(iter.Key(), []byte(dbContractHeader)) {
			if err = json.Unmarshal(iter.Value(), &ch); err != nil {
				return
			}

			// append the value into the list
			chs = append(chs, ch)
		}

		if bytes.HasSuffix(iter.Key(), []byte(dbMerkleRoot)) {
			if err = json.Unmarshal(iter.Value(), &roots); err != nil {
				return
			}
			allRoots = append(allRoots, roots)
		}
	}

	return
}

// FetchAllHeader will fetch all contract header information from the database
func (db *DB) FetchAllHeader() (chs []ContractHeader, err error) {
	iter := db.lvl.NewIterator(nil, nil)
	for iter.Next() {
		if !bytes.HasSuffix(iter.Key(), []byte(dbContractHeader)) {
			continue
		}

		// has suffix, contract header information
		var ch ContractHeader
		if err = json.Unmarshal(iter.Value(), &ch); err != nil {
			return
		}

		chs = append(chs, ch)
	}
	return
}

// FetchAllRoots will fetch all merkle roots information from the database
func (db *DB) FetchAllRoots() (allRoots [][]common.Hash, err error) {
	iter := db.lvl.NewIterator(nil, nil)
	for iter.Next() {
		if !bytes.HasSuffix(iter.Key(), []byte(dbMerkleRoot)) {
			continue
		}

		// has suffix, merkle roots information
		var roots []common.Hash
		if err = json.Unmarshal(iter.Value(), &roots); err != nil {
			return
		}

		allRoots = append(allRoots, roots)
	}

	return
}

// EmptyDB will clear all db entries
func (db *DB) EmptyDB() (err error) {
	iter := db.lvl.NewIterator(nil, nil)
	for iter.Next() {
		if err = db.lvl.Delete(iter.Key(), nil); err != nil {
			return
		}
	}

	return
}

// StoreContractHeader will stored the contract header information into the database
func (db *DB) StoreContractHeader(ch ContractHeader) (err error) {

	// get the header key and json encode the header
	headerKey, err := makeKey(ch.ID, dbContractHeader)
	if err != nil {
		return
	}
	blob, err := json.Marshal(ch)
	if err != nil {
		return
	}

	// store the header information
	if err = db.lvl.Put(headerKey, blob, nil); err != nil {
		return
	}

	return
}

// StoreMerkleRoots will store the contract roots information into the database
func (db *DB) StoreMerkleRoots(id storage.ContractID, roots []common.Hash) (err error) {
	// get the roots key and json encode contract roots
	rootsKey, err := makeKey(id, dbMerkleRoot)
	if err != nil {
		return
	}
	blob, err := json.Marshal(roots)
	if err != nil {
		return
	}

	// store the contract root information
	if err = db.lvl.Put(rootsKey, blob, nil); err != nil {
		return
	}
	return
}

// StoreSingleRoot will store one root into the database. If the contract id already
// have merkle roots stored in the database, append the new root into it. Otherwise
// store the root as a new root
func (db *DB) StoreSingleRoot(id storage.ContractID, root common.Hash) (err error) {
	// try to retrieve the roots first
	oldRoots, err := db.FetchMerkleRoots(id)
	if err != nil && err != errors.ErrNotFound {
		return
	} else if err == nil {
		// meaning there are roots saved using the contract id before
		oldRoots = append(oldRoots, root)
	} else {
		oldRoots = []common.Hash{root}
	}

	if err = db.StoreMerkleRoots(id, oldRoots); err != nil {
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

	// json decode the data
	if err = json.Unmarshal(blob, &ch); err != nil {
		return
	}

	return
}

// FetchMerkleRoots will retrieve the contract roots information based on the contract
// id provided
func (db *DB) FetchMerkleRoots(id storage.ContractID) (roots []common.Hash, err error) {
	// generate the key based on the storage contract ID
	key, err := makeKey(id, dbMerkleRoot)
	if err != nil {
		return
	}

	// try to get the information from the database
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return
	}

	// json decode the data
	if err = json.Unmarshal(blob, &roots); err != nil {
		return
	}
	return
}

// DeleteContractHeader will delete the contract header information
// from the contractsetdb, based on the contract id provided
func (db *DB) DeleteContractHeader(id storage.ContractID) (err error) {
	key, err := makeKey(id, dbContractHeader)
	if err != nil {
		return
	}

	return db.lvl.Delete(key, nil)
}

// DeleteMerkleRoots will delete the contract roots information
// from the contractsetdb, based on the contract id provided
func (db *DB) DeleteMerkleRoots(id storage.ContractID) (err error) {
	key, err := makeKey(id, dbMerkleRoot)
	if err != nil {
		return
	}
	return db.lvl.Delete(key, nil)
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

// splitKey will split the storage contract id and field
func splitKey(key []byte) (id storage.ContractID, field string) {
	copy(id[:], key[:len(id)])
	field = string(key[len(id):])

	return
}
