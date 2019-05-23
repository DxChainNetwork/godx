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
	Host            HostDBEntry
}

// An Allowance dictates how much the Renter is allowed to spend in a given
// period. Note that funds are spent on both storage and bandwidth.
type Allowance struct {
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

// A HostDBEntry represents one host entry in the Renter's host DB. It
// aggregates the host's external settings and metrics with its public key.
type HostDBEntry struct {
	HostExternalSettings

	// FirstSeen is the last block height at which this host was announced.
	FirstSeen uint64 `json:"firstseen"`

	// Measurements that have been taken on the host. The most recent
	// measurements are kept in full detail, historic ones are compressed into
	// the historic values.
	HistoricDowntime int64       `json:"historicdowntime"`
	HistoricUptime   int64       `json:"historicuptime"`
	ScanHistory      HostDBScans `json:"scanhistory"`

	// Measurements that are taken whenever we interact with a host.
	HistoricFailedInteractions     float64 `json:"historicfailedinteractions"`
	HistoricSuccessfulInteractions float64 `json:"historicsuccessfulinteractions"`
	RecentFailedInteractions       float64 `json:"recentfailedinteractions"`
	RecentSuccessfulInteractions   float64 `json:"recentsuccessfulinteractions"`

	LastHistoricUpdate uint64 `json:"lasthistoricupdate"`

	// Measurements related to the IP subnet mask.
	IPNets          []string  `json:"ipnets"`
	LastIPNetChange time.Time `json:"lastipnetchange"`

	// The public key of the host, stored separately to minimize risk of certain
	// MitM based vulnerabilities.
	PublicKey ecdsa.PublicKey `json:"publickey"`

	// Filtered says whether or not a HostDBEntry is being filtered out of the
	// filtered hosttree due to the filter mode of the hosttree
	Filtered bool `json:"filtered"`
}

// HostExternalSettings are the parameters advertised by the host. These
// are the values that the renter will request from the host in order to
// build its database.
type HostExternalSettings struct {
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
type HostDBScan struct {
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// HostDBScans represents a sortable slice of scans.
type HostDBScans []HostDBScan

func (s HostDBScans) Len() int           { return len(s) }
func (s HostDBScans) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s HostDBScans) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
