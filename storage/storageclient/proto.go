// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

type ContractParams struct {
	Allowance       Allowance
	HostEnodeUrl    string
	Funding         *big.Int
	StartHeight     uint64
	EndHeight       uint64
	ClientPublicKey ecdsa.PublicKey
	Host            StorageHostEntry
}

// the amount that client is allowed to spend in a given period
type Allowance struct {
	Funds       *big.Int `json:"funds"`
	Hosts       uint64   `json:"hosts"`
	Period      uint64   `json:"period"`
	RenewWindow uint64   `json:"renewwindow"`

	// the amount of data that we expect to have in a contract.
	ExpectedStorage uint64 `json:"expectedstorage"`

	// the expected amount of data uploaded
	ExpectedUpload uint64 `json:"expectedupload"`

	// the expected amount of data downloaded
	ExpectedDownload uint64 `json:"expecteddownload"`

	// the average redundancy of files being uploaded.
	ExpectedRedundancy float64 `json:"expectedredundancy"`
}

// define a host entry in the client's host db
type StorageHostEntry struct {
	HostExternalSettings

	// the last block height at which this host was announced
	FirstSeen        uint64      `json:"firstseen"`
	HistoricDowntime int64       `json:"historicdowntime"`
	HistoricUptime   int64       `json:"historicuptime"`
	ScanHistory      HostDBScans `json:"scanhistory"`

	// record some measurements when interacting with the host
	HistoricFailedInteractions     float64 `json:"historicfailedinteractions"`
	HistoricSuccessfulInteractions float64 `json:"historicsuccessfulinteractions"`
	RecentFailedInteractions       float64 `json:"recentfailedinteractions"`
	RecentSuccessfulInteractions   float64 `json:"recentsuccessfulinteractions"`

	LastHistoricUpdate uint64 `json:"lasthistoricupdate"`

	// measurements related to the IP subnet mask.
	IPNets          []string  `json:"ipnets"`
	LastIPNetChange time.Time `json:"lastipnetchange"`

	// the public key of the host
	PublicKey ecdsa.PublicKey `json:"publickey"`

	// whether or not a StorageHostEntry is being filtered out of the hosttree
	Filtered bool `json:"filtered"`
}

// HostExternalSettings represents the parameters broadcast by the host
type HostExternalSettings struct {
	AcceptingContracts   bool        `json:"acceptingcontracts"`
	MaxDownloadBatchSize uint64      `json:"maxdownloadbatchsize"`
	MaxDuration          uint64      `json:"maxduration"`
	MaxReviseBatchSize   uint64      `json:"maxrevisebatchsize"`
	NetAddress           string      `json:"netaddress"`
	RemainingStorage     uint64      `json:"remainingstorage"`
	SectorSize           uint64      `json:"sectorsize"`
	TotalStorage         uint64      `json:"totalstorage"`
	UnlockHash           common.Hash `json:"unlockhash"`
	WindowSize           uint64      `json:"windowsize"`

	// the amount of collateral that the host will put up for storage contract,
	// as an assurance to the client that the host really is committed to keeping the file.
	Collateral *big.Int `json:"collateral"`

	// the maximum number of coins that a host is willing to put into a storage contract
	MaxCollateral *big.Int `json:"maxcollateral"`

	// a flat per-RPC fee charged by the host for any non-free RPC.
	BaseRPCPrice *big.Int `json:"baserpcprice"`

	// the number of coins that the client needs to pay to the host to form contract.
	ContractPrice *big.Int `json:"contractprice"`

	// the cost per byte of downloading data from the host
	DownloadBandwidthPrice *big.Int `json:"downloadbandwidthprice"`

	// the cost per sector of data accessed when downloading data
	SectorAccessPrice *big.Int `json:"sectoraccessprice"`

	// the cost per-byte-per-block in hastings of storing data on the host
	StoragePrice *big.Int `json:"storageprice"`

	// the cost per byte of uploading data to the host
	UploadBandwidthPrice *big.Int `json:"uploadbandwidthprice"`

	// the revision number of host setting
	RevisionNumber uint64 `json:"revisionnumber"`
	Version        string `json:"version"`
}

// HostDBScan represents a single scan event.
type HostDBScan struct {
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// HostDBScans represents a sortable slice of scans.
type HostDBScans []HostDBScan

func (s HostDBScans) Len() int           { return len(s) }
func (s HostDBScans) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s HostDBScans) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
