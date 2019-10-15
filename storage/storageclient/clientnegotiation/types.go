// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package clientnegotiation

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

type ContractCreateProtocol interface {
	NegotiationError
	FindWallet(account accounts.Account) (accounts.Wallet, error)
	SetupConnection(enodeURL string) (storage.Peer, error)
	GetStorageContractSet() (contractSet *contractset.StorageContractSet)
	InsertContract(ch contractset.ContractHeader, roots []common.Hash) (storage.ContractMetaData, error)
	SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error)
}

type ContractRenewProtocol interface {
	NegotiationError
	FindWallet(account accounts.Account) (accounts.Wallet, error)
	SetupConnection(enodeURL string) (storage.Peer, error)
	SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error)
	InsertContract(ch contractset.ContractHeader, roots []common.Hash) (storage.ContractMetaData, error)
	GetStorageContractSet() (contractSet *contractset.StorageContractSet)
}

type UploadProtocol interface {
	NegotiationError
	GetContractBasedOnHostID(hostID enode.ID) (*contractset.Contract, error)
	GetAccountManager() storage.ClientAccountManager
	ContractReturn(c *contractset.Contract)
	GetBlockHeight() uint64
}

type DownloadProtocol interface {
	NegotiationError
	GetContractBasedOnHostID(hostID enode.ID) (*contractset.Contract, error)
	GetAccountManager() storage.ClientAccountManager
	ContractReturn(c *contractset.Contract)
}

type NegotiationError interface {
	IncrementSuccessfulInteractions(id enode.ID, interactionType storagehostmanager.InteractionType)
	IncrementFailedInteractions(id enode.ID, interactionType storagehostmanager.InteractionType)
	CheckAndUpdateConnection(peerNode *enode.Node)
}
