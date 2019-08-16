// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import (
	"context"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
)

type FakeClientBackend struct{}

func (fc *FakeClientBackend) Online() bool {
	return true
}

func (fc *FakeClientBackend) Syncing() bool {
	return false
}

func (fc *FakeClientBackend) GetStorageHostSetting(hostEnodeID enode.ID, hostEnodeURL string, config *HostExtConfig) error {
	config = &HostExtConfig{
		AcceptingContracts: true,
	}
	return nil
}

func (fc *FakeClientBackend) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (fc *FakeClientBackend) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return types.Transactions{}, nil
}

func (fc *FakeClientBackend) SetupConnection(enodeURL string) (Peer, error) {
	return nil, nil
}

func (fc *FakeClientBackend) AccountManager() *accounts.Manager {
	return nil
}

func (fc *FakeClientBackend) ChainConfig() *params.ChainConfig {
	return nil
}

func (fc *FakeClientBackend) CurrentBlock() *types.Block {
	return nil
}

func (fc *FakeClientBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return nil
}

func (fc *FakeClientBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return nil, nil
}

func (fc *FakeClientBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return 0, nil
}

func (fc *FakeClientBackend) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (fc *FakeClientBackend) GetHostAnnouncementWithBlockHash(blockHash common.Hash) (hostAnnouncements []types.HostAnnouncement, number uint64, errGet error) {
	return nil, 0, nil
}

func (fc *FakeClientBackend) GetPaymentAddress() (common.Address, error) {
	return common.Address{}, nil
}

func (fc *FakeClientBackend) TryToRenewOrRevise(hostID enode.ID) bool {
	return false
}

func (fc *FakeClientBackend) RevisionOrRenewingDone(hostID enode.ID) {}

func (fc *FakeClientBackend) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (fc *FakeClientBackend) SelfEnodeURL() string {
	return ""
}
