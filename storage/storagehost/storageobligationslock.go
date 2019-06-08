// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"time"

	"github.com/DxChainNetwork/godx/common"

	"github.com/pkg/errors"
)

var (
	// errObligationLocked is returned if the file contract being requested is
	// currently locked. The lock can be in place if there is a storage proof
	// being submitted, if there is another storage client altering the contract, or if
	// there have been network connections with have not resolved yet.
	errObligationLocked = errors.New("the requested file contract is currently locked")
)

// managedLockStorageObligation puts a storage obligation under lock in the
// host.	managedLockStorageObligation将存储义务置于主机的锁定之下
func (h *StorageHost) managedLockStorageObligation(soid common.Hash) {

	h.lock.Lock()
	defer h.lock.Unlock()

	tl, exists := h.lockedStorageObligations[soid]
	if !exists {
		tl = new(TryMutex)
		h.lockedStorageObligations[soid] = tl
	}
	tl.Lock()

}

// managedTryLockStorageObligation attempts to put a storage obligation under
// lock, returning an error if the lock cannot be obtained.
// managedTryLockStorageObligation尝试将存储义务置于锁定状态，如果无法获取锁定则返回错误。
func (h *StorageHost) managedTryLockStorageObligation(soid common.Hash, timeout time.Duration) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	tl, exists := h.lockedStorageObligations[soid]
	if !exists {
		tl = new(TryMutex)
		h.lockedStorageObligations[soid] = tl
	}

	if tl.TryLockTimed(timeout) {
		return nil
	}
	return errObligationLocked
}

// managedUnlockStorageObligation takes a storage obligation out from under lock in
// the host.
// managedUnlockStorageObligation从主机锁定中解锁存储义务。
func (h *StorageHost) managedUnlockStorageObligation(soid common.Hash) {
	h.lock.Lock()
	defer h.lock.Unlock()

	tl, exists := h.lockedStorageObligations[soid]
	if !exists {
		return
	}
	tl.Unlock()

}
