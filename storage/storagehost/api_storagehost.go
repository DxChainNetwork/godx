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
func (h *HostDeBugAPI) PrintHostPersist() {
	h.storagehost.PrintHostPersist()
}

// print the internal setting of host
func (h *HostDeBugAPI) PrintIntSetting() {
	h.storagehost.PrintIntSetting()
}

// print the internal financial metrics of the host
func (h *HostDeBugAPI) PrintFinancialMetrics() {
	h.storagehost.PrintFinancialMetrics()
}

// load the internal setting back to default
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) SetDefault() {
	h.storagehost.SetDefault()
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

// Set the Internal setting of the host in String format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadInternalSettingStream(str string) {
	h.storagehost.LoadIntSettingStr(str)
}

// Set the Financial Metrics of the host in String format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadFinancialMetricsStream(str string) {
	h.storagehost.LoadFinancialMetricsStr(str)
}

// Set the Internal setting of the host in Object format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadInternalSetting(internalSetting StorageHostIntSetting) {
	h.storagehost.LoadIntSetting(internalSetting)
}

// Set the Financial Metrics of the host in Object format
// the same as the structure input to the console
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *HostDeBugAPI) LoadFinancialMetrics(metric HostFinancialMetrics) {
	h.storagehost.LoadFinancialMetrics(metric)
}
