package storagehost

import (
	tm "github.com/DxChainNetwork/godx/common/threadManager"
	sm "github.com/DxChainNetwork/godx/storage/storagehost/storagemanager"
	"os"
)

type StorageHost struct {
	persistDir string
	tg tm.ThreadManager
	sm.StorageManager
}


func NewStorageHost(persistDir string) (*StorageHost, error){
	err := os.Mkdir(persistDir, 0700)
	if err != nil{
		// TODO: handle the load
		return &StorageHost{persistDir: persistDir}, nil
	}
	host := StorageHost{
		persistDir: persistDir,
	}

	return &host, nil
}



func (h *StorageHost)GetPersistDir() string{
	return h.persistDir
}