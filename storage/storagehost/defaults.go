// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/core/types"
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

	//Total time to sign the contract
	PostponedExecutionBuffer = 12 * unit.BlocksPerHour
)

var (
	storageHostMeta = common.Metadata{
		Header:  "DxChain StorageHost JSON",
		Version: "V1.0",
	}

	//Storage contract should not be empty
	emptyStorageContract = types.StorageContract{}
)

// defaultConfig loads the default setting when
// it is the first time use the host service, or cannot find the setting file
func defaultConfig() storage.HostIntConfig {
	return storage.HostIntConfig{
		MaxDownloadBatchSize: uint64(storage.DefaultMaxDownloadBatchSize),
		MaxDuration:          uint64(storage.DefaultMaxDuration),
		MaxReviseBatchSize:   uint64(storage.DefaultMaxReviseBatchSize),
		WindowSize:           uint64(storage.ProofWindowSize),

		Deposit:       storage.DefaultDeposit,
		DepositBudget: storage.DefaultDepositBudget,
		MaxDeposit:    storage.DefaultMaxDeposit,

		BaseRPCPrice:           storage.DefaultBaseRPCPrice,
		ContractPrice:          storage.DefaultContractPrice,
		DownloadBandwidthPrice: storage.DefaultDownloadBandwidthPrice,
		SectorAccessPrice:      storage.DefaultSectorAccessPrice,
		StoragePrice:           storage.DefaultStoragePrice,
		UploadBandwidthPrice:   storage.DefaultUploadBandwidthPrice,
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
