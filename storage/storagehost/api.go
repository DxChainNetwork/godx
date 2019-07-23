package storagehost

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
)

// HostPublicAPI is the api for private usage
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
func (h *HostPrivateAPI) GetHostConfig() storage.HostIntConfig {
	return h.storageHost.getInternalConfig()
}

// GetFinancialMetrics get the financial metrics of the host
func (h *HostPrivateAPI) GetFinancialMetrics() HostFinancialMetrics {
	return h.storageHost.getFinancialMetrics()
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

// host setters
//var hostSetterCallbacks = map[string]func(*StorageHost, string) error{
//	"acceptingContracts": (*StorageHost).SetAcceptContracts,
//}

// SetAcceptingContracts set host AcceptingContracts to value
func (h *HostPrivateAPI) SetAcceptingContracts(valStr string) (string, error) {
	val, err := unit.ParseBool(valStr)
	if err != nil {
		return "", fmt.Errorf("invalid bool string: %v", err)
	}
	if err := h.storageHost.setAcceptContracts(val); err != nil {
		return "", err
	}
	return fmt.Sprintf("successully set accepting contracts to %v", val), nil
}

// SetMaxDownloadBatchSize set host MaxDownloadBatchSize to value
func (h *StorageHost) SetMaxDownloadBatchSize(valStr string) (string, error) {
	val, err := unit.ParseStorage(valStr)
	if err != nil {
		return "", fmt.Errorf("invalid storage string: %v", err)
	}
	if err := h.setMaxDownloadBatchSize(val); err != nil {
		return "", fmt.Errorf("cannot set MaxDownloadBatchSize: %v", err)
	}
	return fmt.Sprintf("successully set MaxDownloadBatchSize to %v", val), nil
}

// SetMaxDuration set host MaxDuration to value
func (h *StorageHost) SetMaxDuration(str string) (string, error) {
	val, err := unit.ParseTime(str)
	if err != nil {
		return "", fmt.Errorf("invalid time string: %v", err)
	}
	if err := h.setMaxDuration(val); err != nil {
		return "", fmt.Errorf("cannot set MaxDuration: %v", err)
	}
	return fmt.Sprintf("successully set MaxDuration to %v", str), nil
}

// SetMaxReviseBatchSize set host MaxReviseBatchSize to value
func (h *StorageHost) SetMaxReviseBatchSize(str string) (string, error) {
	val, err := unit.ParseStorage(str)
	if err != nil {
		return "", fmt.Errorf("invalid size string: %v", err)
	}
	if err := h.setMaxReviseBatchSize(val); err != nil {
		return "", fmt.Errorf("cannot set MaxReviseBatchSize: %v", err)
	}
	return fmt.Sprintf("successully set MaxReviseBatchSize to %v", val), nil
}

// SetWindowSize set host WindowSize to value
func (h *StorageHost) SetWindowSize(str string) (string, error) {
	val, err := unit.ParseTime(str)
	if err != nil {
		return "", fmt.Errorf("invalid time duration string: %v", err)
	}
	if err := h.setWindowSize(val); err != nil {
		return "", fmt.Errorf("cannot set WindowSize: %v", err)
	}
	return fmt.Sprintf("successully set WindowSize to %v", val), nil
}

//SetPaymentAddress configure the account address used to sign the storage contract, which has and can only be the address of the local wallet.
func (h *StorageHost) SetPaymentAddress(addrStr string) (string, error) {
	addr := common.HexToAddress(addrStr)
	if err := h.setPaymentAddress(addr); err != nil {
		return "", fmt.Errorf("cannot set payment address: %v", err)
	}
	return "successfully set the payment address", nil
}

// SetDeposit set host Deposit to value.
func (h *StorageHost) SetDeposit(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setDeposit(wei); err != nil {
		return "", fmt.Errorf("cannot set Deposit: %v", err)
	}
	return fmt.Sprintf("successully set Deposit to %v", str), nil
}

// SetDepositBudget set host DepositBudget to value
func (h *HostPrivateAPI) SetDepositBudget(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.storageHost.setDepositBudget(wei); err != nil {
		return "", fmt.Errorf("cannot set DepositBudget: %v", err)
	}
	return fmt.Sprintf("successully set DepositBudget to %v", str), nil
}

// SetMaxDeposit set host MaxDeposit to value
func (h *StorageHost) SetMaxDeposit(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMaxDeposit(wei); err != nil {
		return "", fmt.Errorf("cannot set MaxDeposit: %v", err)
	}
	return fmt.Sprintf("successully set MaxDeposit to %v", str), nil
}

// SetMinBaseRPCPrice set host MinBaseRPCPrice to value
func (h *StorageHost) SetMinBaseRPCPrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinBaseRPCPrice(wei); err != nil {
		return "", fmt.Errorf("cannot set MinBaseRPCPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinBaseRPCPrice to %v", str), nil
}

// SetMinContractPrice set host MinContractPrice to value
func (h *StorageHost) SetMinContractPrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinContractPrice(wei); err != nil {
		return "", fmt.Errorf("cannot set MinContractPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinContractPrice to %v", str), nil
}

// SetMinDownloadBandwidthPrice set host MinDownloadBandwidthPrice to value
func (h *StorageHost) SetMinDownloadBandwidthPrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinDownloadBandwidthPrice(wei); err != nil {
		return "", fmt.Errorf("cannot set MinDownloadBandwidthPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinDownloadBandwidthPrice to %v", str), nil
}

// SetMinSectorAccessPrice set host MinSectorAccessPrice to value
func (h *StorageHost) SetMinSectorAccessPrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinSectorAccessPrice(wei); err != nil {
		return "", fmt.Errorf("cannot set MinSectorAccessPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinSectorAccessPrice to %v", str), nil
}

// SetMinStoragePrice set host MinStoragePrice to value
func (h *StorageHost) SetMinStoragePrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinStoragePrice(wei); err != nil {
		return "", fmt.Errorf("cannot set MinStoragePrice: %v", err)
	}
	return fmt.Sprintf("successully set MinStoragePrice to %v", str), nil
}

// SetMinUploadBandwidthPrice set host MinUploadBandwidthPrice to value
func (h *StorageHost) SetMinUploadBandwidthPrice(str string) (string, error) {
	wei, err := unit.ParseCurrency(str)
	if err != nil {
		return "", fmt.Errorf("invalid currency expression: %v", err)
	}
	if err := h.setMinUploadBandwidthPrice(wei); err != nil {
		return "", fmt.Errorf("cannot set WindowSize: %v", err)
	}
	return fmt.Sprintf("successully set WindowSize to %v", str), nil
}
