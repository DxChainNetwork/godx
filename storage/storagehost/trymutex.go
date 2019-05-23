// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"sync"
	"time"
)

// TryMutex provides a mutex that allows you to attempt to grab a mutex, and
// then fail if the mutex is either not grabbed immediately or is not grabbed
// by the specified duration.
type TryMutex struct {
	once sync.Once
	lock chan struct{}
}

// init will create the channel that manages the lock.
func (tm *TryMutex) init() {
	tm.lock = make(chan struct{}, 1)
	tm.lock <- struct{}{}
}

// Lock grabs a lock on the TryMutex, blocking until the lock is obtained.
func (tm *TryMutex) Lock() {
	tm.once.Do(tm.init)

	<-tm.lock
}

// TryLock grabs a lock on the TryMutex, returning an error if the mutex is
// already locked.
func (tm *TryMutex) TryLock() bool {
	tm.once.Do(tm.init)
	select {
	case <-tm.lock:
		return true
	default:
		return false
	}
}

// TryLockTimed grabs a lock on the TryMutex, returning an error if the mutex
// is not grabbed after the provided duration.
func (tm *TryMutex) TryLockTimed(t time.Duration) bool {
	tm.once.Do(tm.init)

	// For very small t (mostly t == 0), it's possible that the timer below
	// could fire before we acquire the lock, even if the lock is actually
	// available. To prevent this, do a non-blocking check for the lock before
	// racing the timer.
	select {
	case <-tm.lock:
		return true
	default:
	}

	timer := time.NewTimer(t)
	select {
	case <-tm.lock:
		timer.Stop()
		return true
	case <-timer.C:
		return false
	}
}

// Unlock releases a lock on the TryMutex.
func (tm *TryMutex) Unlock() {
	tm.once.Do(tm.init)

	select {
	case tm.lock <- struct{}{}:
		// Success - do nothing.
	default:
		panic("unlock called when TryMutex is not locked")
	}
}
