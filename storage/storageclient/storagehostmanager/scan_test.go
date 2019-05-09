package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/storage"
)

type storageClientBackendTestData struct {}

func (st *storageClientBackendTestData) Online() bool {
	return true
}

func (st *storageClientBackendTestData) Syncing() bool {
	return false
}

func (st *storageClientBackendTestData) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	config = &storage.HostExtConfig{
		AcceptingContracts: true,
		Deposit: common.NewBigInt(10),
		MaxDeposit: common.NewBigInt(100),
	}
	return nil
}

func (st *storageClientBackendTestData) SubscribeChainChangeEvent(ch chan <- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (st *storageClientBackendTestData) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return nil, nil
}


var storageHostManager = &StorageHostManager{
	b: &storageClientBackendTestData{},
	rent: storage.DefaultRentPayment,
	scanLookup: make(map[string]struct{}),

}