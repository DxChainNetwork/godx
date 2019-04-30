package storagehost

import (
	"context"
)

// TODO: NOTE, this API should be make as private.
//  It provide a way to modify the internal setting

// TODO: Provide a bridge method for take interaction with user,
//  make sure they understand the effect of taking some of the operation in debug

type HostDeBugAPI struct {
	storagehost *StorageHost
}

func NewHostDebugAPI(storagehost *StorageHost) *HostDeBugAPI {
	return &HostDeBugAPI{
		storagehost: storagehost,
	}
}

func (h *HostDeBugAPI) HelloWorld(ctx context.Context) string {
	return "confirmed! host api is working"
}

func (h *HostDeBugAPI) Version() string {
	return "mock host version"
}

// print the persist directory of the host
func (h *HostDeBugAPI) Persistdir() string {
	return h.storagehost.GetPersistDir()
}

// print the structure of the host
func (h *HostDeBugAPI) PrintStorageHost() {
	h.storagehost.PrintStorageHost()
}

// print the internal financial metrics of the host
func (h *HostDeBugAPI) PrintPrintFinancialMetrics(){
	h.storagehost.PrintFinancialMetrics()
}

// Set the broadcast to a boolean value and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetBroadCast(b bool) {
	h.storagehost.SetBroadCast(b)
}

// Set the revision number and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetRevisionNumber(num int) {
	h.storagehost.SetRevisionNumber(num)
}

// Simply set the Accepting contract and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetAcceptingContract(b bool) {
	h.storagehost.SetAcceptingContract(b)
}

// Simply set the deposit and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetDeposit(num int) {
	h.storagehost.SetDeposit(num)
}
