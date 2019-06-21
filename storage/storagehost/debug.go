package storagehost

// load the financial metrics to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the config file
func (h *StorageHost) setFinancialMetrics(metric HostFinancialMetrics) {
	h.lock.Lock()
	defer h.lock.Unlock()

	// directly load the financial metrics to the host
	h.financialMetrics = metric

	// synchronize to file
	if err := h.syncConfig(); err != nil {
		h.log.Warn(err.Error())
	}
}
