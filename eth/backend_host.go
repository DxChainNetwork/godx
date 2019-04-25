package eth

import "github.com/DxChainNetwork/godx/storage/storagehost"

func (s *Ethereum) GetStorageHost() *storagehost.StorageHost { return s.storageHost }
