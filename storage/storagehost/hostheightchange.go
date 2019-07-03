package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/rlp"
)

// subscribeChainChangeEvent will receive changes on the block chain (blocks added / reverted)
// once received, a function will be triggered to analyze those blocks
func (h *StorageHost) subscribeChainChangEvent() {
	if err := h.tm.Add(); err != nil {
		return
	}
	defer h.tm.Done()

	chainChanges := make(chan core.ChainChangeEvent, 1)
	h.ethBackend.SubscribeChainChangeEvent(chainChanges)

	for {
		select {
		case change := <-chainChanges:
			h.hostBlockHeightChange(change)
		case <-h.tm.StopChan():
			return
		}
	}
}

//hostBlockHeightChange handle when a new block is generated or a block is rolled back
func (h *StorageHost) hostBlockHeightChange(cce core.ChainChangeEvent) {

	h.lock.Lock()
	defer h.lock.Unlock()

	//Handling rolled back blocks
	h.revertedBlockHashesStorageResponsibility(cce.RevertedBlockHashes)

	//Block executing the main chain
	taskItems := h.applyBlockHashesStorageResponsibility(cce.AppliedBlockHashes)
	for i := range taskItems {
		go h.handleTaskItem(taskItems[i])
	}

	err := h.syncConfig()
	if err != nil {
		h.log.Error("could not save during ProcessConsensusChange", "err", err)
	}
}

//applyBlockHashesStorageResponsibility block executing the main chain
func (h *StorageHost) applyBlockHashesStorageResponsibility(blocks []common.Hash) []common.Hash {
	var taskItems []common.Hash
	for _, blockApply := range blocks {
		//apply contract transaction
		ContractCreateIDsApply, revisionIDsApply, storageProofIDsApply, number, errGetBlock := h.getAllStorageContractIDsWithBlockHash(blockApply)
		if errGetBlock != nil {
			continue
		}

		//Traverse all contract transactions and modify storage responsibility status
		for _, id := range ContractCreateIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.CreateContractConfirmed = true
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		//Traverse all revision transactions and modify storage responsibility status
		for key, value := range revisionIDsApply {
			so, errGet := getStorageResponsibility(h.db, key)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			if len(so.StorageContractRevisions) < 1 {
				h.log.Error("Storage contract cannot get revisions", "id", so.id())
				continue
			}
			//To prevent vicious attacks, determine the consistency of the revision number.
			if value == so.StorageContractRevisions[len(so.StorageContractRevisions)-1].NewRevisionNumber {
				so.StorageRevisionConfirmed = true
			}
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		//Traverse all storageProof transactions and modify storage responsibility status
		for _, id := range storageProofIDsApply {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageProofConfirmed = true
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		if number != 0 {
			h.blockHeight++
		}
		existingItems, err := getHeight(h.db, h.blockHeight)
		if err != nil {
			continue
		}

		// From the existing items, pull out a storage responsibility.
		knownActionItems := make(map[common.Hash]struct{})
		responsibilityIDs := make([]common.Hash, len(existingItems)/common.HashLength)
		for i := 0; i < len(existingItems); i += common.HashLength {
			copy(responsibilityIDs[i/common.HashLength][:], existingItems[i:i+common.HashLength])
		}
		for _, soid := range responsibilityIDs {
			_, exists := knownActionItems[soid]
			if !exists {
				taskItems = append(taskItems, soid)
				knownActionItems[soid] = struct{}{}
			}
		}

	}
	return taskItems
}

//revertedBlockHashesStorageResponsibility handling rolled back blocks
func (h *StorageHost) revertedBlockHashesStorageResponsibility(blocks []common.Hash) {
	for _, blockReverted := range blocks {
		//Rollback contract transaction
		ContractCreateIDs, revisionIDs, storageProofIDs, number, errGetBlock := h.getAllStorageContractIDsWithBlockHash(blockReverted)
		if errGetBlock != nil {
			h.log.Error("Failed to get the data from the block as expected ", "err", errGetBlock)
			continue
		}

		//Traverse all ContractCreate transactions and modify storage responsibility status
		for _, id := range ContractCreateIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.CreateContractConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		//Traverse all revision transactions and modify storage responsibility status
		for key := range revisionIDs {
			so, errGet := getStorageResponsibility(h.db, key)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageRevisionConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		//Traverse all storageProof transactions and modify storage responsibility status
		for _, id := range storageProofIDs {
			so, errGet := getStorageResponsibility(h.db, id)
			//This transaction is not involved by the local node, so it should be skipped
			if errGet != nil {
				continue
			}
			so.StorageProofConfirmed = false
			errPut := putStorageResponsibility(h.db, so.id(), so)
			if errPut != nil {
				h.log.Error("Failed to put storage responsibility", "err", errPut)
				continue
			}
		}

		if number != 0 && h.blockHeight > 1 {
			h.blockHeight--
		}
	}
}

//getAllStorageContractIDsWithBlockHash analyze the block structure and get three kinds of transaction collections: contractCreate, revision, and proof、block height.
func (h *StorageHost) getAllStorageContractIDsWithBlockHash(blockHash common.Hash) (ContractCreateIDs []common.Hash, revisionIDs map[common.Hash]uint64, storageProofIDs []common.Hash, number uint64, errGet error) {
	revisionIDs = make(map[common.Hash]uint64)
	precompiled := vm.PrecompiledEVMFileContracts
	block, err := h.ethBackend.GetBlockByHash(blockHash)
	if err != nil {
		errGet = err
		return
	}
	number = block.NumberU64()
	txs := block.Transactions()
	for _, tx := range txs {
		p, ok := precompiled[*tx.To()]
		if !ok {
			continue
		}
		switch p {
		case vm.ContractCreateTransaction:
			var sc types.StorageContract
			err := rlp.DecodeBytes(tx.Data(), &sc)
			if err != nil {
				h.log.Error("Error when serializing storage contract:", "err", err)
				continue
			}
			ContractCreateIDs = append(ContractCreateIDs, sc.RLPHash())
		case vm.CommitRevisionTransaction:
			var scr types.StorageContractRevision
			err := rlp.DecodeBytes(tx.Data(), &scr)
			if err != nil {
				h.log.Error("Error when serializing revision:", "err", err)
				continue
			}
			revisionIDs[scr.ParentID] = scr.NewRevisionNumber
		case vm.StorageProofTransaction:
			var sp types.StorageProof
			err := rlp.DecodeBytes(tx.Data(), &sp)
			if err != nil {
				h.log.Error("Error when serializing proof:", "err", err)
				continue
			}
			storageProofIDs = append(storageProofIDs, sp.ParentID)
		default:
			continue
		}
	}
	return
}
