package storagehost

import (
	"encoding/json"
	"fmt"
)

// print the persist directory of the host
func (h *StorageHost) GetPersistDir() string {
	return h.persistDir
}

// print the structure persistence of storage host
func (h *StorageHost) PrintHostPersist() {
	persist := h.extractPersistence()
	b, _ := json.MarshalIndent(persist, "", "")
	fmt.Println(string(b))
}

// print the internal setting of the host
func (h *StorageHost) PrintIntSetting() {
	b, _ := json.MarshalIndent(h.InternalSetting(), "", "")
	fmt.Println(string(b))
}

// print the host financial metrics
func (h *StorageHost) PrintFinancialMetrics() {
	b, _ := json.MarshalIndent(h.FinancialMetrics(), "", "")
	fmt.Println(string(b))
}

// load the internal setting back to default
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetDefault() {
	h.loadDefaults()
	// synchronize to file
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// load the internal setting to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) LoadIntSettingStr(str string) {
	data := []byte(str)
	internalSetting := StorageHostIntSetting{}
	if err := json.Unmarshal(data, &internalSetting); err != nil {
		// TODO: log the information in a better way
		fmt.Println(err.Error())
		h.log.Warn("fail to load the internal setting to storage host")
		return
	}

	// instead of using SetIntSetting method
	// directly call to load the setting to host, to avoid some checking
	// and increment of revision number
	h.settings = internalSetting

	// synchronize to file
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// load the financial metrics to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) LoadFinancialMetricsStr(str string) {
	data := []byte(str)
	metrix := HostFinancialMetrics{}
	if err := json.Unmarshal(data, &metrix); err != nil {
		h.log.Warn("fail to load the HostFinancialMetrics to storagehost")
		return
	}

	// directly load the financial metrics to the host
	h.financialMetrics = metrix

	// synchronize to file
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// Set the broadcast to a boolean value and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetBroadCast(b bool) {
	h.broadcast = b
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// Set the revision number and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetRevisionNumber(num int) {
	h.revisionNumber = uint64(num)
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// load the internal setting to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) LoadIntSetting(internalSetting StorageHostIntSetting) {
	h.settings = internalSetting

	// synchronize to file
	if err := h.syncSetting(); err != nil {
		fmt.Println(err.Error())
		h.log.Warn(err.Error())
	}
}

// load the financial metrics to the host
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) LoadFinancialMetrics(metric HostFinancialMetrics) {
	// directly load the financial metrics to the host
	h.financialMetrics = metric

	// synchronize to file
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}
