// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

// NegotiationProtocol contains methods that are used in contract negotiation
// upload negotiation and download negotiation
type Protocol interface {
	GetFinancialMetrics() storagehost.HostFinancialMetrics
	GetHostConfig() storage.HostIntConfig
	GetStateDB() (*state.StateDB, error)
	GetBlockHeight() uint64
	GetStorageResponsibility(storageContractID common.Hash) (storagehost.StorageResponsibility, error)
	FindWallet(account accounts.Account) (accounts.Wallet, error)
	CheckAndUpdateConnection(peerNode *enode.Node)
	InsertContract(peerNode string, contractID common.Hash)
	IsAcceptingContract() bool
	SetStatic(node *enode.Node)
	FinalizeStorageResponsibility(sr storagehost.StorageResponsibility) error
	RollBackCreateStorageResponsibility(sr storagehost.StorageResponsibility) error
	RollBackConnectionType(sp storage.Peer)
	ModifyStorageResponsibility(sr storagehost.StorageResponsibility, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error
	CheckAndSetStaticConnection(sp storage.Peer)
	RollbackUploadStorageResponsibility(oldSr storagehost.StorageResponsibility, sectorsGained []common.Hash, sectorsRemoved []common.Hash, removedSectorData [][]byte) error
	ReadSector(sectorRoot common.Hash) ([]byte, error)
}

type ContractCreateSession struct {
	ClientPubKey *ecdsa.PublicKey
	HostPubKey   *ecdsa.PublicKey
	Account      accounts.Account
	Wallet       accounts.Wallet
}

type UploadSession struct {
	SrSnapshot       storagehost.StorageResponsibility
	NewRoots         []common.Hash
	SectorsChanged   map[uint64]struct{}
	BandwidthRevenue common.BigInt
	SectorGained     []common.Hash
	GainedSectorData [][]byte
	StorageRevenue   common.BigInt
	NewDeposit       common.BigInt
	NewMerkleRoot    common.Hash
	MerkleProof      storage.UploadMerkleProof
}

type DownloadSession struct {
	SrSnapshot storagehost.StorageResponsibility
	NewRev     types.StorageContractRevision
}
