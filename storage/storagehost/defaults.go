package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"math/big"
)

const (
	PersistHostDir  = "storagehost" // the dir for storing the host log, json, and ect.
	HostSettingFile = "host.json"   // the file name for saving the setting of host
	HostLog         = "host.log"    // the log file for log of host
)

var (
	storageHostMeta = common.Metadata{
		Header:  "DxChain StorageHost JSON",
		Version: "DxChain host mock version",
	}

	// persistence default value
	defaultBroadcast      = false
	defaultRevisionNumber = 0

	// host internal settings default value
	// TODO: values are mock, need to compute reasonable values
	defaultMaxDuration          = 144 * 30 * 6
	defaultMaxDownloadBatchSize = 17 * (1 << 20)
	defaultMaxReviseBatchSize   = 17 * (1 << 20)
	defaultWindowSize           = 144

	// deposit defaults value
	defaultDeposit       = 0
	defaultDepositBudget = 1000000
	defaultMaxDeposit    = 10000000000000000

	// prices
	defaultBaseRPCPrice           = 2000
	defaultContractPrice          = 3000
	defaultDownloadBandwidthPrice = 4000
	defaultSectorAccessPrice      = 5000
	defaultStoragePrice           = 6000
	defaultUploadBandwidthPrice   = 7000
)

func loadDefaultsPersistence() persistence {
	return persistence{
		BroadCast:      defaultBroadcast,
		RevisionNumber: uint64(defaultRevisionNumber),
		Settings:       loadDefaultSetting(),
	}
}

func loadDefaultSetting() StorageHostIntSetting {
	return StorageHostIntSetting{
		MaxDownloadBatchSize: uint64(defaultMaxDownloadBatchSize),
		MaxDuration:          uint64(defaultMaxDuration),
		MaxReviseBatchSize:   uint64(defaultMaxReviseBatchSize),
		WindowSize:           uint64(defaultWindowSize),

		Deposit:       *big.NewInt(int64(defaultDeposit)),
		DepositBudget: *big.NewInt(int64(defaultDepositBudget)),
		MaxDeposit:    *big.NewInt(int64(defaultMaxDeposit)),

		MinBaseRPCPrice:           *big.NewInt(int64(defaultBaseRPCPrice)),
		MinContractPrice:          *big.NewInt(int64(defaultContractPrice)),
		MinDownloadBandwidthPrice: *big.NewInt(int64(defaultDownloadBandwidthPrice)),
		MinSectorAccessPrice:      *big.NewInt(int64(defaultSectorAccessPrice)),
		MinStoragePrice:           *big.NewInt(int64(defaultStoragePrice)),
		MinUploadBandwidthPrice:   *big.NewInt(int64(defaultUploadBandwidthPrice)),
	}
}
