// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rpc"
)

// HostBackend is the interface for Ethereum to be used in storage host module
type HostBackend interface {
	APIs() []rpc.API
	SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription
	GetBlockByHash(blockHash common.Hash) (*types.Block, error)
	GetBlockByNumber(number uint64) (*types.Block, error)
	GetBlockChain() *core.BlockChain
	AccountManager() *accounts.Manager
	SetStatic(node *enode.Node)
	CheckAndUpdateConnection(peerNode *enode.Node)
}

// AccountManager is the interface for account.Manager to be used in storage host module
type AccountManager interface {
	Find(accounts.Account) (accounts.Wallet, error)
	Wallets() []accounts.Wallet
}
