package eth

import "github.com/DxChainNetwork/godx/host"

func (b *EthAPIBackend) Host() *host.Host{
	return b.eth.GetHost()
}