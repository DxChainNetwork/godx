package storagehostmanager

import "github.com/DxChainNetwork/godx/internal/ethapi"

type ParsedAPI struct {

}


func (sc *StorageClient) filterAPIs(apis []rpc.API) {
	for _, api := range apis {
		switch typ := reflect.TypeOf(api.Service); typ {
		case reflect.TypeOf(&ethapi.PublicNetAPI{}):
			sc.netInfo = api.Service.(*ethapi.PublicNetAPI)
		case reflect.TypeOf(&ethapi.PrivateAccountAPI{}):
			sc.account = api.Service.(*ethapi.PrivateAccountAPI)
		case reflect.TypeOf(&ethapi.PublicEthereumAPI{}):
			sc.ethInfo = api.Service.(*ethapi.PublicEthereumAPI)
		default:
			continue
		}
	}
}