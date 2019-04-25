package ethapi

import (
	"context"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

type StorageHostAPI struct {
	am        		*accounts.Manager
	nonceLock 		*AddrLocker
	b         		Backend
	storagehost     *storagehost.StorageHost
}

func NewStorageHostAPI(b Backend, nonceLock *AddrLocker) *StorageHostAPI {
	return &StorageHostAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
		storagehost:     b.StorageHost(),
	}
}

func (h *StorageHostAPI) HelloWorld(ctx context.Context) string {
	return "confirmed! host api is working"
}

func (h *StorageHostAPI) Version() string {
	return "mock host version"
}

func (h *StorageHostAPI) Persistdir() string{
	return h.storagehost.GetPersistDir()
}