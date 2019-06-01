// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"crypto/ecdsa"
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/internal/ethapi"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"io/ioutil"
	"path/filepath"
	"reflect"
)

// ParsedAPI will parse the APIs saved in the Ethereum
// and get the ones needed
type ParsedAPI struct {
	netInfo *ethapi.PublicNetAPI
	account *ethapi.PrivateAccountAPI
	ethInfo *ethapi.PublicEthereumAPI
}

// filterAPIs will filter the APIs saved in the Ethereum and
// save them into ParsedAPI data structure
func (sc *StorageClient) filterAPIs(apis []rpc.API) error {
	for _, api := range apis {
		switch typ := reflect.TypeOf(api.Service); typ {
		case reflect.TypeOf(&ethapi.PublicNetAPI{}):
			netAPI := api.Service.(*ethapi.PublicNetAPI)
			if netAPI == nil {
				return errors.New("failed to acquire netInfo information")
			}
			sc.info.netInfo = netAPI
		case reflect.TypeOf(&ethapi.PrivateAccountAPI{}):
			accountAPI := api.Service.(*ethapi.PrivateAccountAPI)
			if accountAPI == nil {
				return errors.New("failed to acquire account information")
			}
			sc.info.account = accountAPI
		case reflect.TypeOf(&ethapi.PublicEthereumAPI{}):
			ethAPI := api.Service.(*ethapi.PublicEthereumAPI)
			if ethAPI == nil {
				return errors.New("failed to acquire eth information")
			}
			sc.info.ethInfo = ethAPI
		default:
			continue
		}
	}
	return nil
}

// Online will be used to indicate if the local node is connected to the internet
func (sc *StorageClient) Online() bool {
	return sc.info.netInfo.PeerCount() > 0
}

// Syncing will be used to indicate if the local node is syncing with the blockchain
func (sc *StorageClient) Syncing() bool {
	sync, _ := sc.info.ethInfo.Syncing()
	syncing, ok := sync.(bool)
	if ok && !syncing {
		return false
	}

	return true
}

// GetTxByBlockHash will be used to get the detailed transaction by using the block hash
func (sc *StorageClient) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	block, err := sc.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return block.Transactions(), nil
}

// GetStorageHostSetting will be used to get the storage host's external setting based on the
// peerID provided
func (sc *StorageClient) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	return sc.ethBackend.GetStorageHostSetting(peerID, config)
}

// SubscribeChainChangeEvent will be used to get block information every time a change happened
// in the blockchain
func (sc *StorageClient) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return sc.ethBackend.SubscribeChainChangeEvent(ch)
}

// GetStorageHostManager will be used to acquire the storage host manager
func (sc *StorageClient) GetStorageHostManager() *storagehostmanager.StorageHostManager {
	return sc.storageHostManager
}
// DirInfo returns the Directory Information of the siadir
func (sc *StorageClient) DirInfo(dxPath dxdir.DxPath) (DirectoryInfo, error) {
	entry, err := sc.staticDirSet.Open(dxPath)
	if err != nil {
		return DirectoryInfo{}, err
	}
	defer entry.Close()
	// Grab the health information and return the Directory Info, the worst
	// health will be returned. Depending on the directory and its contents that
	// could either be health or stuckHealth
	metadata := entry.Metadata()
	return DirectoryInfo{
		NumFiles:metadata.NumFiles,
		NumStuckSegments:metadata.NumStuckSegments,
		TotalSize:metadata.TotalSize,
		Health:metadata.Health,
		StuckHealth:metadata.StuckHealth,
		MinRedundancy:metadata.MinRedundancy,
		TimeLastHealthCheck:metadata.TimeLastHealthCheck,
		TimeModify:metadata.TimeModify,
		DxPath:metadata.DxPath,
	}, nil
}

// DirList returns directories and files stored in the siadir as well as the
// DirectoryInfo of the siadir
func (sc *StorageClient) DirList(dxPath dxdir.DxPath) ([]DirectoryInfo, []FileInfo, error) {
	if err := sc.tm.Add(); err != nil {
		return nil, nil, err
	}
	defer sc.tm.Done()

	var dirs []DirectoryInfo
	var files []FileInfo
	// Get DirectoryInfo
	di, err := sc.DirInfo(dxPath)
	if err != nil {
		return nil, nil, err
	}
	dirs = append(dirs, di)
	// Read Directory
	fileInfos, err := ioutil.ReadDir(dxPath.DxDirSysPath(sc.staticFilesDir))
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
			di, err := sc.DirInfo(dirDxPath)
			if err != nil {
				return nil, nil, err
			}
			dirs = append(dirs, di)
			continue
		}
		// Ignore non siafiles
		ext := filepath.Ext(fi.Name())
		if ext != DxFileExtension {
			continue
		}
		// Grab dxfile
		// fileName := strings.TrimSuffix(fi.Name(), modules.SiaFileExtension)
		// fileSiaPath, err := dxPath.Join(fileName)
		//if err != nil {
		//	return nil, nil, err
		//}
		// TODO fileinfo
		//file, err := sc.File(fileSiaPath)
		//if err != nil {
		//	return nil, nil, err
		//}
		files = append(files, FileInfo{})
	}
	return dirs, files, nil
}

// TODO complete
func (sc *StorageClient) getClientHostHealthInfoTable(entries []*dxfile.FileSetEntryWithID) storage.HostHealthInfoTable {
	infoMap := make(storage.HostHealthInfoTable, 1)

	for _, e := range entries {
		var used []enode.ID
		for _, id := range e.HostIDs() {
			used = append(used, id)
		}

		if err := e.UpdateUsedHosts(used); err != nil {
			sc.log.Debug("Could not update used hosts:", err)
		}
	}

	return infoMap
}

// covert pubkey to hex string
func PubkeyToHex(pubkey *ecdsa.PublicKey) string {
	pubkeyBytes := crypto.FromECDSAPub(pubkey)
	pubkeyHex := hexutil.Encode(pubkeyBytes)
	return pubkeyHex
}
