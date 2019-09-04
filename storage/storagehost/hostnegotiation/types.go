// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

// NegotiationProtocol contains methods that are used in contract negotiation
// upload negotiation and download negotiation
type NegotiationProtocol interface {
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
	RollBackStorageResponsibility(sr storagehost.StorageResponsibility) error
	RollBackConnectionType(sp storage.Peer)
}

type contractNegotiationData struct {
	clientPubKey *ecdsa.PublicKey
	hostPubKey   *ecdsa.PublicKey
	account      accounts.Account
	wallet       accounts.Wallet
}

type uploadNegotiationData struct {
	srSnapshot       storagehost.StorageResponsibility
	newRoots         []common.Hash
	sectorsChanged   map[uint64]struct{}
	bandwidthRevenue common.BigInt
	sectorGained     []common.Hash
	gainedSectorData [][]byte
	storageRevenue   common.BigInt
	newDeposit       common.BigInt
	newMerkleRoot    common.Hash
}

type downloadNegotiationData struct {
}
