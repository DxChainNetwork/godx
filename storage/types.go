// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
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

	// StorageHostManager related data structures
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

		// TODO (mzhang): not sure if IP will be provided. However, if not
		// it can still be derived from the enode information. Need to verify
		// with HZ office
		IP string `json:"ip"`

		IPNets          []string  `json:"ipnets"`
		LastIPNetChange time.Time `json:"lastipnetchange"`

		// TODO (mzhang): verify with hz, check if the public key will be proviced
		// if public key is not provided, what kind of information will be used to
		// verify the host identity, enode ?
		PublicKey string `json:"publickey"`

		Filtered bool `json:"filtered"`
	}


	// TODO (mzhang): change the name to maintenance
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
	// TODO (mzhang): double check with Hangzhou office to see if they will be in charge of the
	// rent payment
	// TODO (mzhang): ensure those value should not be 0
	RentPayment struct {
		Payment            common.BigInt `json:"payment"`
		StorageHosts       uint64        `json:"storagehosts"`
		Period             uint64        `json:"period"`
		RenewWindow        uint64        `json:"renewwindow"`
		ExpectedStorage    uint64        `json:"expectedstorage"`
		ExpectedUpload     uint64        `json:"expectedupload"`
		ExpectedDownload   uint64        `json:"expecteddownload"`
		ExpectedRedundancy float64       `json:"expectedredundancy"`
	}
)
