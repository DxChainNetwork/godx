// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"encoding/hex"
	"fmt"
	"github.com/DxChainNetwork/godx/p2p/enode"

	"github.com/DxChainNetwork/godx/storage"
)

// PublicStorageHostManagerAPI defines the object used to call eligible public
// APIs that are used to acquire storage host information
type PublicStorageHostManagerAPI struct {
	shm *StorageHostManager
}

// NewPublicStorageHostManagerAPI initialize PublicStorageHostManagerAPI object
// which implemented a bunch of API methods
func NewPublicStorageHostManagerAPI(shm *StorageHostManager) *PublicStorageHostManagerAPI {
	return &PublicStorageHostManagerAPI{
		shm: shm,
	}
}

// ActiveStorageHosts returns active storage host information
func (api *PublicStorageHostManagerAPI) ActiveStorageHosts() (activeStorageHosts []storage.HostInfo) {
	allHosts := api.shm.storageHostTree.All()
	// based on the host information, filter out active hosts
	for _, host := range allHosts {
		numScanRecords := len(host.ScanRecords)
		if numScanRecords == 0 {
			continue
		}
		if !host.ScanRecords[numScanRecords-1].Success {
			continue
		}
		if !host.AcceptingContracts {
			continue
		}
		activeStorageHosts = append(activeStorageHosts, host)
	}
	return
}

// AllStorageHosts will return all storage hosts information stored from the storage host pool
func (api *PublicStorageHostManagerAPI) AllStorageHosts() (allStorageHosts []storage.HostInfo) {
	return api.shm.storageHostTree.All()
}

// StorageHost will return a specific host detailed information from the storage host pool
func (api *PublicStorageHostManagerAPI) StorageHost(id string) storage.HostInfo {
	var enodeid enode.ID

	// convert the hex string back to the enode.ID type
	idSlice, err := hex.DecodeString(id)
	if err != nil {
		return storage.HostInfo{}
	}
	copy(enodeid[:], idSlice)

	// get the storage host information based on the enode id
	info, exist := api.shm.storageHostTree.RetrieveHostInfo(enodeid)

	if !exist {
		return storage.HostInfo{}
	}
	return info
}

// StorageHostRanks will return the storage host rankings based on their evaluations. The
// higher the evaluation is, the higher order it will be placed
func (api *PublicStorageHostManagerAPI) StorageHostRanks() (rankings []StorageHostRank) {
	allHosts := api.shm.storageHostTree.All()
	// based on the host information, calculate the evaluation
	for _, host := range allHosts {
		eval := api.shm.evalFunc(host)

		rankings = append(rankings, StorageHostRank{
			EvaluationDetail: eval.EvaluationDetail(eval.Evaluation(), false, false),
			EnodeID:          host.EnodeID.String(),
		})
	}

	return
}

// PublicHostManagerDebugAPI defines the object used to call eligible APIs
// that are used to perform testing
type PublicHostManagerDebugAPI struct {
	shm *StorageHostManager
}

// NewPublicStorageClientDebugAPI initialize PublicStorageClientDebugAPI object
// which implemented a bunch of debug API methods
func NewPublicStorageClientDebugAPI(shm *StorageHostManager) *PublicHostManagerDebugAPI {
	return &PublicHostManagerDebugAPI{
		shm: shm,
	}
}

// Online will be used to indicate if the local node is connected to the internet or not
// by checking the number of peers it connected width
func (api *PublicHostManagerDebugAPI) Online() bool {
	return api.shm.b.Online()
}

// Syncing will be used to indicate if the local node is currently syncing with the blockchain
func (api *PublicHostManagerDebugAPI) Syncing() bool {
	return api.shm.b.Syncing()
}

// BlockHeight will be used to retrieve the current block height stored in the
// storage host manager data structure. If everything function correctly, the
// block height it returned should be same as the blockheight it synced
func (api *PublicHostManagerDebugAPI) BlockHeight() uint64 {
	return api.shm.blockHeight
}

// InsertHostInfo will insert host information into the storage host tree
// all host information are generated randomly
func (api *PublicHostManagerDebugAPI) InsertHostInfo(amount int) string {
	for i := 0; i < amount; i++ {
		hi := hostInfoGenerator()

		err := api.shm.insert(hi)
		if err != nil {
			return fmt.Sprintf("insert failed: %s", err.Error())
		}
	}
	return fmt.Sprintf("Successfully inserted %v Storage Host Information", amount)
}

// InsertActiveHostInfo will insert active host information into the storage host tree
// all host information are generated randomly. NOTE: if the information is not checked
// immediately, those active hosts will became inactive because of failing to establish
// connection
func (api *PublicHostManagerDebugAPI) InsertActiveHostInfo(amount int) string {
	for i := 0; i < amount; i++ {
		hi := activeHostInfoGenerator()
		err := api.shm.insert(hi)
		if err != nil {
			return fmt.Sprintf("insert failed: %s", err.Error())
		}
	}
	return fmt.Sprintf("Successfully inserted %v Active Storage Host Information", amount)
}
