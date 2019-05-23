// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/storage"
)

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
	c.headerLock.Unlock()

	c.lock.Lock()
	rootCount := c.merkleRoots.len()
	c.lock.Unlock()

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

// RecordUploadPreRev will record pre-revision contract revision after file download operation,
// which is not stored in the database. once negotiation has completed, CommitUpload
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

	// TODO (mzhang): double check if the insert method is really needed
	if err = c.merkleRoots.push(contractHeader.ID, root); err != nil {
		return fmt.Errorf("failed to save the root into db while commit upload: %s", err.Error())
	}

	if err = t.Release(); err != nil {
		return
	}

	// clear all un-applied transactions
	// TODO (mzhang): should I only remove only upload transaction from the list, not all of them?
	// This remove include both upload and download
	c.lock.Lock()
	c.unappliedTxns = nil
	c.lock.Unlock()

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

	// clear all un-applied transactions
	// TODO (mzhang): should I only remove only download transaction from the list, not all of them?
	// This remove include both upload and download
	c.unappliedTxns = nil

	return
}

// CommitTxns will apply all un-applied transactions
func (c *Contract) CommitTxns() (err error) {
	// TODO (mzhang): this is not thread safe
	// loop through all un-applied transactions
	for _, t := range c.unappliedTxns {
		for _, op := range t.Operations {
			switch op.Name {
			// update contract header
			case dbContractHeader:
				var walHeader walContractHeaderEntry
				if err = rlp.DecodeBytes(op.Data, &walHeader); err != nil {
					return
				}
				if err = c.contractHeaderUpdate(walHeader.Header); err != nil {
					return
				}
			case dbMerkleRoot:
				var walRoot walRootsEntry
				if err = rlp.DecodeBytes(op.Data, &walRoot); err != nil {
					return
				}
				if err = c.merkleRoots.push(walRoot.ID, walRoot.Root); err != nil {
					return
				}
			}
		}
		if err = t.Release(); err != nil {
			return
		}
	}

	// empty all un-applied transactions
	// TODO (mzhang): should I only remove the commited transactions instead of all transactions?
	c.unappliedTxns = nil
	return
}

// RevisionValidation will validate the storage host revision passed in, check if
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
	c.lock.Lock()
	defer c.lock.Unlock()
	meta = storage.ContractMetaData{
		ID:                     c.header.ID,
		EnodeID:                c.header.EnodeID,
		LatestContractRevision: c.header.LatestContractRevision,
		StartHeight:            c.header.StartHeight,

		// TODO (mzhang): add EndHeight calculation

		// TODO (mzhang): add ClientBalance calculation

		UploadCost:   c.header.UploadCost,
		DownloadCost: c.header.DownloadCost,
		StorageCost:  c.header.StorageCost,
		TotalCost:    c.header.TotalCost,
		GasFee:       c.header.GasFee,
		ContractFee:  c.header.ContractFee,
		Status:       c.header.Status,
	}
	return
}

// Status will return the current status of the contract
func (c *Contract) Status() (stats storage.ContractStatus) {
	c.headerLock.Lock()
	defer c.headerLock.Unlock()

	return c.header.Status
}

// contractHeaderUpdate will update contract header information in both db and mermory
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

func (c *Contract) contractHeaderWalOP(ch ContractHeader) (op writeaheadlog.Operation, err error) {
	// get the contract id
	c.headerLock.Lock()
	contractID := c.header.ID
	c.headerLock.Unlock()

	// rlp encode the data that is going to be saved in the wal
	data, err := rlp.EncodeToBytes(walContractHeaderEntry{
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

func (c *Contract) merkleRootWalOP(root common.Hash, rootCount int) (op writeaheadlog.Operation, err error) {
	// retrieve contract id
	c.headerLock.Lock()
	contractID := c.header.ID
	c.headerLock.Unlock()

	// rlp encode the data that is going to be saved in the wal
	data, err := rlp.EncodeToBytes(walRootsEntry{
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
