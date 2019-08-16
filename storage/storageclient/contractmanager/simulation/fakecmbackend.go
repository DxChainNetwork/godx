// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

type FakeContractManagerBackend struct {
	TestType bool
}

func (fc *FakeContractManagerBackend) Syncing() bool {
	return false
}

func (fc *FakeContractManagerBackend) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (fc *FakeContractManagerBackend) GetPaymentAddress() (common.Address, error) {
	return common.Address{}, nil
}

func (fc *FakeContractManagerBackend) AccountManager() storage.ClientAccountManager {
	return nil
}

func (fc *FakeContractManagerBackend) SetupConnection(enodeURL string) (storage.Peer, error) {
	return nil, nil
}

func (fc *FakeContractManagerBackend) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (fc *FakeContractManagerBackend) TryToRenewOrRevise(hostID enode.ID) bool {
	return false
}

func (fc *FakeContractManagerBackend) RevisionOrRenewingDone(hostID enode.ID) {}

func (fc *FakeContractManagerBackend) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}
