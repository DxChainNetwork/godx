package storage

import "github.com/DxChainNetwork/godx/rpc"

// ClientBackend allows Ethereum object to be passed in as interface
type ClientBackend interface {
	APIs() []rpc.API
	GetStorageHostSetting(peerID string, config *HostExtConfig) error
}
