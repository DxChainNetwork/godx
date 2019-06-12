// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package common

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// TryLock provides synchronization mechanism
type TryLock struct {
	lock sync.Mutex
}

const mutexLocked = 1 << iota

// Lock will empty the lock channel
func (tl *TryLock) Lock() {
	tl.lock.Lock()
}

// Unlock will occupy the lock channel if the unlock
//  is called while the process is not locked, do nothing
func (tl *TryLock) Unlock() {
	tl.lock.Unlock()
}

// TryToLock will try to perform lock operation if succeed, true will be returned
// otherwise, false will be returned
func (tl *TryLock) TryToLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&tl.lock)), 0, mutexLocked)
}
