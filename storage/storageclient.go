// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"context"
	"github.com/DxChainNetwork/godx/params"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/p2p/enode"

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

// a metadata about a storage contract.
type ClientContract struct {
	ContractID  ContractID
	HostID      enode.ID
	Transaction types.Transaction

	StartHeight uint64
	EndHeight   uint64

	// the amount remaining in the contract that the client can spend.
	ClientFunds common.BigInt

	// track the various costs manually.
	DownloadSpending common.BigInt
	StorageSpending  common.BigInt
	UploadSpending   common.BigInt

	// record status information about the contract.
	Status ContractStatus

	// the amount of money that the client spent or locked while forming a contract.
	TotalCost common.BigInt
}

// the parameters to download from outer request
type ClientDownloadParameters struct {
	Async       bool
	HttpWriter  io.Writer
	Length      uint64
	Offset      uint64
	DxFilePath  DxPath
	Destination string
}
