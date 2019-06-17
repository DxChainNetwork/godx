// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
)

// update is the data structure used for all storage manager operations
type update interface {
	// str returns a brief string of what the update is doing
	str() string

	// lockResource locks the resources effected by the update in the recovery process
	lockResource(manager *storageManager) error

	// recordIntent record the intent in the wal, and register the transaction to
	// the update itself
	recordIntent(manager *storageManager) error

	// prepare prepare the data and then commit. target is defined at defaults.go
	// which defines three scenarios this function is called. normal execution /
	// recover commmitted txn / recover uncommitted txn
	prepare(manager *storageManager, target uint8) error

	// process do the actual updates. If any error happened, return
	process(manager *storageManager, target uint8) error

	// release handle the error, and release the transaction. During error handling,
	// also reverse or redo as needed.
	release(manager *storageManager, err *updateError) error
}

// decodeFromTransaction decode and create an update from the transaction
func decodeFromTransaction(txn *writeaheadlog.Transaction) (up update, err error) {
	switch txn.Operations[0].Name {
	case opNameAddStorageFolder:
		up, err = decodeAddStorageFolderUpdate(txn)
	case opNameAddSector:
		up, err = decodeAddSectorUpdate(txn)
	case opNameAddSectorBatch:
		up, err = decodeAddSectorBatchUpdate(txn)
	case opNameDeleteSectorBatch:
		up, err = decodeDeleteSectorBatchUpdate(txn)
	case opNameExpandFolder:
		up, err = decodeExpandFolderUpdate(txn)
	default:
		err = errInvalidTransactionType
	}
	return
}

// prepareProcessReleaseUpdate is called with a goroutine to prepare, process, release the update.
func (sm *storageManager) prepareProcessReleaseUpdate(up update, target uint8) (upErr *updateError) {
	// register the error handling
	upErr = &updateError{}
	defer func() {
		if err := up.release(sm, upErr); err != nil {
			upErr = upErr.setReleaseError(err)
			sm.logError(up, upErr)
		}
		return
	}()
	// prepare the update
	if err := up.prepare(sm, target); err != nil {
		upErr = upErr.setPrepareError(err)
		return
	}
	if sm.stopped() {
		upErr = upErr.setPrepareError(errStopped)
		return
	}
	// process the update
	if err := up.process(sm, target); err != nil {
		upErr = upErr.setProcessError(err)
		return
	}
	if sm.stopped() || sm.disrupter.disrupt("mock process disrupted") {
		upErr = upErr.setProcessError(errStopped)
		return
	}
	return
}
