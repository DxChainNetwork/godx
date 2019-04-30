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

type PrivateStorageClientAPI struct {
	sc *StorageClient
}

func NewPrivateStorageClientAPI(sc *StorageClient) *PrivateStorageClientAPI {
	return &PrivateStorageClientAPI{sc}
}

func (api *PrivateStorageClientAPI) Setpayment() string {
	return "working in progress: setting payment information"
}
