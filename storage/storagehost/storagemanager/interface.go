package storagemanager

// StorageManager is an interface for giving user
// right to manipulate storage manager
type StorageManager interface {
	AddStorageFolder(path string, size uint64) error
	AddSector(root [32]byte, sectorData []byte) error
	ReadSector(root [32]byte) ([]byte, error)
	RemoveSector(id sectorID) error
	Close() error
}
