// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
)

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

// Fund returns the current payment settings
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

// SetClientSetting will configure the client setting based on the user input data
func (api *PrivateStorageClientAPI) SetClientSetting(settings map[string]string) (resp string) {
	prevClientSetting := api.sc.RetrieveClientSetting()
	clientSetting, err := parseClientSetting(settings, prevClientSetting)
	if err != nil {
		resp = fmt.Sprintf("form contract failed, failed to parse the client settings: %s", err.Error())
	}

	// validation, for any 0 value, set them to default value
	clientSetting = clientSettingValidation(clientSetting)

	// call set client setting methods
	if err := api.sc.SetClientSetting(clientSetting); err != nil {
		resp = fmt.Sprintf("form contract failed, failed to set the client settings: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("successfully set client setting with value: %v, contracts will be formed automatically.",
		clientSetting)
	return
}
