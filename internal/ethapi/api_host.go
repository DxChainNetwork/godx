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


func (h *StorageHostAPI) PrintSetting(){
	h.storagehost.PrintIntSetting()
}


// TODO: Setting will not be saved until the program close
func (h *StorageHostAPI) SetBroadCast(b bool){
	h.storagehost.SetBroadCast(b)
}

func (h *StorageHostAPI) SetRevisionNumber(num int){
	h.storagehost.SetRevisionNumber(num)
}

func (h *StorageHostAPI) SetAcceptingContract(b bool){
	h.storagehost.SetAcceptingContract(b)
}

func (h *StorageHostAPI) SetDeposit(num int){
	h.storagehost.SetDeposit(num)
}