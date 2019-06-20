package storagehost

import (
	"math/big"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	// PersistHostDir is dir path for storing the host log, json, and ect.
	PersistHostDir = "storageHost"
	// HostSettingFile is the file name for saving the setting of host
	HostSettingFile = "host.json"
	// HostDB is the database dir for storing host obligation
	HostDB = "hostdb"
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

	// TODO: ALL values are mock, need to compute reasonable values

	storageHostMeta = common.Metadata{
		Header:  "DxChain StorageHost JSON",
		Version: "DxChain host mock version",
	}

	// persistence default value
	//defaultBroadcast      = false
	//defaultRevisionNumber = 0

	// host internal config default value
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

// loadDefaultConfig loads the default setting when
// it is the first time use the host service, or cannot find the setting file
func loadDefaultConfig() storage.HostIntConfig {
	return storage.HostIntConfig{
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
