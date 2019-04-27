package storagehost

import "math/big"

type (
	StorageHostIntSetting struct {
		AcceptingContracts   bool
		MaxDownloadBatchSize uint64
		MaxDuration          uint64 // the block height, currently use uint64, if the value is too large, switch to bigint
		MaxReviseBatchSize   uint64
		WindowSize           uint64

		Deposit       big.Int
		DepositBudget big.Int
		MaxDeposit    big.Int

		MinBaseRPCPrice           big.Int
		MinContractPrice          big.Int
		MinDownloadBandwidthPrice big.Int
		MinSectorAccessPrice      big.Int
		MinStoragePrice           big.Int
		MinUploadBandwidthPrice   big.Int
	}
)
