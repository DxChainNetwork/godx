// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	dberrors "github.com/syndtr/goleveldb/leveldb/errors"
)

// StorageContractSet is used to record all contract signed by the storage client
type StorageContractSet struct {
	contracts        map[storage.ContractID]*Contract
	hostToContractID map[enode.ID]storage.ContractID
	persistDir       string
	db               *DB
	lock             sync.Mutex
	rl               *RateLimit
	wal              *writeaheadlog.Wal
}

// New will initialize the StorageContractSet object, as well as
// loading the data that already stored in the database
func New(persistDir string) (scs *StorageContractSet, err error) {
	// initialize the directory
	if err = os.MkdirAll(persistDir, 0700); err != nil {
		err = fmt.Errorf("error initializing directory: %s", err.Error())
		return
	}

	// initialize DB
	db, err := OpenDB(filepath.Join(persistDir, persistDBName))
	if err != nil {
		err = fmt.Errorf("error initializing database: %s", err.Error())
		return
	}

	// initialize wal
	wal, walTxns, err := writeaheadlog.New(filepath.Join(persistDir, persistWalName))
	if err != nil {
		err = fmt.Errorf("error initializing wal: %s", err.Error())
		return
	}

	scs = &StorageContractSet{
		contracts:        make(map[storage.ContractID]*Contract),
		hostToContractID: make(map[enode.ID]storage.ContractID),
		persistDir:       persistDir,
		db:               db,
		wal:              wal,
	}

	// initialize rateLimit object
	scs.rl = NewRateLimit(0, 0, 0)

	// load the contracts from the database
	if err = scs.loadContract(walTxns); err != nil {
		err = fmt.Errorf("error loading contracts from the database: %s", err.Error())
		return
	}

	return
}

// Close will close the database and closing the writeaheadlog
func (scs *StorageContractSet) Close() (err error) {
	scs.db.Close()
	_, err = scs.wal.CloseIncomplete()
	return
}

// EmptyDB will clear all data stored in the contractset database
func (scs *StorageContractSet) EmptyDB() (err error) {
	err = scs.db.EmptyDB()
	return
}

// InsertContract will insert the formed or renewed contract into the storage contract set
// allow contract manager further to maintain them
func (scs *StorageContractSet) InsertContract(ch ContractHeader, roots []common.Hash) (cm storage.ContractMetaData, err error) {
	// contract header validation
	if err = ch.validation(); err != nil {
		return
	}

	// save the contract header and roots information
	if err = scs.db.StoreContractHeader(ch); err != nil {
		err = fmt.Errorf("failed to store contract header information into database: %s",
			err.Error())
		return
	}

	// add the root to db as well as memory merkle tree
	merkleRoots := newMerkleRoots(scs.db, ch.ID)
	for _, root := range roots {
		if err = merkleRoots.push(root); err != nil {
			return
		}
	}

	// initialize contract
	c := &Contract{
		header:      ch,
		merkleRoots: merkleRoots,
		db:          scs.db,
		wal:         scs.wal,
	}

	// get the contract meta data
	cm = c.Metadata()

	// save the contract into the contract set
	scs.lock.Lock()
	defer scs.lock.Unlock()
	scs.contracts[c.header.ID] = c
	scs.hostToContractID[c.header.EnodeID] = c.header.ID

	return
}

// Acquire will acquire the contract from the contractSet, the contract acquired from the
// contract set will be locked. Once acquired, the contract must be returned to unlock it.
func (scs *StorageContractSet) Acquire(id storage.ContractID) (c *Contract, exists bool) {
	scs.lock.Lock()
	c, exists = scs.contracts[id]
	scs.lock.Unlock()

	if !exists {
		return
	}

	// lock the contract
	c.lock.Lock()
	return
}

// Return will unlock the contract acquired from the contractSet using the acquire function
// the contract must be acquired using the Acquire function first before using this function
func (scs *StorageContractSet) Return(c *Contract) (err error) {
	scs.lock.Lock()
	defer scs.lock.Unlock()

	_, exists := scs.contracts[c.header.ID]

	if !exists {
		err = errors.New("the contract does not exist while returning the contract")
	}
	c.lock.Unlock()
	return
}

// Delete will delete a storage contract from the contract set
// the contract must be acquired using the Acquire function
func (scs *StorageContractSet) Delete(c *Contract) (err error) {
	scs.lock.Lock()
	defer scs.lock.Unlock()
	// check if the contract existed in the contract set
	_, exists := scs.contracts[c.header.ID]

	if !exists {
		err = errors.New("the contract does not exist while deleting the contract")
		return
	}

	// delete memory contract information
	delete(scs.contracts, c.header.ID)

	c.lock.Unlock()

	// delete disk contract information
	if err = scs.db.DeleteHeaderAndRoots(c.header.ID); err != nil {
		return
	}

	return
}

// IDs return a list of storage contract id stored in the contract set
func (scs *StorageContractSet) IDs() (ids []storage.ContractID) {
	scs.lock.Lock()
	defer scs.lock.Unlock()
	for id := range scs.contracts {
		ids = append(ids, id)
	}
	return
}

// SetRateLimit will set the rate limit based on the value user provided
func (scs *StorageContractSet) SetRateLimit(readBPS, writeBPS int64, packetSize uint64) {
	scs.rl.SetRateLimit(readBPS, writeBPS, packetSize)
}

// RetrieveRateLimit will retrieve the current rate limit settings
func (scs *StorageContractSet) RetrieveRateLimit() (readBPS, writeBPS int64, packetSize uint64) {
	return scs.rl.RetrieveRateLimit()
}

// RetrieveContractMetaData will return ContractMetaData based on the contract id provided
func (scs *StorageContractSet) RetrieveContractMetaData(id storage.ContractID) (cm storage.ContractMetaData, exist bool) {
	scs.lock.Lock()
	defer scs.lock.Unlock()

	contract, exist := scs.contracts[id]
	if !exist {
		return
	}

	cm = contract.Metadata()
	return
}

// RetrieveAllContractsMetaData will return all ContractMetaData stored in the contract set
// in the form of list
func (scs *StorageContractSet) RetrieveAllContractsMetaData() (cms []storage.ContractMetaData) {
	scs.lock.Lock()
	defer scs.lock.Unlock()

	// get metadata from all contract stored in the contracts
	for _, contract := range scs.contracts {
		cms = append(cms, contract.Metadata())
	}

	return
}

// loadContract will load contracts information from the database, it will also
// filter out the un-applied transaction for the particular contract
func (scs *StorageContractSet) loadContract(walTxns []*writeaheadlog.Transaction) (err error) {
	// get all the contract id
	ids := scs.db.FetchAllContractID()

	// iterate through all contract id
	var ch ContractHeader
	var roots []common.Hash

	for _, id := range ids {
		// get the contract header based on the contract id
		if ch, err = scs.db.FetchContractHeader(id); err != nil {
			return
		}

		// get the merkle roots based on the contract id
		if roots, err = scs.db.FetchMerkleRoots(id); err != nil && err != dberrors.ErrNotFound {
			return
		}

		// load merkle roots
		mr, err := loadMerkleRoots(scs.db, id, roots)
		if err != nil {
			return fmt.Errorf("failed to load merkle roots, load contract failed: %s", err.Error())
		}

		// TODO (mzhang): currently, un-applied WAL transaction will be ignored
		// in the future, they should be handled, however, the negotiation process
		// needs to be modified

		// initialize contract
		c := &Contract{
			header:      ch,
			merkleRoots: mr,
			db:          scs.db,
			wal:         scs.wal,
		}

		// update contract set
		scs.contracts[id] = c
		scs.hostToContractID[c.header.EnodeID] = c.header.ID

	}

	err = nil
	return
}

// Contracts is used to get all active contracts signed by the storage client
func (scs *StorageContractSet) Contracts() map[storage.ContractID]*Contract {
	scs.lock.Lock()
	defer scs.lock.Unlock()
	return scs.contracts
}

// GetContractIDByHostID will retrieve the contractID based on the hostID provided
func (scs *StorageContractSet) GetContractIDByHostID(hostID enode.ID) (storage.ContractID, bool) {
	scs.lock.Lock()
	defer scs.lock.Unlock()
	id, exist := scs.hostToContractID[hostID]
	return id, exist
}
