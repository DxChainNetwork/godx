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
	FinancialMetrics HostFinancialMetrics  `json:"financialmetrics"`
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

// loadConfig load host config from the file.
func (h *StorageHost) loadConfig() error {
	// load and create a persist from JSON file
	persist := new(persistence)
	// if it is loaded the file causing the error, directly return the error info
	// and not do any modification to the host
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join(h.persistDir, HostSettingFile), persist); err != nil {
		return err
	}
	h.loadPersistence(persist)
	return nil
}

// Require: lock the storageHost by caller
// extract the persistence data from the host
func (h *StorageHost) extractPersistence() *persistence {
	return &persistence{
		BlockHeight:      h.blockHeight,
		BroadCast:        h.broadcast,
		FinancialMetrics: h.financialMetrics,
		Config:           h.config,
	}
}

// Require: lock the storageHost by caller
// load the persistence data to the host
func (h *StorageHost) loadPersistence(persist *persistence) {
	h.blockHeight = persist.BlockHeight
	h.broadcast = persist.BroadCast
	h.financialMetrics = persist.FinancialMetrics
	h.config = persist.Config
}
