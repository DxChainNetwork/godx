package storagehostmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
)

func (shm *StorageHostManager) SubscribeChainChangEvent() {
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	chainChanges := make(chan core.ChainChangeEvent)
	shm.b.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <- chainChanges:
			shm.analyzeChainEventChange(change)
		case <-shm.tm.StopChan():
			return
		}
	}
}

func (shm *StorageHostManager) analyzeChainEventChange(change core.ChainChangeEvent) {

	revert := len(change.RevertedBlockHashes)
	apply := len(change.AppliedBlockHashes)

	// update the block height
	for i := 0; i < revert; i++ {
		shm.blockHeight--
		if shm.blockHeight < 0 {
			shm.log.Error("the block height stores in StorageHostManager should be positive")
			shm.blockHeight = 0
			break
		}
	}

	for i := 0; i < apply; i++ {
		fmt.Println(shm.blockHeight)
		shm.blockHeight++
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

// TODO (mzhang): wait for the storage host announce data structure
func (shm *StorageHostManager) analyzeTransactions(txs types.Transactions) {
	//for _, tx := range txs {
	//
	//}
}
