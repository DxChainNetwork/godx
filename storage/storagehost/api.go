package storagehost

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
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

// TODO: refactor the set methods to a map?
// SetAcceptingContracts set host AcceptingContracts to value
func (h *HostPrivateAPI) SetAcceptingContracts(val bool) string {
	if err := h.storageHost.setAcceptContracts(val); err != nil {
		return fmt.Sprintf("cannot set accepting contracts: %v", err)
	}
	return fmt.Sprintf("successully set accepting contracts to %v", val)
}

// SetMaxDownloadBatchSize set host MaxDownloadBatchSize to value
func (h *HostPrivateAPI) SetMaxDownloadBatchSize(val uint64) string {
	if err := h.storageHost.setMaxDownloadBatchSize(val); err != nil {
		return fmt.Sprintf("cannot set MaxDownloadBatchSize: %v", err)
	}
	return fmt.Sprintf("successully set MaxDownloadBatchSize to %v", val)
}

// SetMaxDuration set host MaxDuration to value
func (h *HostPrivateAPI) SetMaxDuration(str string) string {
	val, err := storage.ParseTime(str)
	if err != nil {
		return fmt.Sprintf("invalid time string: %v", err)
	}
	if err := h.storageHost.setMaxDuration(val); err != nil {
		return fmt.Sprintf("cannot set MaxDuration: %v", err)
	}
	return fmt.Sprintf("successully set MaxDuration to %v", val)
}

// SetMaxReviseBatchSize set host MaxReviseBatchSize to value
func (h *HostPrivateAPI) SetMaxReviseBatchSize(val uint64) string {
	if err := h.storageHost.setMaxReviseBatchSize(val); err != nil {
		return fmt.Sprintf("cannot set MaxReviseBatchSize: %v", err)
	}
	return fmt.Sprintf("successully set MaxReviseBatchSize to %v", val)
}

// SetWindowSize set host WindowSize to value
func (h *HostPrivateAPI) SetWindowSize(val uint64) string {
	if err := h.storageHost.setWindowSize(val); err != nil {
		return fmt.Sprintf("cannot set WindowSize: %v", err)
	}
	return fmt.Sprintf("successully set WindowSize to %v", val)
}

//SetPaymentAddress configure the account address used to sign the storage contract, which has and can only be the address of the local wallet.
func (h *HostPrivateAPI) SetPaymentAddress(addrStr string) string {
	addr := common.HexToAddress(addrStr)
	if err := h.storageHost.setPaymentAddress(addr); err != nil {
		return fmt.Sprintf("cannot set payment address: %v", err)
	}
	return "successfully set the payment address"
}

// SetDeposit set host Deposit to value.
// TODO: explain the unit of the deposit
func (h *HostPrivateAPI) SetDeposit(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setDeposit(wei); err != nil {
		return fmt.Sprintf("cannot set Deposit: %v", err)
	}
	return fmt.Sprintf("successully set Deposit to %v", str)
}

// SetDepositBudget set host DepositBudget to value
func (h *HostPrivateAPI) SetDepositBudget(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setDepositBudget(wei); err != nil {
		return fmt.Sprintf("cannot set DepositBudget: %v", err)
	}
	return fmt.Sprintf("successully set DepositBudget to %v", str)
}

// SetMaxDeposit set host MaxDeposit to value
func (h *HostPrivateAPI) SetMaxDeposit(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMaxDeposit(wei); err != nil {
		return fmt.Sprintf("cannot set MaxDeposit: %v", err)
	}
	return fmt.Sprintf("successully set MaxDeposit to %v", str)
}

// SetMinBaseRPCPrice set host MinBaseRPCPrice to value
func (h *HostPrivateAPI) SetMinBaseRPCPrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinBaseRPCPrice(wei); err != nil {
		return fmt.Sprintf("cannot set MinBaseRPCPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinBaseRPCPrice to %v", str)
}

// SetMinContractPrice set host MinContractPrice to value
func (h *HostPrivateAPI) SetMinContractPrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinContractPrice(wei); err != nil {
		return fmt.Sprintf("cannot set MinContractPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinContractPrice to %v", str)
}

// SetMinDownloadBandwidthPrice set host MinDownloadBandwidthPrice to value
func (h *HostPrivateAPI) SetMinDownloadBandwidthPrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinDownloadBandwidthPrice(wei); err != nil {
		return fmt.Sprintf("cannot set MinDownloadBandwidthPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinDownloadBandwidthPrice to %v", str)
}

// SetMinSectorAccessPrice set host MinSectorAccessPrice to value
func (h *HostPrivateAPI) SetMinSectorAccessPrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinSectorAccessPrice(wei); err != nil {
		return fmt.Sprintf("cannot set MinSectorAccessPrice: %v", err)
	}
	return fmt.Sprintf("successully set MinSectorAccessPrice to %v", str)
}

// SetMinStoragePrice set host MinStoragePrice to value
func (h *HostPrivateAPI) SetMinStoragePrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinStoragePrice(wei); err != nil {
		return fmt.Sprintf("cannot set MinStoragePrice: %v", err)
	}
	return fmt.Sprintf("successully set MinStoragePrice to %v", str)
}

// SetMinUploadBandwidthPrice set host MinUploadBandwidthPrice to value
func (h *HostPrivateAPI) SetMinUploadBandwidthPrice(str string) string {
	wei, err := storage.ParseFund(str)
	if err != nil {
		return fmt.Sprintf("invalid fund expression: %v", err)
	}
	if err := h.storageHost.setMinUploadBandwidthPrice(wei); err != nil {
		return fmt.Sprintf("cannot set WindowSize: %v", err)
	}
	return fmt.Sprintf("successully set WindowSize to %v", str)
}

// AddStorageFolder add a storage folder with a specified size
func (h *HostPrivateAPI) AddStorageFolder(path string, sizeStr string) (string, error) {
	size, err := storage.ParseStorage(sizeStr)
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
	size, err := storage.ParseStorage(sizeStr)
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
