package eth

import "github.com/DxChainNetwork/godx/storage/storagehost"

func (b *EthAPIBackend) StorageHost() *storagehost.StorageHost {
	return b.eth.GetStorageHost()
}

func (b *EthAPIBackend) peerCount() int{
	return int(b.eth.netRPCService.PeerCount())
}
