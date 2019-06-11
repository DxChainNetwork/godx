// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"reflect"
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

// StorageClientSetting will retrieve the current storage client settings
func (api *PublicStorageClientAPI) StorageClientSetting() (setting storage.ClientSetting) {
	return api.sc.RetrieveClientSetting()
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

// SetMemoryLimit allows user to expand or shrink the current memory limit
func (api *PrivateStorageClientAPI) SetMemoryLimit(amount uint64) string {
	return api.sc.memoryManager.SetMemoryLimit(amount)
}

// SetClientSetting will configure the client setting based on the user input data
func (api *PrivateStorageClientAPI) SetClientSetting(settings map[string]string) (resp string, err error) {
	prevClientSetting := api.sc.RetrieveClientSetting()
	var currentSetting storage.ClientSetting

	if currentSetting, err = parseClientSetting(settings, prevClientSetting); err != nil {
		err = fmt.Errorf("form contract failed, failed to parse the client settings: %s", err.Error())
		return
	}

	// if user did not enter anything, set the current setting to the default one
	if reflect.DeepEqual(currentSetting.RentPayment, storage.RentPayment{}) {
		currentSetting = clientSettingGetDefault(currentSetting)
	}

	// call set client setting methods
	if err = api.sc.SetClientSetting(currentSetting); err != nil {
		err = fmt.Errorf("failed to set the client settings: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("Successfully set the storage client setting, you can use storageclient.setting() to verify")

	return
}
