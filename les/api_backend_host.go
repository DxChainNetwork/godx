package les

import "github.com/DxChainNetwork/godx/storage/storagehost"

func (b *LesApiBackend) StorageHost() *storagehost.StorageHost {
	return b.eth.storageHost
}
