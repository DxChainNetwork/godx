// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import "github.com/DxChainNetwork/godx/storage"

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

// AllStorageHosts will return all storage hosts information stored in the storage host pool
func (api *PublicStorageHostManagerAPI) AllStorageHosts() (allStorageHosts []storage.HostInfo) {
	return api.shm.storageHostTree.All()
}

// TODO: (mzhang) search based on the public key

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

// TODO: (mzhang) private method, set filter mode
