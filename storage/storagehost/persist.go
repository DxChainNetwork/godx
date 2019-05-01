package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"path/filepath"
)

// the fields that need to write into the jason file
type persistence struct {
	BlockHeight      uint64                `json:"blockHeight"`
	BroadCast        bool                  `json:"broadcast"`
	RevisionNumber   uint64                `json:"revisionnumber"`
	FinalcialMetrics HostFinancialMetrics  `json:"finalcialmetrics"`
	Settings         StorageHostIntSetting `json:"settings"`
}

// save the host settings: the filed as persistence shown, to the json file
func (h *StorageHost) syncSetting() error {
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
		BlockHeight:      h.blockHeight,
		BroadCast:        h.broadcast,
		FinalcialMetrics: h.financialMetrics,
		RevisionNumber:   h.revisionNumber,
		Settings:         h.settings,
	}
}

// Require: lock the storageHost by caller
// load the persistence data to the host
func (h *StorageHost) loadPersistence(persist *persistence) {
	h.blockHeight = persist.BlockHeight
	h.broadcast = persist.BroadCast

	// TODO: address checking if NetAddress need to be store to the Setting file
	// TODO: unlock hash checking if unlock hash need to be store to the Setting file

	h.financialMetrics = persist.FinalcialMetrics
	h.revisionNumber = persist.RevisionNumber
	h.settings = persist.Settings
}
