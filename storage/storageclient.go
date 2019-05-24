// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/rpc"
)

// EthBackend is an interface used to get methods implemented by Ethereum
type EthBackend interface {
	APIs() []rpc.API
	GetStorageHostSetting(peerID string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetBlockByHash(blockHash common.Hash) (*types.Block, error)
	GetBlockChain() *core.BlockChain
	SetupConnection(hostEnodeUrl string) (*Session, error)
	Disconnect(hostEnodeUrl string) error

	AccountManager() *accounts.Manager
	GetCurrentBlockHeight() uint64
}

// ClientBackend is an interface that used to provide necessary functions
// to storagehostmanager and contract manager
type ClientBackend interface {
	Online() bool
	Syncing() bool
	GetStorageHostSetting(peerID string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error)
}

// A StorageClientContract contains metadata about a storage contract. It is read-only;
// modifying a StorageClientContract does not modify the actual storage contract.
type ClientContract struct {
	ID            common.Hash
	HostPublicKey *ecdsa.PublicKey
	Transaction   types.Transaction

	StartHeight uint64
	EndHeight   uint64

	// ClientFunds is the amount remaining in the contract that the client can spend.
	ClientFunds common.BigInt

	// The StorageContract does not indicate what funds were spent on, so we have
	// to track the various costs manually.
	DownloadSpending common.BigInt
	StorageSpending  common.BigInt
	UploadSpending   common.BigInt

	// Utility contains utility information about the client.
	Utility ContractUtility

	// TotalCost indicates the amount of money that the client spent and/or
	// locked up while forming a contract.
	TotalCost common.BigInt
}

// ContractUtility contains metrics internal to the contractor that reflect the
// utility of a given contract.
type ContractUtility struct {
	GoodForUpload bool
	GoodForRenew  bool
	Locked        bool // Locked utilities can only be set to false.
}
