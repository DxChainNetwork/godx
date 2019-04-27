package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/davecgh/go-spew/spew"
	"path/filepath"
)

// the fields that need to write into the jason file
type persistence struct {
	BroadCast      bool // Indicate if the host broadcast
	RevisionNumber uint64
	Settings       StorageHostIntSetting
}

// save the host settings: the filed as persistence shown, to the json file
func (h *StorageHost) SyncSetting() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// extract the persistence from host
	persist := h.extractPersistence()

	spew.Dump(persist)

	// use the json package save the extracted persistence data
	return common.SaveDxJSON(storageHostMeta,
		filepath.Join(h.persistDir, HostSettingFile), persist)
}

// Require: lock the storageHost by caller
// extract the persistence data from the host
func (h *StorageHost) extractPersistence() *persistence {
	return &persistence{
		BroadCast:      h.broadcast,
		RevisionNumber: h.revisionNumber,
		Settings:       h.settings,
	}
}

// Require: lock the storageHost by caller
// load the persistence data to the host
func (h *StorageHost) loadPersistence(persist *persistence) {
	h.broadcast = persist.BroadCast
	h.revisionNumber = persist.RevisionNumber
	h.settings = persist.Settings
}
