package les

import "github.com/DxChainNetwork/godx/host"

func (b *LesApiBackend) Host() *host.Host {
	return b.eth.host
}


