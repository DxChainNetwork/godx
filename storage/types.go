// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
)

var (
	// define the program running environment: test, prod
	ENV = Env_Prod

	// TODO 为了测试方便，调小参数默认值
	DefaultMinSectors uint32 = 1
	DefaultNumSectors uint32 = 2
)

const (
	Override = iota
	Append

	Env_Prod = "prod"
	Env_Test = "test"

	// DxFileExt is the extension of DxFile
	DxFileExt     = ".dxfile"
	ConfigVersion = "1.0.1"
)

type (
	// HostIntConfig make group of host setting as object
	HostIntConfig struct {
		AcceptingContracts   bool           `json:"acceptingcontracts"`
		MaxDownloadBatchSize uint64         `json:"maxdownloadbatchsize"`
		MaxDuration          uint64         `json:"maxduration"`
		MaxReviseBatchSize   uint64         `json:"maxrevisebatchSize"`
		WindowSize           uint64         `json:"windowsize"`
		PaymentAddress       common.Address `json:"paymentaddress"`

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
		AcceptingContracts   bool           `json:"acceptingcontracts"`
		MaxDownloadBatchSize uint64         `json:"maxdownloadbatchSize"`
		MaxDuration          uint64         `json:"maxduration"`
		MaxReviseBatchSize   uint64         `json:"maxrevisebatchSize"`
		PaymentAddress       common.Address `json:"paymentaddress"`
		RemainingStorage     uint64         `json:"remainingstorage"`
		SectorSize           uint64         `json:"sectorsize"`
		TotalStorage         uint64         `json:"totalstorage"`

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

		EnodeID    enode.ID        `json:"enodeid"`
		EnodeURL   string          `json:"enodeurl"`
		NodePubKey ecdsa.PublicKey `json:"nodepubkey"`

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
type ContractParams struct {
	Allowance            RentPayment
	HostEnodeUrl         string
	Funding              common.BigInt
	StartHeight          uint64
	EndHeight            uint64
	ClientPaymentAddress common.Address
	Host                 HostInfo
}

// RentPayment stores the StorageClient payment settings for renting the storage space from the host
type RentPayment struct {
	Fund         common.BigInt `json:"fund"`
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

// ClientSetting defines the settings that client used to create contract with other peers,
// where EnableIPViolation specifies if the host with same network IP addresses will be filtered
// out or not
type ClientSetting struct {
	RentPayment       RentPayment `json:"rentpayment"`
	EnableIPViolation bool        `json:"enableipviolation"`
	MaxUploadSpeed    int64       `json:"maxuploadspeed"`
	MaxDownloadSpeed  int64       `json:"maxdownloadspeed"`
}

// both RentPaymentAPIDisplay and ClientSettingAPIDisplay are used for API configurations
type (
	RentPaymentAPIDisplay struct {
		Fund         string `json:"Fund"`
		StorageHosts string `json:"Number of Storage Hosts"`
		Period       string `json:"Storage Time"`
		RenewWindow  string `json:"Renew Time"`

		// ExpectedStorage is amount of data expected to be stored
		ExpectedStorage string `json:"Expected Storage"`
		// ExpectedUpload is expected amount of data upload before redundancy / block
		ExpectedUpload string `json:"Expected Upload"`
		// ExpectedDownload is expected amount of data downloaded / block
		ExpectedDownload string `json:"Expected Download"`
		// ExpectedRedundancy is the average redundancy of files uploaded
		ExpectedRedundancy string `json:"Expected Redundancy"`
	}

	ClientSettingAPIDisplay struct {
		RentPayment       RentPaymentAPIDisplay `json:"RentPayment Setting"`
		EnableIPViolation string                `json:"IP Violation Check Status"`
		MaxUploadSpeed    string                `json:"Max Upload Speed"`
		MaxDownloadSpeed  string                `json:"Max Download Speed"`
	}
)

// Storage Contract Related
type (
	ContractID common.Hash

	ContractStatus struct {
		UploadAbility bool
		RenewAbility  bool
		Canceled      bool
	}

	RecoverableContract struct {
		ID      ContractID
		EnodeID enode.ID
	}

	ContractMetaData struct {
		ID                     ContractID
		EnodeID                enode.ID
		LatestContractRevision types.StorageContractRevision
		StartHeight            uint64
		EndHeight              uint64

		ContractBalance common.BigInt

		UploadCost   common.BigInt
		DownloadCost common.BigInt
		StorageCost  common.BigInt

		// contract available fund
		TotalCost common.BigInt

		GasCost     common.BigInt
		ContractFee common.BigInt

		Status ContractStatus
	}

	// PeriodCost specifies cost storage client needs to pay within one
	// period cycle. It includes cost for all contracts
	PeriodCost struct {
		// ContractFees = ContractFee + GasFee
		ContractFees     common.BigInt `json:"contractfees"`
		UploadCost       common.BigInt `json:"uploadcost"`
		DownloadCost     common.BigInt `json:"downloadcost"`
		StorageCost      common.BigInt `json:"storagecost"`
		PrevContractCost common.BigInt `json:"prevcontractcost"`

		ContractFund             common.BigInt `json:"contractfund"`
		UnspentFund              common.BigInt `json:"unspentfund"`
		WithheldFund             common.BigInt `json:"withheadfund"`
		WithheldFundReleaseBlock uint64        `json:"withheldfundreleaseblock"`
	}
)

func (ci ContractID) String() string {
	return hexutil.Encode(ci[:])
}

func StringToContractID(s string) (id ContractID, err error) {
	// decode the string to byte slice
	decodedString, err := hexutil.Decode(s)
	if err != nil {
		err = fmt.Errorf("failed to convert the string %s back to contractID: %s", s, err.Error())
		return
	}

	// convert the byte slice to ContractID
	copy(id[:], decodedString)
	return
}

type (
	// UploadParams contains the information used by the Client to upload a file
	FileUploadParams struct {
		Source      string
		DxPath      DxPath
		ErasureCode erasurecode.ErasureCoder
		Mode        int
	}

	// FileInfo provides information about a file
	UploadFileInfo struct {
		AccessTime       time.Time `json:"accesstime"`
		Available        bool      `json:"available"`
		ChangeTime       time.Time `json:"changetime"`
		CipherType       string    `json:"ciphertype"`
		CreateTime       time.Time `json:"createtime"`
		Expiration       uint64    `json:"expiration"`
		Filesize         uint64    `json:"filesize"`
		Health           float64   `json:"health"`
		LocalPath        string    `json:"localpath"`
		MaxHealth        float64   `json:"maxhealth"`
		MaxHealthPercent float64   `json:"maxhealthpercent"`
		ModTime          time.Time `json:"modtime"`
		NumStuckChunks   uint64    `json:"numstuckchunks"`
		OnDisk           bool      `json:"ondisk"`
		Recoverable      bool      `json:"recoverable"`
		Redundancy       float64   `json:"redundancy"`
		Renewing         bool      `json:"renewing"`
		SiaPath          string    `json:"siapath"`
		Stuck            bool      `json:"stuck"`
		StuckHealth      float64   `json:"stuckhealth"`
		UploadedBytes    uint64    `json:"uploadedbytes"`
		UploadProgress   float64   `json:"uploadprogress"`
	}

	// DirectoryInfo provides information about a dxdir
	DirectoryInfo struct {
		NumFiles uint64 `json:"num_files"`

		TotalSize uint64 `json:"total_size"`

		Health uint32 `json:"health"`

		StuckHealth uint32 `json:"stuck_health"`

		MinRedundancy uint32 `json:"min_redundancy"`

		TimeLastHealthCheck time.Time `json:"time_last_health_check"`

		TimeModify time.Time `json:"time_modify"`

		NumStuckSegments uint32 `json:"num_stuck_segments"`

		DxPath DxPath `json:"dx_path"`
	}

	// HostHealthInfo is the file structure used for DxFile health update.
	// It has two fields, one indicating whether the host if offline or not,
	// One indicating whether the contract with the host is good for renew.
	HostHealthInfo struct {
		Offline      bool
		GoodForRenew bool
	}

	// HostHealthInfoTable is the map the is passed into DxFile health update.
	// It is a map from host id to HostHealthInfo
	HostHealthInfoTable map[enode.ID]HostHealthInfo

	// FileInfo is the structure containing file info to be displayed
	FileInfo struct {
		DxPath         string  `json:"dxpath"`
		Status         string  `json:"status"`
		SourcePath     string  `json:"sourcepath"`
		FileSize       uint64  `json:"filesize"`
		Redundancy     uint32  `json:"redundancy"`
		StoredOnDisk   bool    `json:"storedondisk"`
		UploadProgress float64 `json:"uploadprogress"`
	}

	// FileBriefInfo is the brief info about a DxFile
	FileBriefInfo struct {
		Path           string  `json:"dxpath"`
		Status         string  `json:"status"`
		UploadProgress float64 `json:"uploadProgress"`
	}
)

const (
	// 4 MB
	SectorSize = uint64(1 << 22)
	HashSize   = 32

	// the minimum size of an RPC message. If an encoded message
	// would be smaller than RPCMinLen, it is padded with random data.
	RPCMinLen = uint64(4096)
)
