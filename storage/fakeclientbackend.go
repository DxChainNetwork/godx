// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package fakedata

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
	"github.com/DxChainNetwork/godx/storage"
)

type fakeClientBackend struct{}

func (fc *fakeClientBackend) Online() bool {
	return true
}

func (fc *fakeClientBackend) Syncing() bool {
	return false
}

func (fc *fakeClientBackend) GetStorageHostSetting(hostEnodeID enode.ID, hostEnodeURL string, config *storage.HostExtConfig) error {
	config = &storage.HostExtConfig{
		AcceptingContracts: true,
	}
	return nil
}

func (fc *fakeClientBackend) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (fc *fakeClientBackend) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return types.Transactions{}, nil
}

func (fc *fakeClientBackend) SetupConnection(enodeURL string) (storage.Peer, error) {
	return nil, nil
}

func (fc *fakeClientBackend) AccountManager() *accounts.Manager {
	return nil
}

func (fc *fakeClientBackend) ChainConfig() *params.ChainConfig {
	return nil
}

func (fc *fakeClientBackend) CurrentBlock() *types.Block {
	return nil
}

func (fc *fakeClientBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return nil
}

func (fc *fakeClientBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return nil, nil
}

func (fc *fakeClientBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return 0, nil
}

func (fc *fakeClientBackend) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (fc *fakeClientBackend) GetHostAnnouncementWithBlockHash(blockHash common.Hash) (hostAnnouncements []types.HostAnnouncement, number uint64, errGet error) {
	return nil, 0, nil
}

func (fc *fakeClientBackend) GetPaymentAddress() (common.Address, error) {
	return common.Address{}, nil
}

func (fc *fakeClientBackend) TryToRenewOrRevise(hostID enode.ID) bool {
	return false
}

func (fc *fakeClientBackend) RevisionOrRenewingDone(hostID enode.ID) {}

func (fc *fakeClientBackend) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (fc *fakeClientBackend) SelfEnodeURL() string {
	return ""
}
