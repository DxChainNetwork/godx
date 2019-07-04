// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/core"
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

	cm.lock.Lock()
	// if chain got reverted, then decrement the blockHeight
	for i := 0; i < revert; i++ {
		cm.blockHeight--

		if cm.blockHeight < 0 {
			cm.log.Error("the block height stores in the storage contract manager should be positive")
			cm.blockHeight = 0
		}
	}

	// if new blocks applied to the block chain, then increment the blockHeight
	for i := 0; i < apply; i++ {
		cm.blockHeight++
	}

	if cm.blockHeight >= cm.currentPeriod+cm.rentPayment.Period {
		cm.currentPeriod += cm.rentPayment.Period
	}
	cm.lock.Unlock()

	// save the newest settings (blockHeight) persistently
	if err := cm.saveSettings(); err != nil {
		cm.log.Warn("failed to save the current contract manager settings while analyzing the chain change event", "err", err.Error())
	}

	// if the block chain finished syncing, start the contract maintenance routine
	if !cm.b.Syncing() {
		go cm.contractMaintenance()
	}
}
