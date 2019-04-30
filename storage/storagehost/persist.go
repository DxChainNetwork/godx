package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"path/filepath"
)

// the fields that need to write into the jason file
type persistence struct {
	// TODO: not sure if need or not, just place here
	BlockHeight			uint64

	BroadCast      		bool // Indicate if the host broadcast
	AutoAddress			string
	RevisionNumber 		uint64
	FinalcialMetrics	HostFinancialMetrics
	Settings       		StorageHostIntSetting
	//TODO: UnlockHash  types UnlockHash
}

// save the host settings: the filed as persistence shown, to the json file
func (h *StorageHost) syncSetting() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// extract the persistence from host
	persist := h.extractPersistence()

	// use the json package save the extracted persistence data
	return common.SaveDxJSON(storageHostMeta,
		filepath.Join(h.persistDir, HostSettingFile), persist)
}

// Require: lock the storageHost by caller
// extract the persistence data from the host
func (h *StorageHost) extractPersistence() *persistence {
	return &persistence{
		BlockHeight: 	h.blockHeight,

		BroadCast:      	h.broadcast,
		AutoAddress:		h.autoAddress,
		FinalcialMetrics:	h.financialMetrics,
		RevisionNumber: 	h.revisionNumber,
		Settings:       	h.settings,
		// TODO: unlock hash
	}
}

// Require: lock the storageHost by caller
// load the persistence data to the host
func (h *StorageHost) loadPersistence(persist *persistence) {
	h.blockHeight = persist.BlockHeight

	h.broadcast = persist.BroadCast
	h.autoAddress = persist.AutoAddress

	if err := IsValidAddress(persist.AutoAddress); err != nil{
		// TODO: log the warning
		h.autoAddress = ""
	}

	h.financialMetrics = persist.FinalcialMetrics
	h.revisionNumber = persist.RevisionNumber
	h.settings = persist.Settings

	if err := IsValidAddress(persist.Settings.NetAddress); err != nil{
		// TODO: log the warning
		h.settings.NetAddress = ""
	}

	// TODO:  unlock hash
}

