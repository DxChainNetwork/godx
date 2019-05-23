// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"sync"
)

// ************************************************************************
//                             MOCKED DATA
// ************************************************************************
type FileContractRevision struct{}

// ************************************************************************
// ************************************************************************

type Contract struct {
	headerLock  sync.Mutex
	header      ContractHeader
	merkleRoots *merkleRoots

	db   *DB
	lock sync.Mutex
}

func (c *Contract) UpdateStatus(status storage.ContractStatus) (err error) {
	// get the contract header
	c.headerLock.Lock()
	contractHeader := c.header
	c.headerLock.Unlock()

	// update the status field
	contractHeader.Status = status

	// update db
	// TODO (mzhang): implement db update methods, instead of storing it directly
	if err = c.db.StoreContractHeader(contractHeader); err != nil {
		return fmt.Errorf("failed to save the status into db: %s", err.Error())
	}

	// update the contract
	c.headerLock.Lock()
	c.header = contractHeader
	c.headerLock.Unlock()

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
