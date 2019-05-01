package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"path/filepath"
)

// the fields that need to write into the jason file
type persistence struct {
	BlockHeight      uint64                `json:"blockHeight"`
	BroadCast        bool                  `json:"broadcast"`
	RevisionNumber   uint64                `json:"revisionnumber"`
	FinalcialMetrics HostFinancialMetrics  `json:"finalcialmetrics"`
	Config           storage.HostIntConfig `json:"config"`
}

// save the host config: the filed as persistence shown, to the json file
func (h *StorageHost) syncConfig() error {
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
		Config:           h.config,
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
	h.config = persist.Config
}
