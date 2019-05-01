package storagehost

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/rpc"
	"math/big"
)

type (

	// Backend is an interface for full node and light node
	Backend interface {
		AccountManager() *accounts.Manager
		APIs() []rpc.API
	}

	// HostFinancialMetrics record the financial element for host
	HostFinancialMetrics struct {
		ContractCount                 uint64  `json:"contractcount"`
		ContractCompensation          big.Int `json:"contractcompensation"`
		PotentialContractCompensation big.Int `json:"potentialcontractcompensation"`

		LockedStorageDeposit    big.Int `json:"lockedstoragedeposit"`
		LostRevenue             big.Int `json:"lostrevenue"`
		LostStorageDeposit      big.Int `json:"loststoragedeposit"`
		PotentialStorageRevenue big.Int `json:"potentialstoragerevenue"`
		RiskedStorageDeposit    big.Int `json:"riskedstoragedeposit"`
		StorageRevenue          big.Int `json:"storagerevenue"`
		TransactionFeeExpenses  big.Int `json:"transactionfeeexpenses"`

		DownloadBandwidthRevenue          big.Int `json:"downloadbandwidthrevenue"`
		PotentialDownloadBandwidthRevenue big.Int `json:"potentialdownloadbandwidthrevenue"`
		PotentialUploadBandwidthRevenue   big.Int `json:"potentialuploadbandwidthrevenue"`
		UploadBandwidthRevenue            big.Int `json:"uploadbandwidthrevenue"`
	}
)
