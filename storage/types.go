// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
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

		Deposit    common.BigInt `json:"deposit"`
		MaxDeposit common.BigInt `json:"maxdeposit"`

		BaseRPCPrice           common.BigInt `json:"baserpcprice"`
		ContractPrice          common.BigInt `json:"contractprice"`
		DownloadBandwidthPrice common.BigInt `json:"downloadbandwidthprice"`
		SectorAccessPrice      common.BigInt `json:"sectoraccessprice"`
		StoragePrice           common.BigInt `json:"storageprice"`
		UploadBandwidthPrice   common.BigInt `json:"uploadbandwidthprice"`

		RevisionNumber uint64 `json:"revisionnumber"`
		Version        string `json:"version"`
	}

	// HostInfo storage storage host information
	HostInfo struct {
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

		// IP will be decoded from the enode URL
		IP string `json:"ip"`

		IPNetwork           string    `json:"ipnetwork"`
		LastIPNetWorkChange time.Time `json:"lastipnetworkchange"`

		EnodeID  enode.ID `json:"enodeid"`
		EnodeURL string   `json:"enodeurl"`

		Filtered bool `json:"filtered"`
	}

	// HostPoolScans stores a list of host pool scan records
	HostPoolScans []HostPoolScan

	// HostPoolScan recorded the scan details, including the time a storage host got scanned
	// and whether the host is online or not
	HostPoolScan struct {
		Timestamp time.Time `json:"timestamp"`
		Success   bool      `json:"success"`
	}
)

type (
	// RentPayment stores the StorageClient payment settings for renting the storage space from the host
	RentPayment struct {
		Payment      common.BigInt `json:"payment"`
		StorageHosts uint64        `json:"storagehosts"`
		Period       uint64        `json:"period"`
		RenewWindow  uint64        `json:"renewwindow"`

		// ExpectedStorage is amount of data expected to be stored
		ExpectedStorage uint64 `json:"expectedstorage"`
		// ExpectedUpload is expected amount of data upload before redundancy / block
		ExpectedUpload uint64 `json:"expectedupload"`
		// ExpectedDownload is expected amount of data downloaded / block
		ExpectedDownload uint64 `json:"expecteddownload"`
		// ExpectedRedundancy is the average redundancy of files uploaded
		ExpectedRedundancy float64 `json:"expectedredundancy"`
	}
)

type (
	// HostHealthInfo is the file structure used for DxFile health update.
	// It has two fields, one indicating whether the host if offline or not,
	// One indicating whether the contract with the host is good for renew.
	HostHealthInfo struct {
		Offline bool
		GoodForRenew bool
	}

	// HostHealthInfoTable is the map the is passed into DxFile health update.
	// It is a map from host id to HostHealthInfo
	HostHealthInfoTable map[enode.ID]HostHealthInfo
)
