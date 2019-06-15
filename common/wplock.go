package common

import (
	"sync"
	"sync/atomic"
)

// WP lock, write Prior lock is a lock built based on sync.WRLock.
// Just as WRLock, Write is an exclusive lock, and Read is a shared lock.
// WP adds an extra behaviour that read lock is also blocked while a write is waiting.
// So that an exclusive lock will not be starving when read lock is acquired extremely intensive
type WPLock struct {
	writeLock   sync.Mutex
	waiting     int32
	waitingLock sync.Mutex
	lock        sync.RWMutex
}

// Lock is the exclusive lock
func (wp *WPLock) Lock() {

	wp.waitingLock.Lock()
	if atomic.CompareAndSwapInt32(&wp.waiting, 0, 1) {
		wp.writeLock.Lock()
	} else {
		atomic.AddInt32(&wp.waiting, 1)
	}
	wp.waitingLock.Unlock()

	wp.lock.Lock()
}

// Unlock release the exclusive lock
func (wp *WPLock) Unlock() {

	wp.waitingLock.Lock()
	if atomic.CompareAndSwapInt32(&wp.waiting, 1, 0) {
		wp.writeLock.Unlock()
	} else {
		atomic.AddInt32(&wp.waiting, -1)
	}
	wp.waitingLock.Unlock()

	wp.lock.Unlock()
}

// Rlock locks for read
func (wp *WPLock) RLock() {
	wp.writeLock.Lock()
	wp.writeLock.Unlock()

	wp.lock.RLock()
}

// RUnlock unlocks the read
func (wp *WPLock) RUnlock() {
	wp.lock.RUnlock()
}
