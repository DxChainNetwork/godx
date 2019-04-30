package storageclient

const (
	PersistDirectory            = "storageclient"
	PersistFilename             = "storageclient.json"
	PersistLogname              = "storageclient.log"
	PersistStorageClientVersion = "1.3.6"
	DxPathRoot                  = "dxfiles"
)

// StorageClient Settings, where 0 means unlimited
const (
	DefaultMaxDownloadSpeed = 0
	DefaultMaxUploadSpeed   = 0
	DefaultStreamCacheSize  = 2
)
