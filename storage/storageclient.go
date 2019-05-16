package storage

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/rpc"
)

type EthBackend interface {
	APIs() []rpc.API
	GetStorageHostSetting(peerID string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetBlockByHash(blockHash common.Hash) (*types.Block, error)
	SetupConnection(hostEnodeUrl string) (*Session, error)
	Disconnect(hostEnodeUrl string) error
}

type ClientBackend interface {
	Online() bool
	Syncing() bool
	GetStorageHostSetting(peerID string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error)
}
