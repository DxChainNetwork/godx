// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
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
func (api *PublicStorageClientAPI) StorageClientSetting() (setting storage.ClientSettingAPIDisplay) {
	return formatClientSetting(api.sc.RetrieveClientSetting())
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

	// if user entered any 0s for the rent payment, set them to the default rentPayment settings
	currentSetting = clientSettingGetDefault(currentSetting)

	// call set client setting methods
	if err = api.sc.SetClientSetting(currentSetting); err != nil {
		err = fmt.Errorf("failed to set the client settings: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("Successfully set the storage client setting, you can use storageclient.setting() to verify")

	return
}

// CancelAllContracts will cancel all contracts signed with storage client by
// marking all active contracts as canceled, not good for uploading, and not good
// for renewing
func (api *PrivateStorageClientAPI) CancelAllContracts() (resp string) {
	if err := api.sc.CancelContracts(); err != nil {
		resp = fmt.Sprintf("Failed to cancel all contracts: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("All contracts are successfully canceled")
	return resp
}

// ActiveContracts will retrieve all active contracts and display their general information
func (api *PrivateStorageClientAPI) ActiveContracts() (activeContracts []ActiveContractsAPI) {
	activeContracts = api.sc.ActiveContracts()
	return
}

// ContractDetail will retrieve detailed contract information
func (api *PrivateStorageClientAPI) ContractDetail(contractID storage.ContractID) (detail storage.ContractMetaData, err error) {
	contract, exists := api.sc.ContractDetail(contractID)
	if !exists {
		err = fmt.Errorf("the contract with %v does not exist", contractID)
		return
	}

	detail = contract
	return
}

// PublicStorageClientDebugAPI defines the object used to call eligible public APIs
// that are used to mock data
type PublicStorageClientDebugAPI struct {
	sc *StorageClient
}

// NewPublicStorageClientDebugAPI initialize NewPublicStorageClientDebugAPI object
// which implemented a bunch of API methods
func NewPublicStorageClientDebugAPI(sc *StorageClient) *PublicStorageClientDebugAPI {
	return &PublicStorageClientDebugAPI{sc}
}

// InsertActiveContracts will create some random contracts based on the amount user entered
// and inserted them into activeContracts field
func (api *PublicStorageClientDebugAPI) InsertActiveContracts(amount int) (resp string, err error) {
	// validate user input
	if amount <= 0 {
		err = fmt.Errorf("the amount you entered %v must be greater than 0", amount)
		return
	}

	// insert random active contracts
	if err = api.sc.contractManager.InsertRandomActiveContracts(amount); err != nil {
		err = fmt.Errorf("failed to insert mocked active contracts: %s", err.Error())
		return
	}

	resp = fmt.Sprintf("Successfully inserted %v mocked active contracts", amount)
	return
}
