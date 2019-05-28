// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"crypto/ecdsa"
	"errors"
	"reflect"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// ParsedAPI will parse the APIs saved in the Ethereum
// and get the ones needed
type ParsedAPI struct {
	netInfo *ethapi.PublicNetAPI
	account *ethapi.PrivateAccountAPI
	ethInfo *ethapi.PublicEthereumAPI
}

// filterAPIs will filter the APIs saved in the Ethereum and
// save them into ParsedAPI data structure
func (sc *StorageClient) filterAPIs(apis []rpc.API) error {
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

// Online will be used to indicate if the local node is connected to the internet
func (sc *StorageClient) Online() bool {
	return sc.info.netInfo.PeerCount() > 0
}

// Syncing will be used to indicate if the local node is syncing with the blockchain
func (sc *StorageClient) Syncing() bool {
	sync, _ := sc.info.ethInfo.Syncing()
	syncing, ok := sync.(bool)
	if ok && !syncing {
		return false
	}

	return true
}

// GetTxByBlockHash will be used to get the detailed transaction by using the block hash
func (sc *StorageClient) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	block, err := sc.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

// GetStorageHostSetting will be used to get the storage host's external setting based on the
// peerID provided
func (sc *StorageClient) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	return sc.ethBackend.GetStorageHostSetting(peerID, config)
}

// SubscribeChainChangeEvent will be used to get block information every time a change happened
// in the blockchain
func (sc *StorageClient) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return sc.ethBackend.SubscribeChainChangeEvent(ch)
}

// GetStorageHostManager will be used to acquire the storage host manager
func (sc *StorageClient) GetStorageHostManager() *storagehostmanager.StorageHostManager {
	return sc.storageHostManager
}

// calculate Enode.ID, reference:
// p2p/discover/node.go:41
// p2p/discover/node.go:59
func PubkeyToEnodeID(pubkey *ecdsa.PublicKey) enode.ID {
	var pubBytes [64]byte
	math.ReadBits(pubkey.X, pubBytes[:len(pubBytes)/2])
	math.ReadBits(pubkey.Y, pubBytes[len(pubBytes)/2:])
	return enode.ID(crypto.Keccak256Hash(pubBytes[:]))
}
