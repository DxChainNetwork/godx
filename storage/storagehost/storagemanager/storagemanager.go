package storagemanager

import "github.com/DxChainNetwork/godx/common"

type storageManager struct {
	// TODO: currently mock data structure of storage Manager
}

func New(persistDir string) (*storageManager, error) {
	// TODO: currently mock the storage manager
	return &storageManager{}, nil
}

// TODO: currently mock the close of storage manager
func (cm *storageManager) Close() error {
	return nil
}

type StorageManager interface {
	Close() error
	AddSectorBatch(sectorRoots []common.Hash) error
	AddSector(sectorRoot common.Hash, sectorData []byte) error
	RemoveSector(sectorRoot common.Hash) error
	RemoveSectorBatch(sectorRoots []common.Hash) error
	ReadSector(sectorRoot common.Hash) ([]byte, error)
}
