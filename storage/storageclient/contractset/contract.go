// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"github.com/DxChainNetwork/godx/storage"
	"sync"

	"github.com/DxChainNetwork/godx/common/writeaheadlog"
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

	// whenever changes are going to be made on a contract
	// the changes will be saved in this field first
	// in case power-off is encountered. Prevent data lose
	unappliedTxns []*writeaheadlog.Transaction

	db   *DB
	wal  *writeaheadlog.Wal
	lock sync.Mutex
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
