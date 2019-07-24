// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// ActiveContractsAPIDisplay is used to re-format the contract information that is going to
// be displayed on the console
type ActiveContractsAPIDisplay struct {
	ContractID   string
	HostID       string
	AbleToUpload bool
	AbleToRenew  bool
	Canceled     bool
}

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

// Config will retrieve the current storage client settings
func (api *PublicStorageClientAPI) Config() (setting storage.ClientSettingAPIDisplay) {
	return formatClientSetting(api.sc.RetrieveClientSetting())
}

// Hosts will retrieve the current storage hosts from the storage host manager
func (api *PublicStorageClientAPI) Hosts() (hosts []storage.HostInfo) {
	return api.sc.storageHostManager.AllHosts()
}

// Host will retrieve a specific storage host information from the storage host manager
// based on the host id
func (api *PublicStorageClientAPI) Host(id string) (host storage.HostInfo, err error) {
	var enodeid enode.ID

	// convert the hex string back to the enode.ID type
	idSlice, err := hex.DecodeString(id)
	if err != nil {
		return storage.HostInfo{}, errors.New("the hostID provided is not valid")
	}
	copy(enodeid[:], idSlice)

	// get the storage host information based on the enode id
	info, exist := api.sc.storageHostManager.RetrieveHostInfo(enodeid)

	if !exist {
		return storage.HostInfo{}, errors.New("the host you are looking for does not exist")
	}
	return info, nil
}

// HostRank will retrieve the rankings of the storage hosts. The ranking information also
// includes detailed evaluation break down
func (api *PublicStorageClientAPI) HostRank() (evaluation []storagehostmanager.StorageHostRank) {
	return api.sc.storageHostManager.StorageHostRanks()
}

// Contracts will retrieve all active contracts and display their general information
func (api *PublicStorageClientAPI) Contracts() (activeContracts []ActiveContractsAPIDisplay) {
	activeContracts = api.sc.ActiveContracts()
	return
}

// Contract will retrieve detailed contract information
func (api *PublicStorageClientAPI) Contract(contractID string) (detail ContractMetaDataAPIDisplay, err error) {
	// convert the string into contractID format
	var convertContractID storage.ContractID
	if convertContractID, err = storage.StringToContractID(contractID); err != nil {
		err = fmt.Errorf("the contract id provided is invalid: %s", err.Error())
		return
	}

	// get the contract detail
	contract, exists := api.sc.ContractDetail(convertContractID)
	if !exists {
		err = fmt.Errorf("the contract with %v does not exist", contractID)
		return
	}

	// format the contract meta data
	detail = formatContractMetaData(contract)

	return
}

// PaymentAddress get the account address used to sign the storage contract. If not configured, the first address in the local wallet will be used as the paymentAddress by default.
func (api *PublicStorageClientAPI) PaymentAddress() (common.Address, error) {
	return api.sc.GetPaymentAddress()
}

// DownloadSync is used to download remote file by sync mode
// NOTE: RPC not support async download, because it is stateless, should block until download task done.
func (api *PublicStorageClientAPI) DownloadSync(remoteFilePath, localPath string) (string, error) {
	p := storage.DownloadParameters{
		// where to write the downloaded files
		WriteToLocalPath: localPath,

		// where to download the remote file
		RemoteFilePath: remoteFilePath,
	}
	err := api.sc.DownloadSync(p)
	if err != nil {
		return "【ERROR】failed to download", err
	}
	return "File downloaded successfully", nil
}

// Upload their local files to hosts made contract with
func (api *PublicStorageClientAPI) Upload(source string, dxPath string) (string, error) {
	path, err := storage.NewDxPath(dxPath)
	if err != nil {
		return "", err
	}
	param := storage.FileUploadParams{
		Source: source,
		DxPath: path,
		Mode:   storage.Override,
	}
	if err := api.sc.Upload(param); err != nil {
		return "", err
	}
	return "success", nil
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

// SetConfig will configure the client setting based on the user input data
func (api *PrivateStorageClientAPI) SetConfig(settings map[string]string) (resp string, err error) {
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

	resp = fmt.Sprintf("Successfully set the storage client setting")

	return
}

// SetPaymentAddress configure the account address used to sign the storage contract, which has and can only be the address of the local wallet.
func (api *PrivateStorageClientAPI) SetPaymentAddress(addrStr string) bool {
	paymentAddress := common.HexToAddress(addrStr)

	account := accounts.Account{Address: paymentAddress}
	_, err := api.sc.ethBackend.AccountManager().Find(account)
	if err != nil {
		api.sc.log.Error("You must set up an account owned by your local wallet!")
		return false
	}

	api.sc.lock.Lock()
	api.sc.PaymentAddress = paymentAddress
	api.sc.lock.Unlock()

	return true
}
