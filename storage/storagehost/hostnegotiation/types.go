// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"crypto/ecdsa"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"
)

// NegotiationProtocol contains methods that are used in contract negotiation
// upload negotiation and download negotiation
type NegotiationProtocol interface {
	CheckAndUpdateConnection(peerNode *enode.Node)
	GetFinancialMetrics() storagehost.HostFinancialMetrics
	GetHostConfig() storage.HostIntConfig
	GetDB() *ethdb.LDBDatabase
	GetStateDB() (*state.StateDB, error)
	FindWallet(account accounts.Account) (accounts.Wallet, error)
	GetBlockHeight() uint64
	InsertContract(peerNode string, contractID common.Hash)
	DeleteContract(peerNode string)
	GetStorageResponsibility(db ethdb.Database, storageContractID common.Hash) (storagehost.StorageResponsibility, error)
	IsAcceptingContract() bool
	SetStatic(node *enode.Node)
	FinalizeStorageResponsibility(sr storagehost.StorageResponsibility) error
	RollBackStorageResponsibility(sr storagehost.StorageResponsibility) error
	RollBackConnectionType(sp storage.Peer)
}

type contractNegotiation struct {
	clientPubKey *ecdsa.PublicKey
	hostPubKey   *ecdsa.PublicKey
	account      accounts.Account
	wallet       accounts.Wallet
}

type uploadNegotiation struct {
}

type downloadNegotiation struct {
}
