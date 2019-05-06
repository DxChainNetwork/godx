// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

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

// StorageHosts returns all storage host information
func (api *PublicStorageHostManagerAPI) StorageHosts() string {
	return "WIP: working in progress"
}
