package storagemanager

import (
	"github.com/DxChainNetwork/godx/common"
	bit "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager/bitvector"
	"path/filepath"
)

type persistence struct {
	SectorSalt [32]byte
	Folders    []folderPersist
}

type folderPersist struct {
	// Index is the Index of the storageFolder
	Index uint16
	// Path represent the Path of the storageFolder
	Path string
	// Usage represent the Usage of the storageSector in the storageFolder
	Usage []bit.BitVector
}

func (sm *storageManager) extractPersist() *persistence {
	return &persistence{
		SectorSalt: sm.sectorSalt,
		Folders:    sm.extractFolderPersist(),
	}
}

func (sm *storageManager) extractFolderPersist() []folderPersist {
	folders := make([]folderPersist, len(sm.folders))

	var idx int
	for _, folder := range sm.folders {

		persist := folderPersist{
			Index: folder.index,
			Path:  folder.path,
			Usage: folder.usage,
		}

		folders[idx] = persist
		idx++

	}

	return folders
}

func (sm *storageManager) syncConfig() error {
	// extract the persistence information from current storage manager, and save every thing to config
	persist := sm.extractPersist()
	//spew.Dump(persist)
	err := common.SaveDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), persist)
	// if saving to config file fail
	if err != nil {
		return err
	}
	return nil
}
