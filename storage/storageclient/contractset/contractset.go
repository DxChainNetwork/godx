// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"os"
	"path/filepath"
	"sync"
)

// ************************************************************************
//                             MOCKED DATA
// ************************************************************************

type RateLimit struct{}

// ************************************************************************
// ************************************************************************

type StorageContractSet struct {
	contracts        map[storage.ContractID]*Contract
	hostToContractID map[enode.ID]storage.ContractID
	persistDir       string
	db               *DB
	lock             sync.Mutex
	rl               *RateLimit
}

func New(persistDir string) (scs *StorageContractSet, err error) {
	// initialize the directory
	if err = os.MkdirAll(persistDir, 0700); err != nil {
		return
	}

	// initialize DB
	// TODO (mzhang): remember to close the database, db should be closed in upper function call
	db, err := OpenDB(filepath.Join(persistDir, persistDBName))
	if err != nil {
		return
	}

	scs = &StorageContractSet{
		contracts:        make(map[storage.ContractID]*Contract),
		hostToContractID: make(map[enode.ID]storage.ContractID),
		persistDir:       persistDir,
		db:               db,
	}

	// TODO (mzhang): Set rate limit

	// load the contracts from the database
	if err = scs.loadContract(); err != nil {
		return
	}

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
	merkleRoots := newMerkleRoots(scs.db)
	for _, root := range roots {
		if err = merkleRoots.push(ch.ID, root); err != nil {
			return
		}
	}

	// initialize contract
	c := &Contract{
		header:      ch,
		merkleRoots: merkleRoots,
		db:          scs.db,
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
	defer scs.lock.Unlock()

	c, exists = scs.contracts[id]
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
	delete(scs.hostToContractID, c.header.EnodeID)

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

// RetrieveMetaData will return ContractMetaData based on the contract id provided
func (scs *StorageContractSet) RetrieveMetaData(id storage.ContractID) (cm storage.ContractMetaData, exist bool) {
	scs.lock.Lock()
	defer scs.lock.Unlock()

	contract, exist := scs.contracts[id]
	if !exist {
		return
	}

	cm = contract.Metadata()
	return
}

// RetrieveAllMetaData will return all ContractMetaData stored in the contract set
// in the form of list
func (scs *StorageContractSet) RetrieveAllMetaData() (cms []storage.ContractMetaData) {
	scs.lock.Lock()
	defer scs.lock.Unlock()

	// get metadata from all contract stored in the contracts
	for _, contract := range scs.contracts {
		cms = append(cms, contract.Metadata())
	}

	return
}

// loadContract will load contracts information from the database
// as well as applying un-applied transactions read from the writeaheadlog file
func (scs *StorageContractSet) loadContract() (err error) {
	// get all the contract id
	ids := scs.db.FetchAllContractID()

	// iterate through all contract id
	var ch ContractHeader
	var roots []common.Hash

	for _, id := range ids {
		if ch, err = scs.db.FetchContractHeader(id); err != nil {
			return
		}

		if roots, err = scs.db.FetchMerkleRoots(id); err != nil {
			return
		}

		// load merkle roots
		mr := loadMerkleRoots(scs.db, roots)

		// initialize contract
		c := &Contract{
			header:      ch,
			merkleRoots: mr,
			db:          scs.db,
		}

		// update contract set
		scs.contracts[id] = c
		scs.hostToContractID[c.header.EnodeID] = c.header.ID

		return
	}

	return
}