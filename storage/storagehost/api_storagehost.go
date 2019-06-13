package storagehost

import (
	"context"
	"fmt"

	"github.com/DxChainNetwork/godx/accounts"

	"github.com/DxChainNetwork/godx/common"

	"github.com/DxChainNetwork/godx/storage"
)

// TODO: NOTE, this API should be make as private.
//  It provide a way to modify the internal setting

// TODO: Provide a bridge method for take interaction with user,
//  make sure they understand the effect of taking some of the operation in debug

// HostDeBugAPI give developer a way for access and modify the setting file
type HostDeBugAPI struct {
	storagehost *StorageHost
}

// NewHostDebugAPI generate a HostDeBugAPI reference for caller
func NewHostDebugAPI(storagehost *StorageHost) *HostDeBugAPI {
	return &HostDeBugAPI{
		storagehost: storagehost,
	}
}

// HelloWorld just test if the debug API is working, could be called in console
func (h *HostDeBugAPI) HelloWorld(ctx context.Context) string {
	return "confirmed! host api is working"
}

func (h *HostDeBugAPI) SetPaymentAddress(paymentAddress common.Address) bool {
	account := accounts.Account{Address: paymentAddress}
	_, err := h.storagehost.ethBackend.AccountManager().Find(account)
	if err != nil {
		h.storagehost.log.Error("You must set up an account owned by your local wallet!")
		return false
	}

	h.storagehost.lock.Lock()
	h.storagehost.config.PaymentAddress = paymentAddress
	h.storagehost.lock.Unlock()

	return true
}

func (h *HostDeBugAPI) GetPaymentAddress() (common.Address, error) {
	h.storagehost.lock.RLock()
	paymentAddress := h.storagehost.config.PaymentAddress
	h.storagehost.lock.RUnlock()

	if paymentAddress != (common.Address{}) {
		return paymentAddress, nil
	}

	if wallets := h.storagehost.ethBackend.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			paymentAddress := accounts[0].Address
			h.storagehost.lock.Lock()
			h.storagehost.config.PaymentAddress = paymentAddress
			h.storagehost.lock.Unlock()
			h.storagehost.log.Info("host automatically configured", "address", paymentAddress)
			return paymentAddress, nil
		}
	}
	return common.Address{}, fmt.Errorf("paymentAddress must be explicitly specified")
}

// Version gives a mock version of the debugapi
func (h *HostDeBugAPI) Version() string {
	return "mock host version"
}

// Persistdir print the persist directory of the host
func (h *HostDeBugAPI) Persistdir() string {
	return h.storagehost.getPersistDir()
}

// PrintHostPersist print the structure of the host persist
func (h *HostDeBugAPI) PrintHostPersist() {
	h.storagehost.getHostPersist()
}

// PrintIntConfig print the internal setting of host
func (h *HostDeBugAPI) PrintIntConfig() {
	h.storagehost.getIntConfig()
}

// PrintFinancialMetrics print the internal financial metrics of the host
func (h *HostDeBugAPI) PrintFinancialMetrics() {
	h.storagehost.getFinancialMetrics()
}

// SetDefault load the internal setting back to default
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetDefault() {
	h.storagehost.setDefault()
}

// SetBroadCast Set the broadcast to a boolean value and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetBroadCast(b bool) {
	h.storagehost.setBroadCast(b)
}

// SetRevisionNumber Set the revision number and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetRevisionNumber(num int) {
	h.storagehost.setRevisionNumber(num)
}

// LoadIntConfig Set the Internal setting of the host in Object format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadIntConfig(intConfig storage.HostIntConfig) {
	h.storagehost.setIntConfig(intConfig)
}

// LoadFinancialMetrics Set the Financial Metrics of the host in Object format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadFinancialMetrics(metric HostFinancialMetrics) {
	h.storagehost.setFinancialMetrics(metric)
}
