// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"context"
	"github.com/DxChainNetwork/godx/p2p/enode"
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
	GetStorageHostSetting(hostEnodeID enode.ID, hostEnodeURL string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetBlockByHash(blockHash common.Hash) (*types.Block, error)
	GetBlockChain() *core.BlockChain
	GetBlockByNumber(number uint64) (*types.Block, error)
	AccountManager() *accounts.Manager
	GetCurrentBlockHeight() uint64
	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	SuggestPrice(ctx context.Context) (*big.Int, error)
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	SetupConnection(enodeURL string) (Peer, error)
	IsRevising(hostID enode.ID) bool
	RenewDone(hostID enode.ID)
}

// ClientBackend is an interface that used to provide necessary functions
// to storage host manager and contract manager
type ClientBackend interface {
	Online() bool
	Syncing() bool
	GetStorageHostSetting(hostEnodeID enode.ID, hostEnodeURL string, config *HostExtConfig) error
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error)
	SetupConnection(enodeURL string) (Peer, error)
	AccountManager() *accounts.Manager
	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	SuggestPrice(ctx context.Context) (*big.Int, error)
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error)
	GetHostAnnouncementWithBlockHash(blockHash common.Hash) (hostAnnouncements []types.HostAnnouncement, number uint64, errGet error)
	GetPaymentAddress() (common.Address, error)
	IsContractRevising(hostID enode.ID) bool
	RenewDone(hostID enode.ID)
}

// DownloadParameters is the parameters to download from outer request
type DownloadParameters struct {
	RemoteFilePath   string
	WriteToLocalPath string
}
