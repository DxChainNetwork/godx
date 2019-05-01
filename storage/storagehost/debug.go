package storagehost

import (
	"encoding/json"
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
)

// print the persist directory of the host
func (h *StorageHost) getPersistDir() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.persistDir
}

// print the structure persistence of storage host
func (h *StorageHost) getHostPersist() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	persist := h.extractPersistence()
	b, _ := json.MarshalIndent(persist, "", "")
	fmt.Println(string(b))
}

// print the internal config of the host
func (h *StorageHost) getIntConfig() {
	b, _ := json.MarshalIndent(h.InternalConfig(), "", "")
	fmt.Println(string(b))
}

// print the host financial metrics
func (h *StorageHost) getFinancialMetrics() {
	b, _ := json.MarshalIndent(h.FinancialMetrics(), "", "")
	fmt.Println(string(b))
}

// load the internal config back to default
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setDefault() {
	h.loadDefaults()
	// synchronize to file
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}

// Set the broadcast to a boolean value and save into the config file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setBroadCast(b bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.broadcast = b
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}

// Set the revision number and save into the config file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setRevisionNumber(num int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.revisionNumber = uint64(num)
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}

// load the internal config to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setIntConfig(intConfig storage.HostIntConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = intConfig

	// synchronize to file
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}

// load the financial metrics to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setFinancialMetrics(metric HostFinancialMetrics) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// directly load the financial metrics to the host
	h.financialMetrics = metric

	// synchronize to file
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}
