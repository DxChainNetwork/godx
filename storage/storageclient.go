// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"context"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rpc"
)

// EthBackend is an interface used to get methods implemented by Ethereum
type EthBackend interface {
	APIs() []rpc.API
	GetStorageHostSetting(hostEnodeUrl string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetBlockByHash(blockHash common.Hash) (*types.Block, error)
	GetBlockChain() *core.BlockChain
	SetupConnection(hostEnodeUrl string) (*Session, error)
	Disconnect(session *Session, hostEnodeUrl string) error
	GetBlockByNumber(number uint64) (*types.Block, error)

	AccountManager() *accounts.Manager
	GetCurrentBlockHeight() uint64
	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	SuggestPrice(ctx context.Context) (*big.Int, error)
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
}

// ClientBackend is an interface that used to provide necessary functions
// to storagehostmanager and contract manager
type ClientBackend interface {
	Online() bool
	Syncing() bool
	GetStorageHostSetting(hostEnodeUrl string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error)
	SetupConnection(hostEnodeUrl string) (*Session, error)
	AccountManager() *accounts.Manager
	Disconnect(session *Session, hostEnodeUrl string) error
	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	SuggestPrice(ctx context.Context) (*big.Int, error)
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
}

// the parameters to download from outer request
type DownloadParameters struct {
	RemoteFilePath   string
	WriteToLocalPath string
}
