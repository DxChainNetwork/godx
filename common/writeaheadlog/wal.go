package writeaheadlog

import (
	"sync"
	"unsafe"
)

// Wal is a golang implementation of write-ahead-log to perform ACID transactions
type Wal struct {
	// atomic fields. Change these values using atomic package
	nextTxnId         uint64         // Next TxnId to be executed. TxnId increment for each Txn.
	numUnfinishedTxns uint64         // Number of unfinished transactions
	syncStatus        uint8          // 0: No syncing thread; 1: syncing thread, empty queue; 2: syncing thread, non-empty queue
	syncStatePtr      unsafe.Pointer // pointing to a syncState object

	// Storage
	availablePages []uint64 // offset of pages available
	pageCount      uint64   // total number of pages
	logFile        file     // Log file
	logPath        string   // path of the log file

	// utils
	utils utilsSet
	wg    sync.WaitGroup // goroutine management
	mu    sync.Mutex     // protect storage fields
}
