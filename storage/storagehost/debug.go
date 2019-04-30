package storagehost

import (
	"github.com/davecgh/go-spew/spew"
	"math/big"
)

// print the persist directory of the host
func (h *StorageHost) GetPersistDir() string {
	return h.persistDir
}

// print the internal setting of the host
func (h *StorageHost) PrintIntSetting() {
	persist := h.extractPersistence()
	spew.Dump(persist)
}

// print the structure of the host
func (h *StorageHost) PrintStorageHost() {
	spew.Dump(h)
}

// Set the broadcast to a boolean value and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetBroadCast(b bool) {
	h.broadcast = b
	h.syncSetting()
}

// Set the revision number and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetRevisionNumber(num int) {
	h.revisionNumber = uint64(num)
	h.syncSetting()
}

// Simply set the Accepting contract and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetAcceptingContract(b bool) {
	h.settings.AcceptingContracts = b
	h.syncSetting()
}

// Simply set the deposit and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetDeposit(num int) {
	h.settings.Deposit = *big.NewInt(int64(num))
	h.syncSetting()
}
