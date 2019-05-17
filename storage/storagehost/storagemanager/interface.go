package storagemanager

// StorageManager is an interface for giving user
// right to manipulate storage manager
type StorageManager interface {
	AddStorageFolder(path string, size uint64) error
	Close() error
}
