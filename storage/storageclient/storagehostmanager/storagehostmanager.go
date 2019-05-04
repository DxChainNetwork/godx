// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common/hexutil"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common/threadManager"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

type StorageHostManager struct {
	// getting related information
	ethInfo   *ethapi.PublicEthereumAPI
	chainInfo *ethapi.PublicBlockChainAPI
	netInfo   *ethapi.PublicNetAPI

	rent            storage.RentPayment
	evalFunc        storagehosttree.EvaluationFunc
	gasFee          hexutil.Uint64
	storageHostTree *storagehosttree.StorageHostTree

	// ip violation check
	disableIPViolationCheck bool

	// maintenance related
	initialScan          bool
	initialScanLatencies []time.Duration
	scanList             []storage.HostInfo
	scanHosts            map[string]struct{}
	scanWait             bool
	scanRoutines         int

	// persistent directory
	persistDir string

	// utils
	log  log.Logger
	lock sync.RWMutex
	tm   threadmanager.ThreadManager

	// TODO: the filteredHosts has the value type PublicKey. May need to change later
	// filter mode related
	filterMode    FilterMode
	filteredHosts map[string]string
	filteredTree  *storagehosttree.StorageHostTree

	//TODO (mzhang): Consensus Change Related, sync with HZ Office
	blockHeight uint64
}

// New will initialize HostPoolManager, making the host pool stay updated
// , establish connection to the storage host, getting storage host information and etc.
func New(persistDir string, ethInfo *ethapi.PublicEthereumAPI, chainInfo *ethapi.PublicBlockChainAPI, netInfo *ethapi.PublicNetAPI) (*StorageHostManager, error) {
	// initialization
	shm := &StorageHostManager{
		ethInfo:    ethInfo,
		chainInfo:  chainInfo,
		netInfo:    netInfo,
		persistDir: persistDir,

		rent: storage.DefaultRentPayment,

		scanHosts:     make(map[string]struct{}),
		filteredHosts: make(map[string]string),
	}

	// TODO: get the gas fee estimation
	//chainInfo.EstimateGas()

	return shm, nil
}
