// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/syndtr/goleveldb/leveldb"
	"strings"
)

type (
	// addSectorBatchUpdate is the update from function AddSectorBatch
	addSectorBatchUpdate struct {
		// ids is the function arguments
		ids []sectorID

		// sectors is the in memory log for the sector batch
		sectors []*sector

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	// addSectorBatchInitPersist is the initial persist
	addSectorBatchInitPersist struct {
		IDs []sectorID
	}

	// addSectorBatchAppendPersist is the persist for appended operations
	addSectorBatchAppendPersist struct {
		ID       sectorID
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

func (sm *storageManager) AddSectorBatch(roots []common.Hash) (err error) {
	return
}

func (sm *storageManager) createAddSectorBatchUpdate(ids []sectorID) (update *addSectorBatchUpdate) {
	return
}

// str defines the string representation of the update
func (update *addSectorBatchUpdate) str() (s string) {
	s = "Add sector batch\n[\n\t"
	s += strings.Join(common.Bytes2Hex(update.ids), ",\n\t")
	s += "\n]"
	return
}

func (update *addSectorBatchUpdate) recordIntent(manager *storageManager) (err error) {
	return
}

func (update *addSectorUpdate) prepare(manager *storageManager, target uint8) (err error) {
	return
}

func (update *addSectorUpdate) process(manager *storageManager, target uint8) (err error) {
	return
}

func (update *addSectorUpdate) release(manager *storageManager, upErr *updateError) (err error) {

}
