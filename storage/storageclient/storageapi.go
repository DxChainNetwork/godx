package storageclient

type PublicStorageClientAPI struct {
	sc *StorageClient
}

func NewPublicStorageClientAPI(sc *StorageClient) *PublicStorageClientAPI {
	return &PublicStorageClientAPI{sc}
}

func (api *PublicStorageClientAPI) Payment() string {
	return "working in progress: getting payment information"
}

func (api *PublicStorageClientAPI) MemoryAvailable() uint64 {
	return api.sc.memoryManager.MemoryAvailable()
}

func (api *PublicStorageClientAPI) MemoryLimit() uint64 {
	return api.sc.memoryManager.MemoryLimit()
}

type PrivateStorageClientAPI struct {
	sc *StorageClient
}

func NewPrivateStorageClientAPI(sc *StorageClient) *PrivateStorageClientAPI {
	return &PrivateStorageClientAPI{sc}
}

func (api *PrivateStorageClientAPI) SetPayment() string {
	return "working in progress: setting payment information"
}

func (api *PrivateStorageClientAPI) SetMemoryLimit(amount uint64) string {
	return api.sc.memoryManager.SetMemoryLimit(amount)
}
