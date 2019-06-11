package writeaheadlog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	metadataHeader = [24]byte{'D', 'x', 'C', 'h', 'a', 'i', 'n', ' ', 'W', 'r', 'i', 't', 'e', ' ', 'A', 'h',
		'e', 'a', 'd', ' ', 'L', 'o', 'g', '\n'}

	metadataVersion = [7]byte{'v', '1', '.', '0', '.', '0', '\n'}
)

const (
	stateInvalid uint8 = iota
	stateClean
	stateUnclean
)

const (
	metadataLength = len(metadataHeader) + len(metadataVersion) + 2
)

// Wal is a golang implementation of write-ahead-log to perform ACID transactions
type (
	Wal struct {
		// atomic fields. Change these values using atomic package
		nextTxnID         uint64         // Next TxnId to be executed. TxnId increment for each Txn.
		numUnfinishedTxns int64          // Number of unfinished transactions
		syncStatus        uint32         // 0: No syncing thread; 1: syncing thread, empty queue; 2: syncing thread, non-empty queue
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
)

// New create a new Wal file with the path as logfile
func New(path string) (*Wal, []*Transaction, error) {
	return newWal(path, &utilsProd{})
}

// newWal return a new Wal and unreleased transactions (committed and uncommitted)
func newWal(path string, utils utilsSet) (w *Wal, txns []*Transaction, err error) {
	newWal := &Wal{
		utils:   utils,
		logPath: path,
	}
	ss := new(syncState)
	ss.mu.Lock()
	atomic.StorePointer(&newWal.syncStatePtr, unsafe.Pointer(ss))

	// Read the log file
	data, err := utils.readFile(path)
	if err == nil {
		newWal.logFile, err = utils.openFile(path, os.O_RDWR, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to open wal logFile: %v", err)
		}
		txns, err = newWal.recoverWal(data)
		if err != nil {
			err = composeError(err, newWal.logFile.Close())
			return nil, nil, fmt.Errorf("unable to perform wal recovery: %v", err)
		}
		return newWal, txns, nil
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("open log file error: %v", err)
	}

	// Previous logfile not exist. Create a new log file and write metadata
	newWal.logFile, err = utils.create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create new log file: %v", err)
	}
	if err = writeMetadata(newWal.logFile); err != nil {
		return nil, nil, fmt.Errorf("cannot write metadata to logfile [%v]: %v", w.logPath, err)
	}
	return newWal, nil, nil
}

// Close close the Wal. Return error if there are still unfinished transactions
func (w *Wal) Close() error {
	var err1 error
	if unfinishedTxns := atomic.LoadInt64(&w.numUnfinishedTxns); unfinishedTxns != 0 {
		err1 = fmt.Errorf("wal closed with %d unfinished transactions", unfinishedTxns)
	}
	if err1 == nil && !w.utils.disrupt("UncleanShutdown") {
		err1 = w.writeRecoveryState(stateClean)
	}

	w.wg.Wait()
	err2 := w.logFile.Close()

	return composeError(err1, err2)
}

// CloseIncomplete close the Wal. Return number of unfinished transactions, and an error
// for closing the logfile.
func (w *Wal) CloseIncomplete() (int64, error) {
	w.wg.Wait()
	return atomic.LoadInt64(&w.numUnfinishedTxns), w.logFile.Close()
}

// writeWALMetadata writes metadata with stateUnclean to the input file.
func writeMetadata(f file) error {
	// Create the metadata.
	data := make([]byte, 0, len(metadataHeader)+len(metadataVersion)+2)
	data = append(data, metadataHeader[:]...)
	data = append(data, metadataVersion[:]...)
	// Penultimate byte is the recovery state, and final byte is a newline.
	data = append(data, byte(stateUnclean))
	data = append(data, byte('\n'))
	_, err := f.WriteAt(data, 0)

	return err
}

// managedReservePages request the pages for data, allocating new pages as necessary.
// Return the first page in the page chain
func (w *Wal) requestPages(data []byte) *page {
	// Calculate the number of pages needed for storing the data
	numPages := uint64(len(data) / maxPayloadSize)
	if len(data)%maxPayloadSize != 0 {
		numPages++
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if numPages > uint64(len(w.availablePages)) {
		w.allocateNewPages(numPages - uint64(len(w.availablePages)))
	}
	dataPages := w.availablePages[:numPages]       // page used to save data
	w.availablePages = w.availablePages[numPages:] // rest of the pages

	// Write data to dataPages
	buf := bytes.NewBuffer(data)
	pages := make([]page, numPages)
	for i := range pages {
		if uint64(i+1) < numPages {
			pages[i].nextPage = &pages[i+1]
		}
		pages[i].offset = dataPages[i]
		pages[i].payload = buf.Next(maxPayloadSize)
	}
	return &pages[0]
}

// allocateNewPages update the metadata used for new pages. Note the function is not safe to use
func (w *Wal) allocateNewPages(numNewPages uint64) {
	start := w.pageCount + 1
	for i := start; i < start+numNewPages; i++ {
		w.availablePages = append(w.availablePages, uint64(i)*PageSize)
	}
	w.pageCount += numNewPages
}

func composeError(errs ...error) error {
	var errMsg string
	for _, err := range errs {
		if err != nil {
			if len(errMsg) > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
	}
	if len(errMsg) == 0 {
		return nil
	}
	return errors.New(errMsg)
}

// writeRecoveryState is a helper function that changes the recoveryState on disk
func (w *Wal) writeRecoveryState(state uint8) error {
	_, err := w.logFile.WriteAt([]byte{byte(state)}, int64(len(metadataHeader)+len(metadataVersion)))
	if err != nil {
		return err
	}
	return w.logFile.Sync()
}
