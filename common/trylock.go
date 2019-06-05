// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package common

// TryLock provides synchronization mechanism
type TryLock struct {
	lock chan struct{}
}

// NewTryLock will create and initialize a new TryLock object
// by occupying the lock channel
func NewTryLock() *TryLock {
	tl := &TryLock{
		lock: make(chan struct{}, 1),
	}
	tl.lock <- struct{}{}
	return tl
}

// Lock will empty the lock channel
func (tl *TryLock) Lock() {
	<-tl.lock
}

// Unlock will occupy the lock channel if the unlock
//  is called while the process is not locked, do nothing
func (tl *TryLock) Unlock() {
	select {
	case tl.lock <- struct{}{}:
	default:
	}
}

// TryToLock will try to perform lock operation if succeed, true will be returned
// otherwise, false will be returned
func (tl *TryLock) TryToLock() bool {
	select {
	case <-tl.lock:
		return true
	default:
		return false
	}
}
