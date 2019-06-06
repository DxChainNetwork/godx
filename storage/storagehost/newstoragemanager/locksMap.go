// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"sync"
	"sync/atomic"
)

type (
	// sectorLocks is the map from the sector ID to the sectorLock
	sectorLocks struct {
		locks map[sectorID]*sectorLock
		lk    sync.Mutex
	}

	sectorLock struct {
		tl      common.TryLock
		waiting uint32
	}
)

// lock locks sectorLock
func (sl *sectorLock) lock() {
	sl.tl.Lock()
}

// unlock unlock the sectorLock
func (sl *sectorLock) unlock() {
	sl.tl.Unlock()
}

// tryLock tries to lock the sectorLock
func (sl *sectorLock) tryLock() bool {
	return sl.tl.TryToLock()
}

// lock tries to lock the lock with the specified id. Block until the lock is released
func (sl *sectorLocks) lock(id sectorID) {
	sl.lk.Lock()
	defer sl.lk.Unlock()
	// If the id is in the map, increment the waiting.
	// If not in map, create a new lock
	l, exist := sl.locks[id]
	if exist {
		atomic.AddUint32(&l.waiting, 1)
	} else {
		l := &sectorLock{
			waiting: 1,
		}
		sl.locks[id] = l
	}

	l.lock()
}

// unlock unlock the sector with the id
func (sl *sectorLocks) unlock(id sectorID) {
	sl.lk.Lock()
	defer sl.lk.Unlock()

	// If the lock have waiting == 0, delete the lock from the map
	l, exist := sl.locks[id]
	if !exist {
		// unlock a not locked sectorLock, simply return
		return
	}
	if l.waiting <= 1 {
		// If the waiting number is smaller or equal to one, simply release the lock
		// from the map
		delete(sl.locks, id)
	}
	l.waiting -= 1
	l.unlock()
}
