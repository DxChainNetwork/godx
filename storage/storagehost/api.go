package storagehost

import (
	"fmt"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// HostDeBugAPI give developer a way for access and modify the setting file
//type HostDeBugAPI struct {
//	storagehost *StorageHost
//}

// HostPublicAPI is the api for private usage
type HostPrivateAPI struct {
	storageHost *StorageHost
}

// NewHostDebugAPI generate a HostDeBugAPI reference for caller
//func NewHostDebugAPI(storagehost *StorageHost) *HostDeBugAPI {
//	return &HostDeBugAPI{
//		storagehost: storagehost,
//	}
//}

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

// PersistDir print the persist directory of the host
func (h *HostPrivateAPI) PersistDir() string {
	return h.storageHost.getPersistDir()
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

func (h *HostPrivateAPI) GetFinancialMetrics() HostFinancialMetrics {
	return h.storageHost.getFinancialMetrics()
}

//GetPaymentAddress get the account address used to sign the storage contract. If not configured, the first address in the local wallet will be used as the paymentAddress by default.
func (h *HostPrivateAPI) GetPaymentAddress() string {
	addr, err := h.storageHost.getPaymentAddress()
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("Current address:\n%v", common.Bytes2Hex(addr[:]))
}

//SetPaymentAddress configure the account address used to sign the storage contract, which has and can only be the address of the local wallet.
func (h *HostPrivateAPI) SetPaymentAddress(paymentAddress common.Address) string {
	account := accounts.Account{Address: paymentAddress}
	_, err := h.storageHost.ethBackend.AccountManager().Find(account)
	if err != nil {
		return "You must set up an account owned by your local wallet!"
	}
	h.storageHost.lock.Lock()
	h.storageHost.config.PaymentAddress = paymentAddress
	h.storageHost.lock.Unlock()
	return "Payment address successfully set."
}

// SectorSize return the sector size as a basic storage unit of the storage system.
func (h *HostPrivateAPI) SectorSize() uint64 {
	return storage.SectorSize
}

// AddStorageFolder add a storage folder with a specified size
func (h *HostPrivateAPI) AddStorageFolder(path string, size uint64) string {
	err := h.storageHost.StorageManager.AddStorageFolder(path, size)
	if err != nil {
		return err.Error()
	}
	return "successfully added the storage folder"
}

// ResizeFolder resize the folder to specified size
func (h *HostPrivateAPI) ResizeFolder(folderPath string, size uint64) string {
	err := h.storageHost.StorageManager.ResizeFolder(folderPath, size)
	if err != nil {
		return err.Error()
	}
	return "successfully resize the storage folder"
}

// DeleteFolder delete the folder
func (h *HostPrivateAPI) DeleteFolder(folderPath string) string {
	err := h.storageHost.StorageManager.DeleteFolder(folderPath)
	if err != nil {
		return err.Error()
	}
	return "successfully delete the storage folder"
}
