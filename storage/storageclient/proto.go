// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"crypto/ecdsa"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

const (
	Override = iota
	Append
)

var (
	DefaultMinSectors uint32 = 10
	DefaultNumSectors uint32 = 20
)

type (
	ContractParams struct {
		Allowance       Allowance
		HostEnodeUrl    string
		Funding         *big.Int
		StartHeight     uint64
		EndHeight       uint64
		ClientPublicKey ecdsa.PublicKey
		Host            StorageHostEntry
	}

	// An Allowance dictates how much the Renter is allowed to spend in a given
	// period. Note that funds are spent on both storage and bandwidth
	Allowance struct {
		Funds       *big.Int `json:"funds"`
		Hosts       uint64   `json:"hosts"`
		Period      uint64   `json:"period"`
		RenewWindow uint64   `json:"renewwindow"`

		// ExpectedStorage is the amount of data that we expect to have in a contract.
		ExpectedStorage uint64 `json:"expectedstorage"`

		// ExpectedUpload is the expected amount of data uploaded through the API,
		// before redundancy, per block.
		ExpectedUpload uint64 `json:"expectedupload"`

		// ExpectedDownload is the expected amount of data downloaded through the
		// API per block.
		ExpectedDownload uint64 `json:"expecteddownload"`

		// ExpectedRedundancy is the average redundancy of files being uploaded.
		ExpectedRedundancy float64 `json:"expectedredundancy"`
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

		NumStuckSegments uint64 `json:"num_stuck_segments"`

		DxPath dxdir.DxPath `json:"dx_path"`
	}

	// DownloadInfo provides information about a file that has been requested for download
	DownloadInfo struct {
		Destination     string `json:"destination"`     // The destination of the download.
		DestinationType string `json:"destinationtype"` // Can be "file", "memory buffer", or "http stream".
		Length          uint64 `json:"length"`          // The length requested for the download.
		Offset          uint64 `json:"offset"`          // The offset within the siafile requested for the download.
		SiaPath         string `json:"siapath"`         // The siapath of the file used for the download.

		Completed            bool      `json:"completed"`            // Whether or not the download has completed.
		EndTime              time.Time `json:"endtime"`              // The time when the download fully completed.
		Error                string    `json:"error"`                // Will be the empty string unless there was an error.
		Received             uint64    `json:"received"`             // Amount of data confirmed and decoded.
		StartTime            time.Time `json:"starttime"`            // The time when the download was started.
		StartTimeUnix        int64     `json:"starttimeunix"`        // The time when the download was started in unix format.
		TotalDataTransferred uint64    `json:"totaldatatransferred"` // Total amount of data transferred, including negotiation, etc.
	}

	// UploadParams contains the information used by the Client to upload a file
	FileUploadParams struct {
		Source      string
		DxPath      dxdir.DxPath
		ErasureCode erasurecode.ErasureCoder
		Mode        int
	}

	// FileInfo provides information about a file
	FileInfo struct {
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

	// define a host entry in the client's host db
	StorageHostEntry struct {
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

	// HostExternalSettings are the parameters advertised by the host. These
	// are the values that the renter will request from the host in order to
	// build its database.
	HostExternalSettings struct {
		// MaxBatchSize indicates the maximum size in bytes that a batch is
		// allowed to be. A batch is an array of revision actions; each
		// revision action can have a different number of bytes, depending on
		// the action, so the number of revision actions allowed depends on the
		// sizes of each.
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

		// Collateral is the amount of collateral that the host will put up for
		// storage in 'bytes per block', as an assurance to the renter that the
		// host really is committed to keeping the file. But, because the file
		// contract is created with no data available, this does leave the host
		// exposed to an attack by a wealthy renter whereby the renter causes
		// the host to lockup in-advance a bunch of funds that the renter then
		// never uses, meaning the host will not have collateral for other
		// clients.
		//
		// MaxCollateral indicates the maximum number of coins that a host is
		// willing to put into a file contract.
		Collateral    *big.Int `json:"collateral"`
		MaxCollateral *big.Int `json:"maxcollateral"`

		// ContractPrice is the number of coins that the renter needs to pay to
		// the host just to open a file contract with them. Generally, the price
		// is only to cover the siacoin fees that the host will suffer when
		// submitting the file contract revision and storage proof to the
		// blockchain.
		//
		// BaseRPC price is a flat per-RPC fee charged by the host for any
		// non-free RPC.
		//
		// 'Download' bandwidth price is the cost per byte of downloading data
		// from the host. This includes metadata such as Merkle proofs.
		//
		// SectorAccessPrice is the cost per sector of data accessed when
		// downloading data.
		//
		// StoragePrice is the cost per-byte-per-block in hastings of storing
		// data on the host.
		//
		// 'Upload' bandwidth price is the cost per byte of uploading data to
		// the host.
		BaseRPCPrice           *big.Int `json:"baserpcprice"`
		ContractPrice          *big.Int `json:"contractprice"`
		DownloadBandwidthPrice *big.Int `json:"downloadbandwidthprice"`
		SectorAccessPrice      *big.Int `json:"sectoraccessprice"`
		StoragePrice           *big.Int `json:"storageprice"`
		UploadBandwidthPrice   *big.Int `json:"uploadbandwidthprice"`

		// Because the host has a public key, and settings are signed, and
		// because settings may be MITM'd, settings need a revision number so
		// that a renter can compare multiple sets of settings and determine
		// which is the most recent.
		RevisionNumber uint64 `json:"revisionnumber"`
		Version        string `json:"version"`
	}

	// HostDBScan represents a single scan event.
	HostDBScan struct {
		Timestamp time.Time `json:"timestamp"`
		Success   bool      `json:"success"`
	}
)

// HostDBScans represents a sortable slice of scans.
type HostDBScans []HostDBScan

func (s HostDBScans) Len() int           { return len(s) }
func (s HostDBScans) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s HostDBScans) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
