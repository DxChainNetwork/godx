package ethapi

import(
	"context"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/host"
)

type HostAPI struct{
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         Backend
	host	  *host.Host
}

func NewHostAPI(b Backend, nonceLock *AddrLocker) *HostAPI {
	return &HostAPI{
		am:        b.AccountManager(),
		nonceLock: nonceLock,
		b:         b,
		host:	   b.Host(),
	}
}

func (h *HostAPI) HelloWorld(ctx context.Context) string{
	return "confirmed! host api is working"
}

func (h *HostAPI) Version() string{
	return "mock host version"
}