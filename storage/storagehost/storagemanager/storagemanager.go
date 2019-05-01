package storagemanager

type storageManager struct {
	// TODO: currently mock data structure of storage Manager
}


func New(persistDir string) (*storageManager, error){
	// TODO: currently mock the storage manager
	return &storageManager{}, nil
}

// TODO: currently mock the close of storage manager
func (cm *storageManager) Close() error {
	return nil
}


type StorageManager interface {
	Close() error
}