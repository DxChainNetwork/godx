package storagehost

import (
	"github.com/DxChainNetwork/godx/rpc"
	"math/big"
)

type (
	Backend interface {
		APIs() []rpc.API
	}

	// ALL fields here
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

		// TODO: Net Address may be got from somewhere else
		NetAddress string
	}

	// ALL fields here
	StorageHostExtSetting struct {
		AcceptingContracts   bool
		MaxDownloadBatchSize uint64
		MaxDuration          uint64 // the block height, currently use uint64, if the value is too large, switch to bigint
		MaxReviseBatchSize   uint64

		NetAddress string

		RemainingStorage uint64
		SectorSize       uint64
		TotalStorage     uint64

		// TODO: unlockHash
		//UnlockHash         crypto.Hash

		WindowSize uint64

		Deposit    big.Int
		MaxDeposit big.Int

		BaseRPCPrice           big.Int
		ContractPrice          big.Int
		DownloadBandwidthPrice big.Int
		SectorAccessPrice      big.Int
		StoragePrice           big.Int
		UploadBandwidthPrice   big.Int

		RevisionNumber uint64
		Version        string
	}

	// ALL fields here
	HostFinancialMetrics struct {
		ContractCount                 	uint64
		ContractCompensation          	big.Int
		PotentialContractCompensation	big.Int


		LockedStorageDeposit			big.Int
		LostRevenue             		big.Int
		LostStorageDeposit   			big.Int
		PotentialStorageRevenue 		big.Int
		RiskedStorageDeposit 			big.Int
		StorageRevenue          		big.Int
		TransactionFeeExpenses  		big.Int


		DownloadBandwidthRevenue          big.Int
		PotentialDownloadBandwidthRevenue big.Int
		PotentialUploadBandwidthRevenue   big.Int
		UploadBandwidthRevenue            big.Int
	}
)
