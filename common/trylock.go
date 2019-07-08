// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package common

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// TryLock provides TryLock method in addition to sync.Mutex
type TryLock struct {
	lock sync.Mutex
}

const mutexLocked = 1 << iota

// Lock lock the TryLock
func (tl *TryLock) Lock() {
	tl.lock.Lock()
}

// Unlock unlock the TryLock
func (tl *TryLock) Unlock() {
	tl.lock.Unlock()
}

// TryLock try to lock the lock. if succeed, return true. Else return false immediately
func (tl *TryLock) TryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&tl.lock)), 0, mutexLocked)
}

// Lock locks all locks at once.
// The Lock will not cause dead lock
func Lock(locks ...*TryLock) {
	// If no locks input, simply return
	if len(locks) == 0 {
		return
	}
	// Pick the first lock as the hard lock.
	// Hard lock is the lock that is lock last check by other goroutine
	// So the hard lock is the first to check in lockHelper function
	hardLock := 0
	// If the lockHelper function return -1, means that all locks are locked, return.
	// If not -1, the returned value is the index of the lock that cannot be locked.
	// assign the value to hard lock so that the locked lock will be the first to be
	// checked in the next iteration
	for hardLock != -1 {
		hardLock = lockHelper(locks, hardLock)
	}
	return
}

// lockHelper is the helper function of Lock. It tries to lock all locks.
// First it tries to lock the lock that cannot be locked in the last time, and then
// loop over locks and try to lock. If one lock cannot be locked, unlock all previously
// locked locks.
func lockHelper(locks []*TryLock, hardLock int) int {
	// lock the hardLock
	locks[hardLock].lock.Lock()
	for i, lock := range locks {
		// Skip the hardLock
		if i == hardLock {
			continue
		}
		// Try to lock the lock. If cannot be locked, unlock all previously locked locks,
		// and return the blocked index.
		if !lock.TryLock() {
			// Previously we locked locks with index from 0 to i-1. Unlock them
			for j := 0; j != i; j++ {
				locks[j].lock.Unlock()
			}
			// If the hardLock index has not been looped, also unlock the hardLock
			if hardLock > i {
				locks[hardLock].Unlock()
			}
			// return the index that is blocked for this function to be used in the
			// next iteration.
			return i
		}
	}
	// All locks has been locked. Return -1 so that the Lock function could be returned.
	return -1
}
