package storagemanager

type StorageManager struct {
	// TODO: currently mock data structure of storage Manager
}


func New(persistDir string) (*StorageManager, error){
	// TODO: currently mock the storage manager
	return &StorageManager{}, nil
}

// TODO: currently mock the close of storage manager
func (cm *StorageManager) Close() error {
	return nil
}


type StorageManagerAPI interface {
	Close() error
}