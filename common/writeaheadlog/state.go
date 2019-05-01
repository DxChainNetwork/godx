package writeaheadlog

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// syncState is the state for syncing process
type syncState struct {
	err error
	mu  sync.RWMutex
}

// fSync start a file sync thread to sync the Wal content to logfile if no sync thread in progress.
func (w *Wal) fSync() error {
	// Load sync status
	ss := (*syncState)(atomic.LoadPointer(&w.syncStatePtr))

	// Read previous status and set to 2. If previous 0, meaning no threads, start a sync thread
	preStatus := atomic.SwapUint32(&w.syncStatus, 2)
	if preStatus == 0 {
		w.wg.Add(1)
		go w.threadedSync()
	}

	ss.mu.RLock()
	return ss.err
}

// threadedSync sync the w.Logfile
func (w *Wal) threadedSync() {
	defer w.wg.Done()
	for {
		stop := atomic.CompareAndSwapUint32(&w.syncStatus, 1, 0)
		if stop {
			return
		}
		newSS := new(syncState)
		newSS.mu.Lock()

		// w.syncStatus == 1 : file syncing in progress
		atomic.StoreUint32(&w.syncStatus, 1)
		oldSS := (*syncState)(atomic.SwapPointer(&w.syncStatePtr, unsafe.Pointer(newSS)))
		oldSS.err = w.logFile.Sync()
		oldSS.mu.Unlock()

		time.Sleep(time.Microsecond)
	}
}
