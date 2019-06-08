package vm

var (
	StrPrefixExpSC = "expired_storage_contract_"

	BytesClientCollateral   = []byte("ClientCollateral")
	BytesHostCollateral     = []byte("HostCollateral")
	BytesFileSize           = []byte("FileSize")
	BytesUnlockHash         = []byte("UnlockHash")
	BytesFileMerkleRoot     = []byte("FileMerkleRoot")
	BytesRevisionNumber     = []byte("RevisionNumber")
	BytesWindowStart        = []byte("WindowStart")
	BytesWindowEnd          = []byte("WindowEnd")
	BytesValidProofOutputs  = []byte("ValidProofOutputs")
	BytesMissedProofOutputs = []byte("MissedProofOutputs")
)
