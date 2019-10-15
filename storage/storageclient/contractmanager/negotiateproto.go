// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

// FindWallet implements the ContractCreateProtocol and ContractRenewProtocol, used
// to find the storage client's wallet
func (cm *ContractManager) FindWallet(account accounts.Account) (accounts.Wallet, error) {
	return cm.b.AccountManager().Find(account)
}

// SetupConnection implements ContractCreate and ContractRenew Protocol, used to
// establish connection between storage client and storage host
func (cm *ContractManager) SetupConnection(enodeURL string) (storage.Peer, error) {
	return cm.b.SetupConnection(enodeURL)
}

// InsertContract implements ContractCreate and ContractRenew Protocol, used to
// insert the contract into active contract list
func (cm *ContractManager) InsertContract(ch contractset.ContractHeader, roots []common.Hash) (storage.ContractMetaData, error) {
	return cm.GetStorageContractSet().InsertContract(ch, roots)
}

// GetAccountManager implements the negotiation protocol, used to retrieve
// the account manager
func (cm *ContractManager) GetAccountManager() storage.ClientAccountManager {
	return cm.b.AccountManager()
}

// SendStorageContractCreateTx implements negotiation protocol, used to send
// the storage contract create transaction
func (cm *ContractManager) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return cm.b.SendStorageContractCreateTx(clientAddr, input)
}

// CheckAndUpdateConnection implements negotiation protocol, used to check
// and update the connection to static connection between the storage host
// and the storage client.
func (cm *ContractManager) CheckAndUpdateConnection(peerNode *enode.Node) {
	cm.b.CheckAndUpdateConnection(peerNode)
}

// IncrementSuccessfulInteractions implements the negotiation protocol, used to
// increase the interaction score
func (cm *ContractManager) IncrementSuccessfulInteractions(id enode.ID, interactionType storagehostmanager.InteractionType) {
	cm.hostManager.IncrementSuccessfulInteractions(id, interactionType)
}

// IncrementFailedInteractions implements the negotiation protocol, used to
// decrease the interaction score
func (cm *ContractManager) IncrementFailedInteractions(id enode.ID, interactionType storagehostmanager.InteractionType) {
	cm.hostManager.IncrementFailedInteractions(id, interactionType)
}
