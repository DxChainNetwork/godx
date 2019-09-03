// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

var (
	errObligationLocked = errors.New("storage responsibility has been locked")
)

//If not locked, create a new one
func (h *StorageHost) checkAndLockStorageResponsibility(srID common.Hash) {
	h.lock.Lock()
	tl, exists := h.lockedStorageResponsibility[srID]
	if !exists {
		tl = new(TryMutex)
		h.lockedStorageResponsibility[srID] = tl
	}
	h.lock.Unlock()

	tl.Lock()
}

//Try to lock this storage obligation
func (h *StorageHost) checkAndTryLockStorageResponsibility(srID common.Hash, timeout time.Duration) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	tl, exists := h.lockedStorageResponsibility[srID]
	if !exists {
		tl = new(TryMutex)
		h.lockedStorageResponsibility[srID] = tl
	}

	if tl.TryLockTimed(timeout) {
		return nil
	}
	return errObligationLocked
}

//If it exists, unlock it
func (h *StorageHost) checkAndUnlockStorageResponsibility(srID common.Hash) {
	h.lock.Lock()
	defer h.lock.Unlock()

	tl, exists := h.lockedStorageResponsibility[srID]
	if !exists {
		return
	}
	tl.Unlock()

}

func (h *StorageHost) deleteLockedStorageResponsibility(srID common.Hash) {
	h.lock.Lock()
	defer h.lock.Unlock()

	delete(h.lockedStorageResponsibility, srID)
}
