// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"math/big"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	// PersistHostDir is dir path for storing the host log, json, and etc.
	PersistHostDir = "storagehost"
	// Version is the version of the storage host
	Version = "1.0"
	// HostSettingFile is the file name for saving the setting of host
	HostSettingFile = "host.json"
	// HostDB is the database dir for storing host obligation
	databaseFile = "hostdb"
	// StorageManager is a dir for storagemanager related topic
	StorageManager = "storagemanager"
)

const (
	// storage responsibility related constants
	postponedExecution    = 3  //Total length of time to start a test task
	confirmedBufferHeight = 40 //signing transaction not confirmed maximum time

	//prefixStorageResponsibility db prefix for StorageResponsibility
	prefixStorageResponsibility = "StorageResponsibility-"
	//prefixHeight db prefix for task
	prefixHeight = "height-"
)

var (
	// sectorHeight is the parameter used in caching merkle roots
	sectorHeight uint64

	storageHostMeta = common.Metadata{
		Header:  "DxChain StorageHost JSON",
		Version: "V1.0",
	}

	// persistence default value
	defaultMaxDuration          = storage.BlocksPerDay * 30 // 30 days
	defaultMaxDownloadBatchSize = 17 * (1 << 20)            // 17 MB
	defaultMaxReviseBatchSize   = 17 * (1 << 20)            // 17 MB
	defaultWindowSize           = 5 * storage.BlockPerHour  // 5 hours

	// deposit defaults value
	defaultDeposit       = common.PtrBigInt(math.BigPow(10, 3))  // 173 dx per TB per month
	defaultDepositBudget = common.PtrBigInt(math.BigPow(10, 22)) // 10000 DX
	defaultMaxDeposit    = common.PtrBigInt(math.BigPow(10, 20)) // 100 DX

	// prices
	defaultBaseRPCPrice           = common.PtrBigInt(math.BigPow(10, 11))                                   // 100 nDX
	defaultContractPrice          = common.PtrBigInt(new(big.Int).Mul(math.BigPow(10, 15), big.NewInt(50))) // 50mDX
	defaultDownloadBandwidthPrice = common.PtrBigInt(math.BigPow(10, 8))                                    // 100 DX per TB
	defaultSectorAccessPrice      = common.PtrBigInt(math.BigPow(10, 13))                                   // 10 uDX
	defaultStoragePrice           = common.PtrBigInt(math.BigPow(10, 3))                                    // Same as deposit
	defaultUploadBandwidthPrice   = common.PtrBigInt(math.BigPow(10, 7))                                    // 10 DX per TB

	//Storage contract should not be empty
	emptyStorageContract = types.StorageContract{}

	//Total time to sign the contract
	postponedExecutionBuffer = storage.BlocksPerDay
)

// init set the initial value for sector height
func init() {
	sectorHeight = calculateSectorHeight()
}

// calculateSectorHeight calculate the sector height for specified sector size and leaf size
func calculateSectorHeight() uint64 {
	height := uint64(0)
	for 1<<height < (storage.SectorSize / merkle.LeafSize) {
		height++
	}
	return height
}

// defaultConfig loads the default setting when
// it is the first time use the host service, or cannot find the setting file
func defaultConfig() storage.HostIntConfig {
	return storage.HostIntConfig{
		MaxDownloadBatchSize: uint64(defaultMaxDownloadBatchSize),
		MaxDuration:          uint64(defaultMaxDuration),
		MaxReviseBatchSize:   uint64(defaultMaxReviseBatchSize),
		WindowSize:           uint64(defaultWindowSize),

		Deposit:       defaultDeposit,
		DepositBudget: defaultDepositBudget,
		MaxDeposit:    defaultMaxDeposit,

		BaseRPCPrice:           defaultBaseRPCPrice,
		ContractPrice:          defaultContractPrice,
		DownloadBandwidthPrice: defaultDownloadBandwidthPrice,
		SectorAccessPrice:      defaultSectorAccessPrice,
		StoragePrice:           defaultStoragePrice,
		UploadBandwidthPrice:   defaultUploadBandwidthPrice,
	}
}

const (
	// responsibility status
	responsibilityUnresolved storageResponsibilityStatus = iota //Storage responsibility is initialization, no meaning
	responsibilityRejected                                      //Storage responsibility never begins
	responsibilitySucceeded                                     // Successful storage responsibility
	responsibilityFailed                                        //Failed storage responsibility
)

type storageResponsibilityStatus uint64

func (i storageResponsibilityStatus) String() string {
	switch i {
	case 0:
		return "responsibilityUnresolved"
	case 1:
		return "responsibilityRejected"
	case 2:
		return "responsibilitySucceeded"
	case 3:
		return "responsibilityFailed"
	default:
		return "storageResponsibilityStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
