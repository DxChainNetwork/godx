// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
)

// HostPrivateAPI is the api for private usage
type HostPrivateAPI struct {
	storageHost *StorageHost
}

// NewHostPrivateAPI is the api to create the host private api
func NewHostPrivateAPI(storageHost *StorageHost) *HostPrivateAPI {
	return &HostPrivateAPI{
		storageHost: storageHost,
	}
}

// Version gives a mock version of the debugapi
func (h *HostPrivateAPI) Version() string {
	return Version
}

// SectorSize return the sector size as a basic storage unit of the storage system.
func (h *HostPrivateAPI) SectorSize() uint64 {
	return storage.SectorSize
}

// PersistDir print the persist directory of the host
func (h *HostPrivateAPI) PersistDir() string {
	return h.storageHost.getPersistDir()
}

// Announce set accepting contracts to true, and then send the announcement
// transaction
func (h *HostPrivateAPI) Announce() string {
	if err := h.storageHost.setAcceptContracts(true); err != nil {
		return fmt.Sprintf("cannot set AcceptingContracts: %v", err)
	}
	address, err := h.storageHost.getPaymentAddress()
	if err != nil {
		return fmt.Sprintf("cannot get the payment address: %v", err)
	}
	hash, err := h.storageHost.parseAPI.StorageTx.SendHostAnnounceTX(address)
	if err != nil {
		return fmt.Sprintf("cannot send the announce transaction: %v", err)
	}
	return fmt.Sprintf("Announcement transaction: %v", hash.Hex())
}

// Folders return all the folders
func (h *HostPrivateAPI) Folders() []storage.HostFolder {
	return h.storageHost.StorageManager.Folders()
}

// AvailableSpace return the available spaces of the host
func (h *HostPrivateAPI) AvailableSpace() storage.HostSpace {
	return h.storageHost.StorageManager.AvailableSpace()
}

// GetHostConfig return the internal settings of the storage host
func (h *HostPrivateAPI) GetHostConfig() storage.HostIntConfigForDisplay {
	// Get the internal setting
	config := h.storageHost.getInternalConfig()
	// parse the numbers to human readable string
	display := storage.HostIntConfigForDisplay{
		AcceptingContracts:     unit.FormatBool(config.AcceptingContracts),
		MaxDownloadBatchSize:   unit.FormatStorage(config.MaxDownloadBatchSize, false),
		MaxDuration:            unit.FormatTime(config.MaxDuration),
		MaxReviseBatchSize:     unit.FormatStorage(config.MaxReviseBatchSize, false),
		WindowSize:             unit.FormatTime(config.WindowSize),
		PaymentAddress:         config.PaymentAddress.String(),
		Deposit:                unit.FormatCurrency(config.Deposit, "/byte/block"),
		DepositBudget:          unit.FormatCurrency(config.DepositBudget, "/contract"),
		MaxDeposit:             unit.FormatCurrency(config.MaxDeposit),
		BaseRPCPrice:           unit.FormatCurrency(config.BaseRPCPrice),
		ContractPrice:          unit.FormatCurrency(config.ContractPrice, "/contract"),
		DownloadBandwidthPrice: unit.FormatCurrency(config.DownloadBandwidthPrice, "/byte"),
		SectorAccessPrice:      unit.FormatCurrency(config.SectorAccessPrice, "/sector"),
		StoragePrice:           unit.FormatCurrency(config.StoragePrice, "/byte/block"),
		UploadBandwidthPrice:   unit.FormatCurrency(config.UploadBandwidthPrice, "/byte"),
	}

	return display
}

// GetFinancialMetrics get the financial metrics of the host
func (h *HostPrivateAPI) GetFinancialMetrics() HostFinancialMetricsForDisplay {
	fm := h.storageHost.getFinancialMetrics()
	display := HostFinancialMetricsForDisplay{
		ContractCount:                     fm.ContractCount,
		ContractCompensation:              unit.FormatCurrency(fm.ContractCompensation),
		PotentialContractCompensation:     unit.FormatCurrency(fm.PotentialContractCompensation),
		LockedStorageDeposit:              unit.FormatCurrency(fm.LockedStorageDeposit),
		LostRevenue:                       unit.FormatCurrency(fm.LostRevenue),
		LostStorageDeposit:                unit.FormatCurrency(fm.LostStorageDeposit),
		PotentialStorageRevenue:           unit.FormatCurrency(fm.PotentialStorageRevenue),
		RiskedStorageDeposit:              unit.FormatCurrency(fm.RiskedStorageDeposit),
		StorageRevenue:                    unit.FormatCurrency(fm.StorageRevenue),
		TransactionFeeExpenses:            unit.FormatCurrency(fm.TransactionFeeExpenses),
		DownloadBandwidthRevenue:          unit.FormatCurrency(fm.DownloadBandwidthRevenue),
		PotentialDownloadBandwidthRevenue: unit.FormatCurrency(fm.PotentialDownloadBandwidthRevenue),
		PotentialUploadBandwidthRevenue:   unit.FormatCurrency(fm.PotentialUploadBandwidthRevenue),
		UploadBandwidthRevenue:            unit.FormatCurrency(fm.UploadBandwidthRevenue),
	}
	return display
}

//GetPaymentAddress get the account address used to sign the storage contract. If not configured, the first address in the local wallet will be used as the paymentAddress by default.
func (h *HostPrivateAPI) GetPaymentAddress() string {
	addr, err := h.storageHost.getPaymentAddress()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("Current address: %v", common.Bytes2Hex(addr[:]))
}

// AddStorageFolder add a storage folder with a specified size
func (h *HostPrivateAPI) AddStorageFolder(path string, sizeStr string) (string, error) {
	size, err := unit.ParseStorage(sizeStr)
	if err != nil {
		return "", err
	}
	err = h.storageHost.StorageManager.AddStorageFolder(path, size)
	if err != nil {
		return "", err
	}
	return "successfully added the storage folder", nil
}

// ResizeFolder resize the folder to specified size
func (h *HostPrivateAPI) ResizeFolder(folderPath string, sizeStr string) (string, error) {
	size, err := unit.ParseStorage(sizeStr)
	if err != nil {
		return "", err
	}
	err = h.storageHost.StorageManager.ResizeFolder(folderPath, size)
	if err != nil {
		return "", err
	}
	return "successfully resize the storage folder", nil
}

// DeleteFolder delete the folder
func (h *HostPrivateAPI) DeleteFolder(folderPath string) (string, error) {
	err := h.storageHost.StorageManager.DeleteFolder(folderPath)
	if err != nil {
		return "", err
	}
	return "successfully delete the storage folder", nil
}

// hostSetterCallbacks is the mapping from the field name to the setter function
var hostSetterCallbacks = map[string]func(*HostPrivateAPI, string) error{
	"acceptingContracts":     (*HostPrivateAPI).setAcceptingContracts,
	"maxDownloadBatchSize":   (*HostPrivateAPI).setMaxDownloadBatchSize,
	"maxDuration":            (*HostPrivateAPI).setMaxDuration,
	"maxReviseBatchSize":     (*HostPrivateAPI).setMaxReviseBatchSize,
	"paymentAddress":         (*HostPrivateAPI).setPaymentAddress,
	"deposit":                (*HostPrivateAPI).setDeposit,
	"depositBudget":          (*HostPrivateAPI).setDepositBudget,
	"maxDeposit":             (*HostPrivateAPI).setMaxDeposit,
	"baseRPCPrice":           (*HostPrivateAPI).setBaseRPCPrice,
	"contractPrice":          (*HostPrivateAPI).setContractPrice,
	"downloadBandwidthPrice": (*HostPrivateAPI).setDownloadBandwidthPrice,
	"sectorAccessPrice":      (*HostPrivateAPI).setSectorAccessPrice,
	"storagePrice":           (*HostPrivateAPI).setStoragePrice,
	"uploadBandwidthPrice":   (*HostPrivateAPI).setUploadBandwidthPrice,
}

// SetConfig set the config specified by a mapping of key value pair
func (h *HostPrivateAPI) SetConfig(config map[string]string) (string, error) {
	h.storageHost.lock.Lock()
	// record the previous config and register the defer function
	var err error
	prevConfig := h.storageHost.config
	defer func() {
		// If error happened, revert to the previous config
		if err != nil {
			h.storageHost.config = prevConfig
		}
		h.storageHost.lock.Unlock()
	}()

	// Loops over the user set config and change the host settings
	for key, value := range config {
		callback, exist := hostSetterCallbacks[key]
		if !exist {
			err = fmt.Errorf("unknown config variable")
			return "", err
		}
		if err = callback(h, value); err != nil {
			return "", err
		}
	}
	// sync the config
	if err = h.storageHost.syncConfig(); err != nil {
		return "", err
	}
	return "Successfully set the host config", nil
}

// setAcceptingContracts set host AcceptingContracts to val specified by valStr
func (h *HostPrivateAPI) setAcceptingContracts(valStr string) error {
	val, err := unit.ParseBool(valStr)
	if err != nil {
		return fmt.Errorf("invalid bool string: %v", err)
	}
	h.storageHost.config.AcceptingContracts = val
	return nil
}

// setMaxDownloadBatchSize set host MaxDownloadBatchSize to value
func (h *HostPrivateAPI) setMaxDownloadBatchSize(valStr string) error {
	val, err := unit.ParseStorage(valStr)
	if err != nil {
		return fmt.Errorf("invalid storage string: %v", err)
	}
	h.storageHost.config.MaxDownloadBatchSize = val
	return nil
}

// setMaxDuration set host MaxDuration to value
func (h *HostPrivateAPI) setMaxDuration(str string) error {
	val, err := unit.ParseTime(str)
	if err != nil {
		return fmt.Errorf("invalid time string: %v", err)
	}
	h.storageHost.config.MaxDuration = val
	return nil
}

// setMaxReviseBatchSize set host MaxReviseBatchSize to value
func (h *HostPrivateAPI) setMaxReviseBatchSize(str string) error {
	val, err := unit.ParseStorage(str)
	if err != nil {
		return fmt.Errorf("invalid size string: %v", err)
	}
	h.storageHost.config.MaxReviseBatchSize = val
	return nil
}

// setWindowSize set host WindowSize to value
func (h *HostPrivateAPI) setWindowSize(str string) error {
	val, err := unit.ParseTime(str)
	if err != nil {
		return fmt.Errorf("invalid time duration string: %v", err)
	}
	h.storageHost.config.WindowSize = val
	return nil
}

// setPaymentAddress configure the account address used to sign the storage contract,
// which has and can only be the address of the local wallet.
func (h *HostPrivateAPI) setPaymentAddress(addrStr string) error {
	addr := common.HexToAddress(addrStr)
	account := accounts.Account{Address: addr}
	if h.storageHost.am == nil {
		return errors.New("storage host has no account manager")
	}
	_, err := h.storageHost.am.Find(account)
	if err != nil {
		return errors.New("unknown account")
	}
	h.storageHost.config.PaymentAddress = addr
	return nil
}

// setDeposit set host Deposit to value.
func (h *HostPrivateAPI) setDeposit(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.Deposit = wei
	return nil
}

// setDepositBudget set host DepositBudget to value
func (h *HostPrivateAPI) setDepositBudget(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.DepositBudget = wei
	return nil
}

// setMaxDeposit set host MaxDeposit to value
func (h *HostPrivateAPI) setMaxDeposit(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.MaxDeposit = wei
	return nil
}

// setBaseRPCPrice set host BaseRPCPrice to value
func (h *HostPrivateAPI) setBaseRPCPrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.BaseRPCPrice = wei
	return nil
}

// setContractPrice set host ContractPrice to value
func (h *HostPrivateAPI) setContractPrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.ContractPrice = wei
	return nil
}

// setDownloadBandwidthPrice set host DownloadBandwidthPrice to value
func (h *HostPrivateAPI) setDownloadBandwidthPrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.DownloadBandwidthPrice = wei
	return nil
}

// setSectorAccessPrice set host SectorAccessPrice to value
func (h *HostPrivateAPI) setSectorAccessPrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.SectorAccessPrice = wei
	return nil
}

// setStoragePrice set host StoragePrice to value
func (h *HostPrivateAPI) setStoragePrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.StoragePrice = wei
	return nil
}

// setUploadBandwidthPrice set host UploadBandwidthPrice to value
func (h *HostPrivateAPI) setUploadBandwidthPrice(str string) error {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return fmt.Errorf("invalid currency expression: %v", err)
	}
	h.storageHost.config.UploadBandwidthPrice = wei
	return nil
}
