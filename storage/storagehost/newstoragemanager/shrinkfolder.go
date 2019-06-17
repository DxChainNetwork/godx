// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/syndtr/goleveldb/leveldb"
)

// shrinkFolderUpdate shrinks the folder to the target size.
// The processing of shrinkFolderUpdates acquires an exclusive lock from the module,
// so no worry about locks in this update. The update will relocate the sectors that
// needs to be relocated because the folder shrinks.
type (
	shrinkFolderUpdate struct {
		folderPath string

		prevNumSectors uint64

		targetNumSectors uint64

		relocates []sectorRelocation

		folders map[folderID]*storageFolder

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	sectorRelocation struct {
		ID sectorID

		PrevLocation sectorLocation

		AfterLocation sectorLocation
	}

	sectorLocation struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

func (update *shrinkFolderUpdate) lockResource(manager *storageManager) (err error) {
	manager.lock.RLock()
}
