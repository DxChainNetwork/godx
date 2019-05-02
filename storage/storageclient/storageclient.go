package storageclient

import (
	"errors"
	"github.com/DxChainNetwork/godx/common/threadmanager"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage/storageclient/memorymanager"
	"path/filepath"
	"reflect"
	"sync"
)

// ************** MOCKING DATA *****************
// *********************************************
type (
	storageHostManager struct{}
	contractManager    struct{}
	StorageContractID  struct{}
	StorageHostEntry   struct{}
	streamCache        struct{}
	Wal                struct{}
)
// *********************************************
// *********************************************

// Backend allows Ethereum object to be passed in as interface
type Backend interface {
	APIs() []rpc.API
}

// StorageClient contains fileds that are used to perform StorageHost
// selection operation, file uploading, downloading operations, and etc.
type StorageClient struct {
	// TODO (jacky): File Management Related

	// TODO (jacky): File Download Related

	// TODO (jacky): File Upload Related

	// Todo (jacky): File Recovery Related

	// Memory Management
	memoryManager *memorymanager.MemoryManager

	// contract manager and storage host manager
	contractManager    contractManager
	storageHostManager storageHostManager

	// TODO (jacky): workerpool

	// Cache the hosts from the last price estimation result
	lastEstimationStorageHost []StorageHostEntry

	// Directories and File related
	persist        persistence
	persistDir     string
	staticFilesDir string

	// Utilities
	streamCache *streamCache
	log         log.Logger
	// TODO (jacky): considering using the Lock and Unlock with ID ?
	lock    sync.Mutex
	tm      threadmanager.ThreadManager
	wal     Wal
	network *ethapi.PublicNetAPI
	account *ethapi.PrivateAccountAPI
}

// New initializes StorageClient object
func New(persistDir string) (*StorageClient, error) {

	// TODO (Jacky): data initialization
	sc := &StorageClient{
		persistDir:     persistDir,
		staticFilesDir: filepath.Join(persistDir, DxPathRoot),
	}

	sc.memoryManager = memorymanager.New(DefaultMaxMemory, sc.tm.GetStopChan())

	return sc, nil
}

// Start controls go routine checking and updating process
func (sc *StorageClient) Start(eth Backend) error {
	// getting all needed API functions
	sc.filterAPIs(eth.APIs())

	// validation
	if sc.network == nil {
		return errors.New("failed to acquire network information")
	}

	if sc.account == nil {
		return errors.New("failed to acquire account information")
	}

	// TODO (mzhang): Initialize ContractManager & HostManager -> assign to StorageClient

	// Load settings from persist file
	if err := sc.loadPersist(); err != nil {
		return err
	}

	// TODO (mzhang): Subscribe consensus change

	// TODO (Jacky): DxFile / DxDirectory Update & Initialize Stream Cache

	// TODO (Jacky): Starting Worker, Checking file healthy, etc.

	// TODO (mzhang): Register On Stop Thread Control Function, waiting for WAL

	return nil
}

func (sc *StorageClient) filterAPIs(apis []rpc.API) {
	for _, api := range apis {
		switch typ := reflect.TypeOf(api.Service); typ {
		case reflect.TypeOf(&ethapi.PublicNetAPI{}):
			sc.network = api.Service.(*ethapi.PublicNetAPI)
		case reflect.TypeOf(&ethapi.PrivateAccountAPI{}):
			sc.account = api.Service.(*ethapi.PrivateAccountAPI)
		default:
			continue
		}
	}
}

func (sc *StorageClient) setBandwidthLimits(uploadSpeedLimit int64, downloadSpeedLimit int64) error {
	// validation
	if uploadSpeedLimit < 0 || downloadSpeedLimit < 0 {
		return errors.New("upload/download speed limit cannot be negative")
	}

	// Update the contract settings accordingly
	if uploadSpeedLimit == 0 && downloadSpeedLimit == 0 {
		// TODO (mzhang): update contract settings using contract manager
	} else {
		// TODO (mzhang): update contract settings to the loaded data
	}

	return nil
}
