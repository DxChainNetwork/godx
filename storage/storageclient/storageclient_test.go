// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"context"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/eth/downloader"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

type StorageClientTester struct {
	Client  *StorageClient
	Backend *BackendTest
}

func newStorageClientTester(t *testing.T) *StorageClientTester {
	client, err := New(filepath.Join(homeDir(), "storageclient"))
	if err != nil {
		return nil
	}

	b := &BackendTest{}
	return &StorageClientTester{Client: client, Backend: b}
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func newTestServer() *p2p.Server {
	config := p2p.Config{
		Name:       "test",
		MaxPeers:   10,
		ListenAddr: "127.0.0.1:0",
	}
	server := &p2p.Server{
		Config: config,
	}
	return server
}

type BackendTest struct{}

func (b *BackendTest) APIs() []rpc.API {
	var res []rpc.API
	return res
}

func (b *BackendTest) GetStorageHostSetting(hostEnodeUrl string, config *storage.HostExtConfig) error {
	return nil
}

func (b *BackendTest) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	var feed event.Feed
	c := make(chan int)
	return feed.Subscribe(c)
}

func (b *BackendTest) GetBlockByHash(blockHash common.Hash) (*types.Block, error) {
	return &types.Block{}, nil
}

func (b *BackendTest) GetBlockChain() *core.BlockChain {
	return &core.BlockChain{}
}

func (b *BackendTest) SetupConnection(hostEnodeUrl string) (*storage.Session, error) {
	return &storage.Session{}, nil
}

func (b *BackendTest) Disconnect(session *storage.Session, hostEnodeUrl string) error {
	return nil
}

func (b *BackendTest) AccountManager() *accounts.Manager {
	return &accounts.Manager{}
}

func (b *BackendTest) GetCurrentBlockHeight() uint64 {
	return 0
}

func (b *BackendTest) Downloader() *downloader.Downloader {
	return nil
}

func (b *BackendTest) ProtocolVersion() int {
	return 100
}

func (b *BackendTest) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return big.NewInt(100), nil
}

func (b *BackendTest) ChainDb() ethdb.Database {
	return nil
}

func (b *BackendTest) EventMux() *event.TypeMux {
	return nil
}

// BlockChain API
func (b *BackendTest) SetHead(number uint64) {

}

func (b *BackendTest) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	return &types.Header{}, nil
}

func (b *BackendTest) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	return &types.Block{}, nil
}

func (b *BackendTest) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	return &state.StateDB{}, &types.Header{}, nil
}

func (b *BackendTest) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return &types.Block{}, nil
}

func (b *BackendTest) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return types.Receipts{}, nil
}
func (b *BackendTest) GetTd(blockHash common.Hash) *big.Int {
	return big.NewInt(100)
}

func (b *BackendTest) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	return nil, nil, nil
}
func (b *BackendTest) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return nil
}

func (b *BackendTest) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return nil
}
func (b *BackendTest) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return nil
}

func (b *BackendTest) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return nil
}
func (b *BackendTest) GetPoolTransactions() (types.Transactions, error) {
	return nil, nil
}
func (b *BackendTest) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	return &types.Transaction{}
}

func (b *BackendTest) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return 100, nil
}
func (b *BackendTest) Stats() (pending int, queued int) {
	return 100, 100
}
func (b *BackendTest) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return nil, nil
}
func (b *BackendTest) SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription {
	return nil
}

func (b *BackendTest) ChainConfig() *params.ChainConfig {
	return nil
}

func (b *BackendTest) CurrentBlock() *types.Block {
	return nil
}
