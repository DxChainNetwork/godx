// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
)

func (cm *ContractManager) subscribeChainChangeEvent() {
	cm.wg.Add(1)
	defer cm.wg.Done()

	chainChanges := make(chan core.ChainChangeEvent, 100)
	cm.b.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <-chainChanges:
			cm.analyzeChainEventChange(change)
		case <-cm.quit:
			return
		}
	}
}

// analyzeChainEventChange will analyze block changing event and update the corresponded
// storage contract manager field (blockHeight)
func (cm *ContractManager) analyzeChainEventChange(change core.ChainChangeEvent) {
	revert := len(change.RevertedBlockHashes)
	apply := len(change.AppliedBlockHashes)

	for i := 0; i < revert; i++ {
		cm.lock.Lock()
		cm.blockHeight--
		cm.lock.Unlock()

		if cm.blockHeight < 0 {
			cm.log.Error("the block height stores in the storage contract manager should be positive")
			cm.lock.Lock()
			cm.blockHeight = 0
			cm.lock.Unlock()
		}
	}

	for i := 0; i < apply; i++ {
		cm.lock.Lock()
		cm.blockHeight++
		cm.lock.Unlock()
	}

	// get block information
	for _, hash := range change.AppliedBlockHashes {
		txs, err := cm.b.GetTxByBlockHash(hash)
		if err != nil {
			cm.log.Error("failed to get transaction information from the block")
			continue
		}
		cm.analyzeTransactions(txs)
	}
}

// analyzeTransactions will get the transaction from the block, analyze them
// to check if there are any contract needs to be recovered
func (cm *ContractManager) analyzeTransactions(txs types.Transactions) {
	// TODO (mzhang): IMPLEMENTATION NOT FINISHED YET
}
