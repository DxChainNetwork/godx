package storagehost

import (
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"math/big"
)

// print the persist directory of the host
func (h *StorageHost) GetPersistDir() string {
	return h.persistDir
}

// print the structure of the host
func (h *StorageHost) PrintStorageHost() {
	spew.Dump(h)
}

// print the internal setting of the host
func (h *StorageHost) PrintIntSetting() {
	spew.Dump(h.InternalSetting())
}

// print the host financial metrics
func (h *StorageHost) PrintFinancialMetrics(){
	spew.Dump(h.FinancialMetrics())
}


// load the internal setting to the host
func (h *StorageHost) LoadIntSetting(str string){
	data := []byte(str)
	internalSetting := StorageHostIntSetting{}
	if err := json.Unmarshal(data, &internalSetting); err != nil{
		h.log.Warn("fail to load the internal setting to storagehost")
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
func (h *StorageHost) LoadFinancialMetrics(str string){
	data := []byte(str)
	metrix := HostFinancialMetrics{}
	if err := json.Unmarshal(data, &metrix); err != nil{
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


// Simply set the Accepting contract and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetAcceptingContract(b bool) {
	h.settings.AcceptingContracts = b
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}

// Simply set the deposit and save into the setting file
// Warning: make sure you understand this step to continue do the operation
// It will rewrite the setting file
func (h *StorageHost) SetDeposit(num int) {
	h.settings.Deposit = *big.NewInt(int64(num))
	if err := h.syncSetting(); err != nil {
		h.log.Warn(err.Error())
	}
}
