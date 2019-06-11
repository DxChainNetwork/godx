package vm

var (
	StrPrefixExpSC = "ExpiredStorageContract_"

	ProofedStatus    = []byte{'1'}
	NotProofedStatus = []byte{'0'}

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
