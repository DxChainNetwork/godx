// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehost

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

func (h *StorageHost) CheckAndUpdateConnection(peerNode *enode.Node) {
	h.ethBackend.CheckAndUpdateConnection(peerNode)
}

func (h *StorageHost) GetFinancialMetrics() HostFinancialMetrics {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.financialMetrics
}

func (h *StorageHost) GetHostConfig() storage.HostIntConfig {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.config
}

func (h *StorageHost) GetStateDB() (*state.StateDB, error) {
	return h.ethBackend.GetBlockChain().State()
}

func (h *StorageHost) GetBlockHeight() uint64 {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.blockHeight
}

// GetStorageResponsibility will be used to get the storage responsibility information
// based on the storage contractID provided
func (h *StorageHost) GetStorageResponsibility(storageContractID common.Hash) (StorageResponsibility, error) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return getStorageResponsibility(h.db, storageContractID)

}

func (h *StorageHost) FindWallet(account accounts.Account) (accounts.Wallet, error) {
	return h.ethBackend.AccountManager().Find(account)
}

func (h *StorageHost) InsertContract(peerNode string, contractID common.Hash) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.clientToContract[peerNode] = contractID
}

func (h *StorageHost) IsAcceptingContract() bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.config.AcceptingContracts
}

func (h *StorageHost) SetStatic(node *enode.Node) {
	h.ethBackend.SetStatic(node)
}

func (h *StorageHost) FinalizeStorageResponsibility(sr StorageResponsibility) error {
	lockErr := h.checkAndTryLockStorageResponsibility(sr.id(), storage.ResponsibilityLockTimeout)
	if lockErr != nil {
		return lockErr
	}
	defer h.checkAndUnlockStorageResponsibility(sr.id())

	if err := h.insertStorageResponsibility(sr); err != nil {
		h.deleteLockedStorageResponsibility(sr.id())
		return err
	}
	return nil
}

func (h *StorageHost) RollBackStorageResponsibility(sr StorageResponsibility) error {
	lockErr := h.checkAndTryLockStorageResponsibility(sr.id(), storage.ResponsibilityLockTimeout)
	if lockErr != nil {
		return lockErr
	}
	defer h.checkAndUnlockStorageResponsibility(sr.id())

	if err := h.deleteStorageResponsibilities([]common.Hash{sr.id()}); err != nil {
		return err
	}

	h.deleteLockedStorageResponsibility(sr.id())
	return nil
}

func (h *StorageHost) RollBackConnectionType(sp storage.Peer) {
	h.ethBackend.CheckAndUpdateConnection(sp.PeerNode())
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.clientToContract, sp.PeerNode().String())
}
