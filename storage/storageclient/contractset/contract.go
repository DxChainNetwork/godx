// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
)

// Contract is a data structure that stored the contract information
type Contract struct {
	headerLock    sync.Mutex
	header        ContractHeader
	merkleRoots   *merkleRoots
	unappliedTxns []*writeaheadlog.Transaction

	db   *DB
	lock sync.Mutex
	wal  *writeaheadlog.Wal
}

type walContractHeaderEntry struct {
	ID     storage.ContractID
	Header ContractHeader
}

type walRootsEntry struct {
	ID    storage.ContractID
	Root  common.Hash
	Index uint64
}

// Status will return the current status of the contract
func (c *Contract) Status() (stats storage.ContractStatus) {
	c.headerLock.Lock()
	defer c.headerLock.Unlock()

	return c.header.Status
}

// UpdateStatus will update the current contract status
func (c *Contract) UpdateStatus(status storage.ContractStatus) (err error) {
	// get the contract header
	c.headerLock.Lock()
	contractHeader := c.header
	c.headerLock.Unlock()

	// update the status field
	contractHeader.Status = status

	err = c.contractHeaderUpdate(contractHeader)

	return
}

// RecordUploadPreRev will record pre-revision contract revision, which
// is not stored in the database. once negotiation has completed, CommitUpload
// will be called to record the actual contract revision and store it into database
func (c *Contract) RecordUploadPreRev(rev types.StorageContractRevision, root common.Hash, storageCost, bandwidthCost common.BigInt) (t *writeaheadlog.Transaction, err error) {
	// get the contract header from the memory
	c.headerLock.Lock()
	contractHeader := c.header
	rootCount := c.merkleRoots.len()
	c.headerLock.Unlock()

	// update header information
	contractHeader.LatestContractRevision = rev
	contractHeader.UploadCost = contractHeader.UploadCost.Add(bandwidthCost)
	contractHeader.StorageCost = contractHeader.StorageCost.Add(storageCost)

	// make header and root update wal operations
	chOp, err := c.contractHeaderWalOP(contractHeader)
	if err != nil {
		return
	}

	rtOp, err := c.merkleRootWalOP(root, rootCount)
	if err != nil {
		return
	}

	t, err = c.wal.NewTransaction([]writeaheadlog.Operation{
		chOp,
		rtOp,
	})

	if err != nil {
		return
	}

	if err = <-t.Commit(); err != nil {
		return
	}

	// append the transaction into the un-applied transactions
	c.unappliedTxns = append(c.unappliedTxns, t)

	return
}

// RecordDownloadPreRev will record pre-revision contract revision after file download operation,
// which is not stored in the database. once negotiation has completed, CommitDownload
// will be called to record the actual contract revision and store it into database
func (c *Contract) RecordDownloadPreRev(rev types.StorageContractRevision, bandwidthCost common.BigInt) (t *writeaheadlog.Transaction, err error) {
	// get the storage contract header information from the memory
	c.headerLock.Lock()
	contractHeader := c.header
	c.headerLock.Unlock()

	// update the contract header information
	contractHeader.LatestContractRevision = rev
	contractHeader.DownloadCost = contractHeader.DownloadCost.Add(bandwidthCost)

	// make header wal transaction
	chOp, err := c.contractHeaderWalOP(contractHeader)
	if err != nil {
		return
	}

	t, err = c.wal.NewTransaction([]writeaheadlog.Operation{
		chOp,
	})

	if err != nil {
		return
	}

	if err = <-t.Commit(); err != nil {
		return
	}

	// append the transaction into the un-applied transactions
	c.unappliedTxns = append(c.unappliedTxns, t)

	return
}

// CommitUpload will update the contract header and merkle root information after each file upload
// information will be stored in both db and memory
func (c *Contract) CommitUpload(t *writeaheadlog.Transaction, signedRev types.StorageContractRevision, root common.Hash, storageCost, bandwidthCost common.BigInt) (err error) {
	// get the contract header information
	c.headerLock.Lock()
	contractHeader := c.header
	c.headerLock.Unlock()

	// update the contract
	contractHeader.LatestContractRevision = signedRev
	contractHeader.StorageCost = contractHeader.StorageCost.Add(storageCost)
	contractHeader.UploadCost = contractHeader.UploadCost.Add(bandwidthCost)

	if err = c.contractHeaderUpdate(contractHeader); err != nil {
		return fmt.Errorf("during the upload commiting, %s", err.Error())
	}

	if err = c.merkleRoots.push(root); err != nil {
		return fmt.Errorf("failed to save the root into db while commit upload: %s", err.Error())
	}

	if err = t.Release(); err != nil {
		return
	}

	// remove the transaction that committed
	for i, txn := range c.unappliedTxns {
		if txn == t {
			c.unappliedTxns = append(c.unappliedTxns[:i], c.unappliedTxns[i+1:]...)
			break
		}
	}

	return
}

// CommitDownload will update the contract header information after each file download
// information will be stored in both db and memory
func (c *Contract) CommitDownload(t *writeaheadlog.Transaction, signedRev types.StorageContractRevision, bandwidth common.BigInt) (err error) {
	// get the contract header information
	c.headerLock.Lock()
	contractHeader := c.header
	c.headerLock.Unlock()

	// update the contract
	contractHeader.LatestContractRevision = signedRev
	contractHeader.DownloadCost = contractHeader.DownloadCost.Add(bandwidth)

	if err = c.contractHeaderUpdate(contractHeader); err != nil {
		return fmt.Errorf("during the download commiting, %s", err.Error())
	}

	if err = t.Release(); err != nil {
		return
	}

	// remove the transaction that committed
	for i, txn := range c.unappliedTxns {
		if txn == t {
			c.unappliedTxns = append(c.unappliedTxns[:i], c.unappliedTxns[i+1:]...)
			break
		}
	}

	return
}

// CommitTxns will apply all un-applied transactions
func (c *Contract) CommitTxns() (err error) {
	// loop through all un-applied transactions
	for _, t := range c.unappliedTxns {
		for _, op := range t.Operations {
			switch op.Name {
			// update contract header
			case dbContractHeader:
				var walHeader walContractHeaderEntry
				if err = json.Unmarshal(op.Data, &walHeader); err != nil {
					return
				}
				if err = c.contractHeaderUpdate(walHeader.Header); err != nil {
					return
				}
			case dbMerkleRoot:
				var walRoot walRootsEntry
				if err = json.Unmarshal(op.Data, &walRoot); err != nil {
					return
				}
				if err = c.merkleRoots.push(walRoot.Root); err != nil {
					return
				}
			}
		}
		if err = t.Release(); err != nil {
			return
		}
	}

	// empty all un-applied transactions
	c.unappliedTxns = nil
	return
}

// LatestRevisionValidation will validate the storage host revision passed in, check if
// they have the same latest revision
func (c *Contract) LatestRevisionValidation(rev types.StorageContractRevision) (err error) {
	myRev := c.header.LatestContractRevision

	// verify the unlock hash between two contract revisions
	if rev.UnlockConditions.UnlockHash() != myRev.UnlockConditions.UnlockHash() {
		return errors.New("unlock conditions do not match")
	}

	// compare the revision number between two contract revisions
	if rev.NewRevisionNumber != myRev.NewRevisionNumber {
		// if contract revision number are not matching, apply un-applied transactions first
		if err = c.CommitTxns(); err != nil {
			return
		}
		myRev = c.header.LatestContractRevision
		if rev.NewRevisionNumber != myRev.NewRevisionNumber {
			return fmt.Errorf("contract revision number %v does not match with storage host's contract revision number %v",
				myRev.NewRevisionNumber, rev.NewRevisionNumber)
		}
	}
	return
}

// Metadata will generate contract meta data based on the contract information
func (c *Contract) Metadata() (meta storage.ContractMetaData) {
	c.headerLock.Lock()
	defer c.headerLock.Unlock()

	meta = storage.ContractMetaData{
		ID:                     c.header.ID,
		EnodeID:                c.header.EnodeID,
		LatestContractRevision: c.header.LatestContractRevision,
		StartHeight:            c.header.StartHeight,

		EndHeight:       c.header.LatestContractRevision.NewWindowStart,
		ContractBalance: common.PtrBigInt(c.header.LatestContractRevision.NewValidProofOutputs[0].Value),

		UploadCost:   c.header.UploadCost,
		DownloadCost: c.header.DownloadCost,
		StorageCost:  c.header.StorageCost,
		TotalCost:    c.header.TotalCost,
		GasCost:      c.header.GasFee,
		ContractFee:  c.header.ContractFee,
		Status:       c.header.Status,
	}
	return
}

// contractHeaderUpdate will update contract header information in both db and memory
func (c *Contract) contractHeaderUpdate(newHeader ContractHeader) (err error) {
	// update db, store will update the entry with the same key
	if err = c.db.StoreContractHeader(newHeader); err != nil {
		return fmt.Errorf("failed to save the contract header update into db: %s", err.Error())
	}

	// update the contract in memory
	c.headerLock.Lock()
	c.header = newHeader
	c.headerLock.Unlock()

	return
}

// contractHeaderWalOp will create and initialize the contract header write ahead log operation
func (c *Contract) contractHeaderWalOP(ch ContractHeader) (op writeaheadlog.Operation, err error) {
	// get the contract id
	c.headerLock.Lock()
	contractID := c.header.ID
	c.headerLock.Unlock()

	// json encode the data that is going to be saved in the wal
	data, err := json.Marshal(walContractHeaderEntry{
		ID:     contractID,
		Header: ch,
	})

	if err != nil {
		err = fmt.Errorf("failed to encode the contract header entry, the operation was not recorded: %s",
			err.Error())
		return
	}

	// create writeaheadlog operation
	op = writeaheadlog.Operation{
		Name: dbContractHeader,
		Data: data,
	}

	return
}

// merkleRootWalOp will create and initialize the merkle root write ahead log operation
func (c *Contract) merkleRootWalOP(root common.Hash, rootCount int) (op writeaheadlog.Operation, err error) {
	// retrieve contract id
	c.headerLock.Lock()
	contractID := c.header.ID
	c.headerLock.Unlock()

	// json encode the data that is going to be saved in the wal
	data, err := json.Marshal(walRootsEntry{
		ID:    contractID,
		Index: uint64(rootCount),
		Root:  root,
	})

	if err != nil {
		err = fmt.Errorf("failed to enocde the merkle root entry, the oepration was not recodered: %s",
			err.Error())
		return
	}

	// create writeaheadlog operation
	op = writeaheadlog.Operation{
		Name: dbMerkleRoot,
		Data: data,
	}

	return
}

// Header will return the contract header information of the contract
func (c *Contract) Header() ContractHeader {
	return c.header
}

// MerkleRoots will return the merkle roots information of the contract
func (c *Contract) MerkleRoots() ([]common.Hash, error) {
	return c.merkleRoots.roots()
}
