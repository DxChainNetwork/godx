// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
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
	SetupConnection(hostEnodeUrl string) (*Session, error)
	Disconnect(hostEnodeUrl string) error
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
