// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"encoding/hex"
	"fmt"
	"time"

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
	return api.shm.ActiveStorageHosts()
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
	return api.shm.StorageHostRanks()
}

// FilterMode will return the current storage host manager filter mode setting
func (api *PublicStorageHostManagerAPI) FilterMode() (fm string) {
	return api.shm.RetrieveFilterMode()
}

// FilteredHosts will return hosts stored in the filtered host tree
func (api *PublicStorageHostManagerAPI) FilteredHosts() (allFiltered []storage.HostInfo) {
	return api.shm.filteredTree.All()
}

// PrivateStorageHostManagerAPI defines the object used to call eligible APIs
// that are used to configure settings
type PrivateStorageHostManagerAPI struct {
	shm *StorageHostManager
}

// NewPrivateStorageHostManagerAPI initialize PrivateStorageHostManagerAPI object
// which implemented a bunch of API methods
func NewPrivateStorageHostManagerAPI(shm *StorageHostManager) *PrivateStorageHostManagerAPI {
	return &PrivateStorageHostManagerAPI{
		shm: shm,
	}
}

// SetFilterMode will be used to change the current storage host manager
// filter mode settings. There are total of 3 filter modes available
func (api *PrivateStorageHostManagerAPI) SetFilterMode(fm string, hostInfos []enode.ID) (resp string, err error) {
	var filterMode FilterMode
	if filterMode, err = ToFilterMode(fm); err != nil {
		err = fmt.Errorf("failed to set the filter mode: %s", err.Error())
		return
	}

	if err = api.shm.SetFilterMode(filterMode, hostInfos); err != nil {
		err = fmt.Errorf("failed to set the filter mode: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("the filter mode has been successfully set to %s", fm)
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
	return api.shm.getBlockHeight()
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

func (api *PublicHostManagerDebugAPI) InsertGivenHostInfo(hi storage.HostInfo) error {
	return api.shm.insert(hi)
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

// InsertHostInfoIPTime is used for the test case in contractManager module
func (api *PublicHostManagerDebugAPI) InsertHostInfoIPTime(amount int, id enode.ID, ip string, ipChanged time.Time) (err error) {
	for i := 0; i < amount; i++ {
		hi := hostInfoGeneratorIPID(ip, id, ipChanged)
		if err = api.shm.insert(hi); err != nil {
			return
		}
	}
	return
}

// InsertHostInfoHighEval is used for the test case in contractManager module, which is used
// to insert contract with higher evaluation
func (api *PublicHostManagerDebugAPI) InsertHostInfoHighEval(id enode.ID) (err error) {
	hi := hostInfoGeneratorHighEvaluation(id)
	return api.shm.insert(hi)
}

// InsertHostInfoLowEval is used for the test case in contractManager module, which is used
// to insert contract with lower evaluation (evaluation that is smaller than 1)
func (api *PublicHostManagerDebugAPI) InsertHostInfoLowEval(id enode.ID) (err error) {
	hi := hostInfoGeneratorLowEvaluation(id)
	return api.shm.insert(hi)
}

// RetrieveRentPaymentInfo is used to get the rentPayment settings, which is used for debugging
// purposes
func (api *PublicHostManagerDebugAPI) RetrieveRentPaymentInfo() (rentPayment storage.RentPayment) {
	return api.shm.RetrieveRentPayment()
}
