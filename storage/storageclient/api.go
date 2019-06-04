// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import "github.com/DxChainNetwork/godx/storage"

// PublicStorageClientAPI defines the object used to call eligible public APIs
// are used to acquire information
type PublicStorageClientAPI struct {
	sc *StorageClient
}

// NewPublicStorageClientAPI initialize PublicStorageClientAPI object
// which implemented a bunch of API methods
func NewPublicStorageClientAPI(sc *StorageClient) *PublicStorageClientAPI {
	return &PublicStorageClientAPI{sc}
}

// Payment returns the current payment settings
func (api *PublicStorageClientAPI) Payment() string {
	return "working in progress: getting payment information"
}

// MemoryAvailable returns current memory available
func (api *PublicStorageClientAPI) MemoryAvailable() uint64 {
	return api.sc.memoryManager.MemoryAvailable()
}

// MemoryLimit returns max memory allowed
func (api *PublicStorageClientAPI) MemoryLimit() uint64 {
	return api.sc.memoryManager.MemoryLimit()
}

// download remote file by sync mode
//
// NOTE: RPC not support async download, because it is one time connection, should block until download task done.
func (api *PublicStorageClientAPI) DownloadSync(length, offset uint64, destination, dxFilePath string) error {
	p := storage.ClientDownloadParameters{
		Async:  false,
		Length: length,
		Offset: offset,

		// where to write the downloaded files, need to specify from outer request
		// NOTE: can not get httpWriter from here，so choose the destination dir to write the downloaded files.
		Destination: destination,

		// where to download th remote file, need to specify from outer request
		DxFilePath: dxFilePath,
	}
	err := api.sc.DownloadSync(p)
	if err != nil {
		return err
	}
	return nil
}

// PrivateStorageClientAPI defines the object used to call eligible APIs
// that are used to configure settings
type PrivateStorageClientAPI struct {
	sc *StorageClient
}

// NewPrivateStorageClientAPI initialize PrivateStorageClientAPI object
// which implemented a bunch of API methods
func NewPrivateStorageClientAPI(sc *StorageClient) *PrivateStorageClientAPI {
	return &PrivateStorageClientAPI{sc}
}

// SetPayment allows user to configure storage payment settings
// which will be used to select eligible the StorageHost
func (api *PrivateStorageClientAPI) SetPayment() string {
	return "working in progress: setting payment information"
}

// SetMemoryLimit allows user to expand or shrink the current memory limit
func (api *PrivateStorageClientAPI) SetMemoryLimit(amount uint64) string {
	return api.sc.memoryManager.SetMemoryLimit(amount)
}
