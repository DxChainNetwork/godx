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
		lock  sync.Mutex
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

func newSectorLocks() (sls *sectorLocks) {
	return &sectorLocks{
		locks: make(map[sectorID]*sectorLock),
	}
}

// lock tries to lock the lock with the specified id. Block until the lock is released
func (sls *sectorLocks) lockSector(id sectorID) {
	sls.lock.Lock()
	defer sls.lock.Unlock()
	// If the id is in the map, increment the waiting.
	// If not in map, create a new lock
	l, exist := sls.locks[id]
	if exist {
		atomic.AddUint32(&l.waiting, 1)
	} else {
		l = &sectorLock{
			waiting: 1,
		}
		sls.locks[id] = l
	}
	l.lock()
}

// unlock unlock the sector with the id
func (sls *sectorLocks) unlockSector(id sectorID) {
	sls.lock.Lock()
	defer sls.lock.Unlock()

	// If the lock have waiting == 0, delete the lock from the map
	l, exist := sls.locks[id]
	if !exist {
		// unlock a not locked sectorLock, simply return
		return
	}
	if l.waiting <= 1 {
		// If the waiting number is smaller or equal to one, simply release the lock
		// from the map
		delete(sls.locks, id)
	}
	l.waiting -= 1
	l.unlock()
}