// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"context"
	"math/big"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/eth/downloader"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

var hashes = []string{"0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd50", "0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd51",
	"0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd53", "0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd54", "0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd55"}

type StorageClientTester struct {
	Client  *StorageClient
	Backend *BackendTest
}

func newFileEntry(t *testing.T, client *StorageClient) *dxfile.FileSetEntryWithID {
	ec, err := erasurecode.New(erasurecode.ECTypeStandard, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}

	mb := 9
	filePath, fileSize, _ := generateFile(t, homeDir(), mb)

	entry, err := client.fileSystem.NewDxFile(randomDxPath(), storage.SysPath(filePath), false, ec, ck, uint64(fileSize), 777)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func newStorageClientTester(t *testing.T) *StorageClientTester {
	client, err := New(filepath.Join(homeDir(), "storageclient"))
	if err != nil {
		return nil
	}

	b := &BackendTest{}

	if err := client.fileSystem.Start(); err != nil {
		t.Fatal(err)
	}

	return &StorageClientTester{Client: client, Backend: b}
}

// For only test: add mock workers to workpool
func mockAddWorkers(n int, client *StorageClient) {
	for i := 0; i < n; i++ {
		contractID := storage.ContractID(common.HexToHash(hashes[i]))
		worker := &worker{
			contract:     storage.ContractMetaData{ID: contractID},
			hostID:       enode.RandomID(enode.ID{}, i),
			downloadChan: make(chan struct{}, 1),
			uploadChan:   make(chan struct{}, 1),
			killChan:     make(chan struct{}),
			client:       client,
		}
		client.workerPool[storage.ContractID(contractID)] = worker
	}
}

// randomDxPath creates a random DxPath which is a string of byte slice of length 16
func randomDxPath() storage.DxPath {
	b := make([]byte, 16)
	rand.Seed(time.Now().UnixNano())
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	path, err := storage.NewDxPath(common.Bytes2Hex(b))
	if err != nil {
		panic(err)
	}
	return path
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

type BackendTest struct{}

func (b *BackendTest) SelfEnodeURL() string { return "" }

func (b *BackendTest) SetStatic(node *enode.Node) {}

func (b *BackendTest) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (b *BackendTest) APIs() []rpc.API {
	var res []rpc.API
	return res
}

func (b *BackendTest) GetStorageHostSetting(hostEnodeID enode.ID, hostEnodeURL string, config *storage.HostExtConfig) error {
	return nil
}

func (b *BackendTest) IsRevising(hostID enode.ID) bool {
	return false
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

func (b *BackendTest) SetupConnection(enodeURL string) (storage.Peer, error) {
	return nil, nil
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

func (b *BackendTest) GetBlockByNumber(number uint64) (*types.Block, error) {
	return &types.Block{}, nil
}

func (b *BackendTest) SignByNode(hash []byte) ([]byte, error) {
	return []byte{}, nil
}

func (b *BackendTest) GetHostEnodeURL() string {
	return ""
}

func (b *BackendTest) TryToRenewOrRevise(hostID enode.ID) bool { return false }

func (b *BackendTest) RevisionOrRenewingDone(hostID enode.ID) {}

/*
_____  _____  _______      __  _______ ______        ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__         | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|        |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____       | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|      |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func settingValidation(settings storage.ClientSetting) (expectedErr bool) {
	if settings.MaxUploadSpeed < 0 || settings.MaxDownloadSpeed < 0 {
		return true
	}

	if err := contractmanager.RentPaymentValidation(settings.RentPayment); err != nil {
		return true
	}

	return false
}

func randomClientSettingsGenerator() (settings storage.ClientSetting) {
	settings = storage.ClientSetting{
		RentPayment:       randRentPaymentGenerator(),
		EnableIPViolation: true,
		MaxUploadSpeed:    randInt64(),
		MaxDownloadSpeed:  randInt64(),
	}

	return
}

func randRentPaymentGenerator() (rentPayment storage.RentPayment) {
	rentPayment = storage.RentPayment{
		Fund:               common.RandomBigInt(),
		StorageHosts:       randUint64(),
		Period:             randUint64(),
		ExpectedStorage:    0,
		ExpectedUpload:     0,
		ExpectedDownload:   0,
		ExpectedRedundancy: 0,
	}

	return
}

func randUint64() (randUint uint64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint64()
}

func randFloat64() (randFloat float64) {
	rand.Seed(time.Now().UnixNano())
	return rand.Float64()
}

func randInt64() (randBool int64) {
	rand.Seed(time.Now().UnixNano())
	return int64(rand.Int())
}
