package storagehost

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/rpc"
	"math/big"
)

type (
	Backend interface {
		AccountManager() *accounts.Manager
		APIs() []rpc.API
	}

	StorageHostIntSetting struct {
		AcceptingContracts   bool   `json:"acceptingcontracts"`
		MaxDownloadBatchSize uint64 `json:"maxdownloadbatchsize"`
		MaxDuration          uint64 `json:"maxduration"`
		MaxReviseBatchSize   uint64 `json:"maxrevisebatchSize"`
		WindowSize           uint64 `json:"windowsize"`

		Deposit       big.Int `json:"deposit"`
		DepositBudget big.Int `json:"depositbudget"`
		MaxDeposit    big.Int `json:"maxdeposit"`

		MinBaseRPCPrice           big.Int `json:"minbaserpcprice"`
		MinContractPrice          big.Int `json:"mincontractprice"`
		MinDownloadBandwidthPrice big.Int `json:"mindownloadbandwidthprice"`
		MinSectorAccessPrice      big.Int `json:"minsectoraccessprice"`
		MinStoragePrice           big.Int `json:"minstorageprice"`
		MinUploadBandwidthPrice   big.Int `json:"minuploadbandwidthprice"`
	}

	StorageHostExtSetting struct {
		AcceptingContracts   bool   `json:"acceptingcontracts"`
		MaxDownloadBatchSize uint64 `json:"maxdownloadbatchSize"`
		MaxDuration          uint64 `json:"maxduration"`
		MaxReviseBatchSize   uint64 `json:"maxrevisebatchSize"`

		RemainingStorage uint64 `json:"remainingstorage"`
		SectorSize       uint64 `json:"sectorsize"`
		TotalStorage     uint64 `json:"totalstorage"`

		WindowSize uint64 `json:"windowsize"`

		Deposit    big.Int `json:"deposit"`
		MaxDeposit big.Int `json:"maxdeposit"`

		BaseRPCPrice           big.Int `json:"baserpcprice"`
		ContractPrice          big.Int `json:"contractprice"`
		DownloadBandwidthPrice big.Int `json:"downloadbandwidthprice"`
		SectorAccessPrice      big.Int `json:"sectoraccessprice"`
		StoragePrice           big.Int `json:"storageprice"`
		UploadBandwidthPrice   big.Int `json:"uploadbandwidthprice"`

		RevisionNumber uint64 `json:"revisionnumber"`
		Version        string `json:"version"`
	}

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
