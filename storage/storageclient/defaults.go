package storageclient

// Files and directories related constant
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

// Max memory available
const (
	DefaultMaxMemory = uint64(3 * 1 << 28)
)

// Backup Header
const (
	encryptionPlaintext = "plaintext"
	encryptionTwofish   = "twofish"
	encryptionVersion   = "1.0"
)

// DxFile Related
const (
	DxFileExtension = ".dx"
)
