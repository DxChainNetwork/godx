// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"context"
	"math/big"
	"sort"
	"time"

	"io/ioutil"
	"path/filepath"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// ActiveContractAPI is used to re-format the contract information that is going to
// be displayed on the console
type ActiveContractsAPI struct {
	ID           storage.ContractID
	HostID       enode.ID
	AbleToUpload bool
	AbleToRenew  bool
	Canceled     bool
}

// Online will be used to indicate if the local node is connected to the internet
func (client *StorageClient) Online() bool {
	return client.info.NetInfo.PeerCount() > 0
}

// Syncing will be used to indicate if the local node is syncing with the blockchain
func (client *StorageClient) Syncing() bool {
	sync, _ := client.info.EthInfo.Syncing()
	syncing, ok := sync.(bool)
	if ok && !syncing {
		return false
	}

	return true
}

// GetTxByBlockHash will be used to get the detailed transaction by using the block hash
func (client *StorageClient) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	block, err := client.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

// GetStorageHostSetting will be used to get the storage host's external setting based on the
// peerID provided
func (client *StorageClient) GetStorageHostSetting(hostEnodeUrl string, config *storage.HostExtConfig) error {
	return client.ethBackend.GetStorageHostSetting(hostEnodeUrl, config)
}

// SubscribeChainChangeEvent will be used to get block information every time a change happened
// in the blockchain
func (client *StorageClient) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return client.ethBackend.SubscribeChainChangeEvent(ch)
}

// GetStorageHostManager will be used to acquire the storage host manager
func (client *StorageClient) GetStorageHostManager() *storagehostmanager.StorageHostManager {
	return client.storageHostManager
}

// DirInfo returns the Directory Information of the dxdir
func (client *StorageClient) DirInfo(dxPath storage.DxPath) (storage.DirectoryInfo, error) {
	entry, err := client.fileSystem.DirSet().Open(dxPath)
	if err != nil {
		return storage.DirectoryInfo{}, err
	}
	defer entry.Close()
	// Grab the health information and return the Directory Info, the worst
	// health will be returned. Depending on the directory and its contents that
	// could either be health or stuckHealth
	metadata := entry.Metadata()
	return storage.DirectoryInfo{
		NumFiles:         metadata.NumFiles,
		NumStuckSegments: metadata.NumStuckSegments,
		TotalSize:        metadata.TotalSize,
		Health:           metadata.Health,
		StuckHealth:      metadata.StuckHealth,
		MinRedundancy:    metadata.MinRedundancy,

		TimeLastHealthCheck: time.Unix(int64(metadata.TimeLastHealthCheck), 0),
		TimeModify:          time.Unix(int64(metadata.TimeModify), 0),
		DxPath:              metadata.DxPath,
	}, nil
}

// DirList get directories and files in the dxdir
func (client *StorageClient) DirList(dxPath storage.DxPath) ([]storage.DirectoryInfo, []storage.UploadFileInfo, error) {
	if err := client.tm.Add(); err != nil {
		return nil, nil, err
	}
	defer client.tm.Done()

	var dirs []storage.DirectoryInfo
	var files []storage.UploadFileInfo
	// Get DirectoryInfo
	di, err := client.DirInfo(dxPath)
	if err != nil {
		return nil, nil, err
	}
	dirs = append(dirs, di)
	// Read Directory
	fileInfos, err := ioutil.ReadDir(string(dxPath.SysPath(storage.SysPath(client.staticFilesDir))))
	if err != nil {
		return nil, nil, err
	}
	for _, fi := range fileInfos {
		// Check for directories
		if fi.IsDir() {
			dirDxPath, err := dxPath.Join(fi.Name())
			if err != nil {
				return nil, nil, err
			}
			di, err := client.DirInfo(dirDxPath)
			if err != nil {
				return nil, nil, err
			}
			dirs = append(dirs, di)
			continue
		}
		ext := filepath.Ext(fi.Name())
		if ext != storage.DxFileExt {
			continue
		}
		files = append(files, storage.UploadFileInfo{})
	}
	return dirs, files, nil
}

func (client *StorageClient) SetupConnection(enodeURL string, opCode storage.OpCode) (storage.Peer, error) {
	return client.ethBackend.SetupConnection(enodeURL, opCode)
}

func (client *StorageClient) AccountManager() *accounts.Manager {
	return client.ethBackend.AccountManager()
}

func (client *StorageClient) Disconnect(session *storage.Session, hostEnodeUrl string) error {
	return client.ethBackend.Disconnect(session, hostEnodeUrl)
}

func (client *StorageClient) ChainConfig() *params.ChainConfig {
	return client.ethBackend.ChainConfig()
}

func (client *StorageClient) CurrentBlock() *types.Block {
	return client.ethBackend.CurrentBlock()
}

func (client *StorageClient) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return client.ethBackend.SendTx(ctx, signedTx)
}

func (client *StorageClient) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return client.ethBackend.SuggestPrice(ctx)
}

func (client *StorageClient) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return client.ethBackend.GetPoolNonce(ctx, addr)
}

// GetFileSystem will get the file system
func (client *StorageClient) GetFileSystem() *filesystem.FileSystem {
	return client.fileSystem
}

// send contract create tx
func (client *StorageClient) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return client.info.StorageTx.SendContractCreateTX(clientAddr, input)
}

// calculate the proof ranges that should be used to verify a
// pre-modification Merkle diff proof for the specified actions.
func CalculateProofRanges(actions []storage.UploadAction, oldNumSectors uint64) []merkle.SubTreeLimit {
	newNumSectors := oldNumSectors
	sectorsChanged := make(map[uint64]struct{})
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			sectorsChanged[newNumSectors] = struct{}{}
			newNumSectors++
		}
	}

	oldRanges := make([]merkle.SubTreeLimit, 0, len(sectorsChanged))
	for sectorNum := range sectorsChanged {
		if sectorNum < oldNumSectors {
			oldRanges = append(oldRanges, merkle.SubTreeLimit{
				Left:  sectorNum,
				Right: sectorNum + 1,
			})
		}
	}
	sort.Slice(oldRanges, func(i, j int) bool {
		return oldRanges[i].Left < oldRanges[j].Left
	})

	return oldRanges
}

// modify the proof ranges produced by calculateProofRanges
// to verify a post-modification Merkle diff proof for the specified actions.
func ModifyProofRanges(proofRanges []merkle.SubTreeLimit, actions []storage.UploadAction, numSectors uint64) []merkle.SubTreeLimit {
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			proofRanges = append(proofRanges, merkle.SubTreeLimit{
				Left:  numSectors,
				Right: numSectors + 1,
			})
			numSectors++
		}
	}
	return proofRanges
}

// modify the leaf hashes of a Merkle diff proof to verify a
// post-modification Merkle diff proof for the specified actions.
func ModifyLeaves(leafHashes []common.Hash, actions []storage.UploadAction, numSectors uint64) []common.Hash {
	for _, action := range actions {
		switch action.Type {
		case storage.UploadActionAppend:
			leafHashes = append(leafHashes, merkle.Sha256MerkleTreeRoot(action.Data))
		}
	}
	return leafHashes
}
