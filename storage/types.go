package storage

import (
	"math/big"
	"time"
)

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

// StorageHostManager related data structures
type HostInfo struct {
	HostExtConfig

	FirstSeen uint64 `json:"firstseen"`

	HistoricDowntime time.Duration `json:"historicdowntime"`
	HistoricUptime   time.Duration `json:"historicuptime"`
	ScanRecords      HostPoolScans `json:"scanrecords"`

	HistoricFailedInteractions     float64 `json:"historicfailedinteractions"`
	HistoricSuccessfulInteractions float64 `json:"historicsuccessfulinteractions"`
	RecentFailedInteractions       float64 `json:"recentfailedinteractions"`
	RecentSuccessfulInteractions   float64 `json:"recentsuccessfulinteractions"`

	LastHistoricUpdate uint64 `json:"lasthistoricupdate"`

	IPNets          []string  `json:"ipnets"`
	LastIPNetChange time.Time `json:"lastipnetchange"`

	// TODO (mzhang): verify with hz, check if the public key will be proviced
	// if public key is not provided, what kind of information will be used to
	// verify the host identity, enode ?
	PublicKey string `json:"publickey"`

	Filtered bool `json:"filtered"`
}

type HostPoolScans []HostPoolScan

type HostPoolScan struct {
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}
