// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
)

// subscribeChainChangeEvent will receive changes on the blockchain (blocks added / reverted)
// once received, a function will be triggered to analyze those blocks
func (shm *StorageHostManager) subscribeChainChangEvent() {
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	chainChanges := make(chan core.ChainChangeEvent, 100)
	shm.b.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <-chainChanges:
			shm.analyzeChainEventChange(change)
		case <-shm.tm.StopChan():
			return
		}
	}
}

// analyzeChainEventChange will analyze block changing event and update the corresponded
// storage host manager field (blockheight)
func (shm *StorageHostManager) analyzeChainEventChange(change core.ChainChangeEvent) {

	revert := len(change.RevertedBlockHashes)
	apply := len(change.AppliedBlockHashes)

	// TODO (mzhang): delete those Println after finished debugging
	fmt.Println("Applied Blocks", apply)
	fmt.Println("Reverted Blocks", revert)

	// update the block height
	for i := 0; i < revert; i++ {
		shm.lock.Lock()
		shm.blockHeight--
		shm.lock.Unlock()
		if shm.blockHeight < 0 {
			shm.log.Error("the block height stores in StorageHostManager should be positive")
			shm.lock.Lock()
			shm.blockHeight = 0
			shm.lock.Unlock()
			break
		}
	}

	for i := 0; i < apply; i++ {

		// TODO (mzhang): delete those Println after finished debugging
		fmt.Println(shm.blockHeight)
		shm.lock.Lock()
		shm.blockHeight++
		shm.lock.Unlock()
	}

	// get the block information
	for _, hash := range change.AppliedBlockHashes {
		txs, err := shm.b.GetTxByBlockHash(hash)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get transaction information from the block with the hash %v: %s",
				hash.String(), err.Error())
			shm.log.Error(errMsg)
			continue
		}
		shm.analyzeTransactions(txs)
	}
}

// analyzeTransactions will get the transaction from the block, analyze them
// to acquire any host announcement
func (shm *StorageHostManager) analyzeTransactions(txs types.Transactions) {
	// TODO (mzhang): wait for the storage host announce data structure
	//for _, tx := range txs {
	//
	//}
}
