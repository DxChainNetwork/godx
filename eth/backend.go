// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package eth implements the Ethereum protocol.
package eth

import (
	"context"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/crypto"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/consensus/clique"
	"github.com/DxChainNetwork/godx/consensus/ethash"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/bloombits"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/eth/downloader"
	"github.com/DxChainNetwork/godx/eth/filters"
	"github.com/DxChainNetwork/godx/eth/gasprice"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/miner"
	"github.com/DxChainNetwork/godx/node"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Ethereum implements the Ethereum full node service.
type Ethereum struct {
	storageHost *storagehost.StorageHost

	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Ethereum

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer
	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	APIBackend *EthAPIBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etherbase common.Address

	apisOnce       sync.Once
	registeredAPIs []rpc.API
	storageClient  *storageclient.StorageClient

	networkID     uint64
	netRPCService *ethapi.PublicNetAPI

	server *p2p.Server

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}

func (s *Ethereum) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Ethereum object (including the
// initialisation of the common Ethereum object)
func New(ctx *node.ServiceContext, config *Config) (*Ethereum, error) {
	// Ensure configuration values are compatible and sane
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run eth.Ethereum in light sync mode, use les.LightEthereum")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.MinerGasPrice == nil || config.MinerGasPrice.Cmp(common.Big0) <= 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.MinerGasPrice, "updated", DefaultConfig.MinerGasPrice)
		config.MinerGasPrice = new(big.Int).Set(DefaultConfig.MinerGasPrice)
	}
	// Assemble the Ethereum object
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.ConstantinopleOverride)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	eth := &Ethereum{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, chainConfig, &config.Ethash, config.MinerNotify, config.MinerNoverify, chainDb),
		shutdownChan:   make(chan bool),
		networkID:      config.NetworkId,
		gasPrice:       config.MinerGasPrice,
		etherbase:      config.Etherbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
	}

	log.Info("Initialising Ethereum protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := rawdb.ReadDatabaseVersion(chainDb)
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Geth %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
		} else if bcVersion != nil && *bcVersion < core.BlockChainVersion {
			log.Warn("Upgrade blockchain database version", "from", *bcVersion, "to", core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
			EWASMInterpreter:        config.EWASMInterpreter,
			EVMInterpreter:          config.EVMInterpreter,
		}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieCleanLimit: config.TrieCleanCache, TrieDirtyLimit: config.TrieDirtyCache, TrieTimeLimit: config.TrieTimeout}
	)
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, eth.chainConfig, eth.engine, vmConfig, eth.shouldPreserve)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		eth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	eth.txPool = core.NewTxPool(config.TxPool, eth.chainConfig, eth.blockchain)

	if eth.protocolManager, err = NewProtocolManager(eth, eth.chainConfig, config.SyncMode, config.NetworkId, eth.eventMux, eth.txPool, eth.engine, eth.blockchain, chainDb, config.Whitelist); err != nil {
		return nil, err
	}

	eth.miner = miner.New(eth, eth.chainConfig, eth.EventMux(), eth.engine, config.MinerRecommit, config.MinerGasFloor, config.MinerGasCeil, eth.isLocalBlock)
	eth.miner.SetExtra(makeExtraData(config.MinerExtraData))

	eth.APIBackend = &EthAPIBackend{eth, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	eth.APIBackend.gpo = gasprice.NewOracle(eth.APIBackend, gpoParams)

	// if both storageClient and storageHost are true, return error directly
	if config.StorageClient && config.StorageHost {
		return nil, errors.New("a node can only become storage client or storage host, not both")
	}

	// Initialize StorageClient
	if config.StorageClient {
		clientPath := ctx.ResolvePath(config.StorageClientDir)
		eth.storageClient, err = storageclient.New(clientPath)
		if err != nil {
			return nil, err
		}
	} else if config.StorageHost {
		// Initialize StorageHost
		hostPath := ctx.ResolvePath(storagehost.PersistHostDir)
		eth.storageHost, err = storagehost.New(hostPath)
		if err != nil {
			return nil, err
		}

	}

	return eth, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"geth",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*ethdb.LDBDatabase); ok {
		db.Meter("eth/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service
func CreateConsensusEngine(ctx *node.ServiceContext, chainConfig *params.ChainConfig, config *ethash.Config, notify []string, noverify bool, db ethdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// Otherwise assume proof-of-work
	switch config.PowMode {
	case ethash.ModeFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case ethash.ModeTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester(nil, noverify)
	case ethash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ethash.Config{
			CacheDir:       ctx.ResolvePath(config.CacheDir),
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		}, notify, noverify)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Ethereum) APIs() []rpc.API {
	getAPI := func() {
		apis := ethapi.GetAPIs(s.APIBackend)

		// Append any APIs exposed explicitly by the consensus engine
		apis = append(apis, s.engine.APIs(s.BlockChain())...)

		// Append all the local APIs and return
		s.registeredAPIs = append(apis, []rpc.API{
			{
				Namespace: "eth",
				Version:   "1.0",
				Service:   NewPublicEthereumAPI(s),
				Public:    true,
			}, {
				Namespace: "eth",
				Version:   "1.0",
				Service:   NewPublicMinerAPI(s),
				Public:    true,
			}, {
				Namespace: "eth",
				Version:   "1.0",
				Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
				Public:    true,
			}, {
				Namespace: "miner",
				Version:   "1.0",
				Service:   NewPrivateMinerAPI(s),
				Public:    false,
			}, {
				Namespace: "eth",
				Version:   "1.0",
				Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
				Public:    true,
			}, {
				Namespace: "admin",
				Version:   "1.0",
				Service:   NewPrivateAdminAPI(s),
			}, {
				Namespace: "debug",
				Version:   "1.0",
				Service:   NewPublicDebugAPI(s),
				Public:    true,
			}, {
				Namespace: "debug",
				Version:   "1.0",
				Service:   NewPrivateDebugAPI(s.chainConfig, s),
			}, {
				Namespace: "net",
				Version:   "1.0",
				Service:   s.netRPCService,
				Public:    true,
			},
		}...)

		// based on the user's option, choose to register storage client
		// or storage host APIs
		if s.config.StorageClient {
			// StorageClient related APIs
			storageClientAPIs := []rpc.API{
				{
					Namespace: "sclient",
					Version:   "1.0",
					Service:   storageclient.NewPublicStorageClientAPI(s.storageClient),
					Public:    true,
				}, {
					Namespace: "sclient",
					Version:   "1.0",
					Service:   storageclient.NewPrivateStorageClientAPI(s.storageClient),
					Public:    false,
				}, {
					Namespace: "clientfiles",
					Version:   "1.0",
					Service:   filesystem.NewPublicFileSystemAPI(s.storageClient.GetFileSystem()),
					Public:    true,
				},
			}
			s.registeredAPIs = append(s.registeredAPIs, storageClientAPIs...)
		} else if s.config.StorageHost {
			storageHostAPIs := []rpc.API{
				{
					Namespace: "shost",
					Version:   "1.0",
					Service:   storagehost.NewHostPrivateAPI(s.storageHost),
					Public:    false,
				},
			}
			s.registeredAPIs = append(s.registeredAPIs, storageHostAPIs...)
		}
	}

	s.apisOnce.Do(getAPI)
	return s.registeredAPIs
}

func (s *Ethereum) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Ethereum) Etherbase() (eb common.Address, err error) {
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()

	if etherbase != (common.Address{}) {
		return etherbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase := accounts[0].Address

			s.lock.Lock()
			s.etherbase = etherbase
			s.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: etherbase
// and accounts specified via `txpool.locals` flag.
func (s *Ethereum) isLocalBlock(block *types.Block) bool {
	author, err := s.engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whether the given address is etherbase.
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()
	if author == etherbase {
		return true
	}
	// Check whether the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// shouldPreserve checks whether we should preserve the given block
// during the chain reorg depending on whether the author of block
// is a local account.
func (s *Ethereum) shouldPreserve(block *types.Block) bool {
	// The reason we need to disable the self-reorg preserving for clique
	// is it can be probable to introduce a deadlock.
	//
	// e.g. If there are 7 available signers
	//
	// r1   A
	// r2     B
	// r3       C
	// r4         D
	// r5   A      [X] F G
	// r6    [X]
	//
	// In the round5, the inturn signer E is offline, so the worst case
	// is A, F and G sign the block of round5 and reject the block of opponents
	// and in the round6, the last available signer B is offline, the whole
	// network is stuck.
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}
	return s.isLocalBlock(block)
}

// SetEtherbase sets the mining reward address.
func (s *Ethereum) SetEtherbase(etherbase common.Address) {
	s.lock.Lock()
	s.etherbase = etherbase
	s.lock.Unlock()

	s.miner.SetEtherbase(etherbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (s *Ethereum) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := s.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignHash)
		}
		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)

		go s.miner.Start(eb)
	}
	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (s *Ethereum) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	s.miner.Stop()
}

func (s *Ethereum) IsMining() bool      { return s.miner.Mining() }
func (s *Ethereum) Miner() *miner.Miner { return s.miner }

func (s *Ethereum) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *Ethereum) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *Ethereum) TxPool() *core.TxPool               { return s.txPool }
func (s *Ethereum) EventMux() *event.TypeMux           { return s.eventMux }
func (s *Ethereum) Engine() consensus.Engine           { return s.engine }
func (s *Ethereum) ChainDb() ethdb.Database            { return s.chainDb }
func (s *Ethereum) IsListening() bool                  { return true } // Always listening
func (s *Ethereum) EthVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Ethereum) NetVersion() uint64                 { return s.networkID }
func (s *Ethereum) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *Ethereum) GetCurrentBlockHeight() uint64      { return s.blockchain.CurrentHeader().Number.Uint64() }
func (s *Ethereum) GetBlockChain() *core.BlockChain    { return s.blockchain }

// Sign data with node private key. Now it is used to imply host identity
func (s *Ethereum) SignWithNodeSk(hash []byte) ([]byte, error) {
	return crypto.Sign(hash, s.server.Config.PrivateKey)
}

// Get host enode url from enode object
func (s *Ethereum) GetHostEnodeURL() string {
	return s.server.Self().String()
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Ethereum) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ethereum protocol implementation.
func (s *Ethereum) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Start the RPC service
	s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.NetVersion())
	s.server = srvr

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}

	// Start Storage Client
	if s.config.StorageClient {
		err := s.storageClient.Start(s, s.APIBackend)
		if err != nil {
			return err
		}
	}

	// Start Storage Host
	if s.config.StorageHost {
		err := s.storageHost.Start(s)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (s *Ethereum) Stop() error {
	var fullErr error

	err := s.bloomIndexer.Close()
	fullErr = common.ErrCompose(fullErr, err)

	s.blockchain.Stop()

	err = s.engine.Close()
	fullErr = common.ErrCompose(fullErr, err)

	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}

	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()

	if s.config.StorageClient {
		err = s.storageClient.Close()
		fullErr = common.ErrCompose(fullErr, err)
	}

	if s.config.StorageHost {
		err = s.storageHost.Close()
		fullErr = common.ErrCompose(fullErr, err)
	}

	close(s.shutdownChan)

	return nil
}

// IsRevising is used to check if the contract is currently
// revising
func (s *Ethereum) TryToRenewOrRevise(hostID enode.ID) bool {
	peerID := fmt.Sprintf("%x", hostID.Bytes()[:8])
	peer := s.protocolManager.peers.Peer(peerID)
	// if the peer does not exist, meaning currently not revising
	if peer == nil {
		return false
	}

	// otherwise, check if the current connection is revising the contract
	return peer.TryToRenewOrRevise()
}

// RevisionOrRenewingDone indicates the renew finished
func (s *Ethereum) RevisionOrRenewingDone(hostID enode.ID) {
	peerID := fmt.Sprintf("%x", hostID.Bytes()[:8])
	peer := s.protocolManager.peers.Peer(peerID)
	if peer == nil {
		return
	}

	// finished renewing
	peer.RevisionOrRenewingDone()
}

func (s *Ethereum) SetupConnection(enodeURL string) (storagePeer storage.Peer, err error) {
	// get the peer ID
	var destNode *enode.Node
	if destNode, err = enode.ParseV4(enodeURL); err != nil {
		err = fmt.Errorf("failed to parse the enodeURL: %s", err.Error())
		return
	}

	// get the peerID
	peerID := fmt.Sprintf("%x", destNode.ID().Bytes()[:8])

	// check if the peer is already existed
	peer := s.protocolManager.peers.Peer(peerID)
	if peer != nil {
		// if the connection already existed, convert the connection
		// to the static connection
		s.server.SetStatic(destNode)
		// connection is already established
		storagePeer = peer
		return
	}

	// the connection has not been established yet, call add peer
	s.server.AddPeer(destNode)
	timeout := time.After(1 * time.Minute)
	for {
		peer = s.protocolManager.peers.Peer(peerID)
		if peer != nil {
			// check if the connection is static connection
			// if not, set the connection to static connection
			if !peer.IsStaticConn() {
				s.server.SetStatic(destNode)
			}
			// assign the peer and return
			storagePeer = peer
			return
		}

		select {
		case <-timeout:
			err = errors.New("set up connection time out")
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// GetStorageHostSetting will send message to the peer with the corresponded peer ID
func (s *Ethereum) GetStorageHostSetting(enodeID enode.ID, enodeURL string, config *storage.HostExtConfig) error {
	// set up the connection to the storage host node
	sp, err := s.SetupConnection(enodeURL)
	if err != nil {
		return fmt.Errorf("failed to get the storage host configuration: %s", err.Error())
	}

	// check if the client is currently requesting the host config
	// once done, release the channel
	if err := sp.TryRequestHostConfig(); err != nil {
		return err
	}
	defer sp.RequestHostConfigDone()

	// send storage host config request information
	if err := sp.RequestStorageHostConfig(); err != nil {
		return fmt.Errorf("failed to request storage host configuration: %s", err)
	}

	// wait until the result is given back
	msg, err := sp.WaitConfigResp()
	if err != nil {
		return fmt.Errorf("received error while waiting for retriving storage host config: %s", err.Error())
	}

	if err := msg.Decode(config); err != nil {
		return fmt.Errorf("error decoding the storage configuration: %s", err.Error())
	}

	log.Info("Successfully get the storage host settings")

	// once the setting is successfully retrieved, check the connection
	// if the node is the static connection originally, do nothing
	if s.server.IsAddedByUser(enodeID) {
		return nil
	}

	// otherwise, stay connected but remove it from the static connection list, and change
	// the connection type. The error message is ignored intentionally. If the parsing failed,
	// there is no way to get the storage host config
	_ = s.server.DeleteStatic(enodeURL)

	return nil
}

// SubscribeChainChangeEvent will report the changes happened to block chain, the changes will be
// delivered through the channel
func (s *Ethereum) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return s.APIBackend.SubscribeChainChangeEvent(ch)
}

func (s *Ethereum) GetBlockByHash(blockHash common.Hash) (*types.Block, error) {
	return s.APIBackend.GetBlock(context.Background(), blockHash)
}

func (s *Ethereum) ChainConfig() *params.ChainConfig {
	return s.APIBackend.ChainConfig()
}

func (s *Ethereum) CurrentBlock() *types.Block {
	return s.APIBackend.CurrentBlock()
}

func (s *Ethereum) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return s.APIBackend.SendTx(ctx, signedTx)
}

func (s *Ethereum) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return s.APIBackend.SuggestPrice(ctx)
}

func (s *Ethereum) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return s.APIBackend.GetPoolNonce(ctx, addr)
}

func (s *Ethereum) GetBlockByNumber(number uint64) (*types.Block, error) {
	return s.APIBackend.BlockByNumber(context.Background(), rpc.BlockNumber(number))
}
