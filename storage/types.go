package storage

import "math/big"

type (
	// HostIntConfig make group of host setting as object
	HostIntConfig struct {
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

	// HostExtConfig make group of host setting to broadcast as object
	HostExtConfig struct {
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
)
