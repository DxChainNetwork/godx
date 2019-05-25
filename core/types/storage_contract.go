// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package types

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

type StorageContractRLPHash interface {
	RLPHash() common.Hash
}

type HostAnnouncement struct {
	// Specifier  types.Specifier
	NetAddress string // host对外可访问的URI
	// PublicKey  ecdsa.PublicKey // 用于验证host对HostAnnouncement的签名的公钥 （可省去）

	Signature []byte
}

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
	Collateral    big.Int `json:"collateral"`
	MaxCollateral big.Int `json:"maxcollateral"`

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
	BaseRPCPrice           big.Int `json:"baserpcprice"`
	ContractPrice          big.Int `json:"contractprice"`
	DownloadBandwidthPrice big.Int `json:"downloadbandwidthprice"`
	SectorAccessPrice      big.Int `json:"sectoraccessprice"`
	StoragePrice           big.Int `json:"storageprice"`
	UploadBandwidthPrice   big.Int `json:"uploadbandwidthprice"`

	// Because the host has a public key, and settings are signed, and
	// because settings may be MITM'd, settings need a revision number so
	// that a renter can compare multiple sets of settings and determine
	// which is the most recent.
	RevisionNumber uint64 `json:"revisionnumber"`
	Version        string `json:"version"`
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
	HistoricDowntime time.Duration `json:"historicdowntime"`
	HistoricUptime   time.Duration `json:"historicuptime"`
	//ScanHistory      HostDBScans   `json:"scanhistory"`

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

type UnlockConditions struct {
	Timelock           uint64            `json:"timelock"`
	PublicKeys         []ecdsa.PublicKey `json:"publickeys"`
	SignaturesRequired uint64            `json:"signaturesrequired"`
}

type DxcoinCharge struct {
	Address common.Address
	Value   *big.Int
}

type DxcoinCollateral struct {
	DxcoinCharge
}

type StorageContract struct {
	// file part
	FileSize       uint64      `json:"filesize"`
	FileMerkleRoot common.Hash `json:"filemerkleroot"`
	WindowStart    uint64      `json:"windowstart"`
	WindowEnd      uint64      `json:"windowend"`

	// money part
	// original collateral
	RenterCollateral DxcoinCollateral `json:"renter_collateral"`
	HostCollateral   DxcoinCollateral `json:"host_collateral"`

	// temporary book while file upload and download
	ValidProofOutputs  []DxcoinCharge `json:"validproofoutputs"`
	MissedProofOutputs []DxcoinCharge `json:"missedproofoutputs"`

	//解锁条件
	UnlockHash common.Hash `json:"unlockhash"`

	RevisionNumber uint64 `json:"revisionnumber"`

	Signatures [][]byte
}

type StorageContractRevision struct {
	ParentID          common.Hash      `json:"parentid"`
	UnlockConditions  UnlockConditions `json:"unlockconditions"`
	NewRevisionNumber uint64           `json:"newrevisionnumber"`

	NewFileSize           uint64         `json:"newfilesize"`
	NewFileMerkleRoot     common.Hash    `json:"newfilemerkleroot"`
	NewWindowStart        uint64         `json:"newwindowstart"`
	NewWindowEnd          uint64         `json:"newwindowend"`
	NewValidProofOutputs  []DxcoinCharge `json:"newvalidproofoutputs"`
	NewMissedProofOutputs []DxcoinCharge `json:"newmissedproofoutputs"`
	NewUnlockHash         common.Hash    `json:"newunlockhash"`

	Signatures [][]byte
}

type StorageProof struct {
	ParentID common.Hash   `json:"parentid"`
	Segment  [64]byte      `json:"segment"`
	HashSet  []common.Hash `json:"hashset"`

	Signature []byte
}

func (ha HostAnnouncement) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		ha.NetAddress,
	})
}

func (fc StorageContract) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		fc.FileSize,
		fc.FileMerkleRoot,
		fc.WindowStart,
		fc.WindowEnd,
		fc.RenterCollateral,
		fc.HostCollateral,
		fc.ValidProofOutputs,
		fc.MissedProofOutputs,
		fc.RevisionNumber,
	})
}

func (fc StorageContract) ID() common.Hash {
	return common.Hash(fc.RLPHash())
}

// 计算UnlockCondition的hash，主要是保证一致性，确保是双方签订的
func (uc UnlockConditions) UnlockHash() common.Hash {
	return rlpHash([]interface{}{
		uc.Timelock,
		uc.PublicKeys,
		uc.SignaturesRequired,
	})
}

func (fcr StorageContractRevision) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		fcr.ParentID,
		fcr.UnlockConditions,
		fcr.NewRevisionNumber,
		fcr.NewFileSize,
		fcr.NewFileMerkleRoot,
		fcr.NewWindowStart,
		fcr.NewWindowEnd,
		fcr.NewValidProofOutputs,
		fcr.NewMissedProofOutputs,
	})
}

func (sp StorageProof) RLPHash() common.Hash {
	return rlpHash([]interface{}{
		sp.ParentID,
		sp.Segment,
		sp.HashSet,
	})
}

// 真正放进交易payload字段的内容是 StorageContractSet
type StorageContractSet struct {
	HostAnnounce            HostAnnouncement
	StorageContract         StorageContract
	StorageContractRevision StorageContractRevision
	StorageProof            StorageProof
}

func ResolveStorageContractSet(tx *Transaction) (*StorageContractSet, error) {
	payload := tx.Data()
	sc := StorageContractSet{}
	err := rlp.DecodeBytes(payload, &sc)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}
