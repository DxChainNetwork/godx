package storageclient

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"reflect"
)

type ParsedAPI struct {
	netInfo *ethapi.PublicNetAPI
	account *ethapi.PrivateAccountAPI
	ethInfo *ethapi.PublicEthereumAPI
}


func (sc *StorageClient) filterAPIs(apis []rpc.API) error{
	for _, api := range apis {
		switch typ := reflect.TypeOf(api.Service); typ {
		case reflect.TypeOf(&ethapi.PublicNetAPI{}):
			netAPI := api.Service.(*ethapi.PublicNetAPI)
			if netAPI == nil {
				return errors.New("failed to acquire netInfo information")
			}
			sc.info.netInfo = netAPI
		case reflect.TypeOf(&ethapi.PrivateAccountAPI{}):
			accountAPI := api.Service.(*ethapi.PrivateAccountAPI)
			if accountAPI == nil {
				return errors.New("failed to acquire account information")
			}
			sc.info.account = accountAPI
		case reflect.TypeOf(&ethapi.PublicEthereumAPI{}):
			ethAPI := api.Service.(*ethapi.PublicEthereumAPI)
			if ethAPI == nil {
				return errors.New("failed to acquire eth information")
			}
			sc.info.ethInfo = ethAPI
		default:
			continue
		}
	}
	return nil
}


func (sc *StorageClient) Online() bool {
	return sc.info.netInfo.PeerCount() > 0
}

func (sc *StorageClient) Syncing() bool {
	sync, _ := sc.info.ethInfo.Syncing()
	syncing, ok := sync.(bool)
	if ok && !syncing {
		return false
	}

	return true
}

func (sc *StorageClient) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	block, err := sc.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

func (sc *StorageClient) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	return sc.ethBackend.GetStorageHostSetting(peerID, config)
}

func (sc *StorageClient) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription{
	return sc.ethBackend.SubscribeChainChangeEvent(ch)
}

func (sc *StorageClient) GetStorageHostManager() *storagehostmanager.StorageHostManager{
	return sc.storageHostManager
}

