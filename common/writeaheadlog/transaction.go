package writeaheadlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/crypto/sha3"
	"sync"
	"sync/atomic"
)

const (
	checksumSize = 16
	// For the txn's first page, the content is different.
	// status, Id, checksum, nextPageOffset
	txnMetaSize = 8 + 8 + checksumSize + 8

	MaxHeadPagePayloadSize = PageSize - txnMetaSize
)

const (
	txnStatusInvalid = iota
	txnStatusWritten
	txnStatusCommitted
	txnStatusApplied
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, PageSize)
	},
}

// Transaction is a batch of Operations to be committed as a whole
type Transaction struct {
	Id         uint64       // Each transaction is assigned a unique id
	operations []*Operation // Each transaction is composed of a list of operations
	headPage   *page        // The first page to write on
	wal        *Wal

	// State of the transaction. Default to false when created, and set to true when a certain step is finished.
	// The overall routine is setup -> commit -> release
	setupComplete   bool
	commitComplete  bool
	releaseComplete bool

	status       uint64 // txnStatusInvalid, txnStatusWritten, txnStatusCommitted, txnStatusApplied
	initComplete chan struct{}
	initErr      error
	mu           sync.Mutex
}

// NewTransaction create a new transaction. Start a thread to write the operations to disk.
func (w *Wal) NewTransaction(ops []*Operation) (*Transaction, error) {
	// Validate the input
	if len(ops) == 0 {
		return nil, errors.New("cannot create a transaction without operations")
	}
	for i, op := range ops {
		err := op.verify()
		if err != nil {
			return nil, fmt.Errorf("ops[%d] not valid: %v", i, err)
		}
	}

	// Create New Transaction
	txn := &Transaction{
		operations:   ops,
		wal:          w,
		initComplete: make(chan struct{}),
	}

	go txn.threadedInit()
	// Increment the numUnfinishedTxns
	atomic.AddInt64(&w.numUnfinishedTxns, 1)

	return txn, nil
}

// threadedInit write the metadata and operations to wal.LogFile
func (t *Transaction) threadedInit() {
	defer close(t.initComplete)

	data := marshalOps(t.operations)
	if len(data) > MaxHeadPagePayloadSize {
		t.headPage = t.wal.requestPages(data[:MaxHeadPagePayloadSize])
		t.headPage.nextPage = t.wal.requestPages(data[MaxHeadPagePayloadSize:])
	} else {
		t.headPage = t.wal.requestPages(data)
	}
	t.status = txnStatusWritten

	// write the header page
	if err := t.writeHeaderPage(false); err != nil {
		t.initErr = fmt.Errorf("writing the first page to disk failed: %v", err)
		return
	}
	// write subsequent pages
	for page := t.headPage.nextPage; page != nil; page = page.nextPage {
		if err := t.writePage(page); err != nil {
			t.initErr = fmt.Errorf("writing the page to disk failed: %v", err)
			return
		}
	}
}

// Append appends additional updates to a transaction
func (t *Transaction) Append(ops []*Operation) <-chan error {
	// Verify the updates
	for _, op := range ops {
		op.verify()
	}
	done := make(chan error, 1)

	if t.setupComplete || t.commitComplete || t.releaseComplete {
		done <- errors.New("misuse of transaction - can't append to transaction once it is committed/released")
		return done
	}

	go func() {
		done <- t.append(ops)
	}()
	return done
}

// append is a helper function to append updates to a transaction on which
// Commit hasn't been called yet
func (t *Transaction) append(ops []*Operation) (err error) {
	// If there is nothing to append we are done
	if len(ops) == 0 {
		return nil
	}

	// Make sure that the initialization finished
	<-t.initComplete
	if t.initErr != nil {
		return t.initErr
	}

	// Marshal the data
	data := marshalOps(ops)
	if err != nil {
		return err
	}

	// Find last page, to which we will append
	lastPage := t.headPage
	for lastPage.nextPage != nil {
		lastPage = lastPage.nextPage
	}

	// Preserve the original payload of the last page and the original ops
	// of the transaction if an error occurs
	defer func() {
		if err != nil {
			lastPage.payload = lastPage.payload[:len(lastPage.payload)]
			t.operations = t.operations[:len(t.operations)]
			lastPage.nextPage = nil

			// Write last page
			err2 := t.writePage(lastPage)
			err = composeError(fmt.Errorf("writing the last page to disk failed: %v", err), err2)
		}
	}()

	// Write as much data to the last page as possible
	var lenDiff int
	if lastPage == t.headPage {
		// firstPage holds less data than subsequent pages
		lenDiff = MaxHeadPagePayloadSize - len(lastPage.payload)
	} else {
		lenDiff = MaxPayloadSize - len(lastPage.payload)
	}

	if len(data) <= lenDiff {
		lastPage.payload = append(lastPage.payload, data...)
		data = nil
	} else {
		lastPage.payload = append(lastPage.payload, data[:lenDiff]...)
		data = data[lenDiff:]
	}

	// If there is no more data to write, we don't need to allocate any new
	// pages. Write the new last page to disk and append the new ops.
	if len(data) == 0 {
		if err := t.writePage(lastPage); err != nil {
			return fmt.Errorf("writing the last page to disk failed: %v", err)
		}
		t.operations = append(t.operations, ops...)
		return nil
	}

	// Get enough pages for the remaining data
	lastPage.nextPage = t.wal.requestPages(data)

	// Write the new pages, then write the tail page that links to them.
	// Writing in this order ensures that if writing the new pages fails, the
	// old tail page remains valid.
	for page := lastPage.nextPage; page != nil; page = page.nextPage {
		if err := t.writePage(page); err != nil {
			return fmt.Errorf("writing the page to disk failed: %v", err)
		}
	}

	// write last page
	if err := t.writePage(lastPage); err != nil {
		return fmt.Errorf("writing the last page to disk failed: %v", err)
	}

	// Append the ops to the transaction
	t.operations = append(t.operations, ops...)
	return nil
}

// Commit returns a error channel which will block until commit finished.
// The content is error nil if no error in commit, and error if there is an error
func (t *Transaction) Commit() <-chan error {
	done := make(chan error, 1)

	if t.setupComplete || t.commitComplete || t.releaseComplete {
		done <- errors.New("misuse of transaction - call each of the signaling methods exactly ones, in serial, in order")
		return done
	}
	t.setupComplete = true

	// Commit the transaction non-blocking
	go func() {
		done <- t.commit()
	}()
	return done
}

// commit commits a transaction by setting the correct status and checksum
func (t *Transaction) commit() error {
	// Make sure that the initialization of the transaction finished
	<-t.initComplete
	if t.initErr != nil {
		return t.initErr
	}

	// Set the transaction status
	t.status = txnStatusCommitted

	// Set the sequence number and increase the WAL's transactionCounter
	t.Id = atomic.AddUint64(&t.wal.nextTxnId, 1) - 1

	// Calculate the checksum
	checksum := t.checksum()

	if t.wal.utils.disrupt("CommitFail") {
		return errors.New("write failed on purpose")
	}

	// Marshal metadata into buffer
	buf := bufPool.Get().([]byte)
	binary.LittleEndian.PutUint64(buf[:], t.status)
	binary.LittleEndian.PutUint64(buf[8:], t.Id)
	copy(buf[16:], checksum)

	// Finalize the commit by writing the metadata to disk.
	_, err := t.wal.logFile.WriteAt(buf[:16+checksumSize], int64(t.headPage.offset))
	bufPool.Put(buf)
	if err != nil {
		return fmt.Errorf("writing the first page failed: %v", err)
	}

	if err := t.wal.fSync(); err != nil {
		return fmt.Errorf("writing the first page failed, %v", err)
	}

	t.commitComplete = true
	return nil
}

// Release informs the WAL that it is safe to reuse t's pages.
func (t *Transaction) Release() error {
	if !t.setupComplete || !t.commitComplete || t.releaseComplete {
		return errors.New("misuse of transaction - call each of the signaling methods exactly once, in serial, in order")
	}
	t.releaseComplete = true

	// Set the status to applied
	t.status = txnStatusApplied

	// Write the status to disk
	if t.wal.utils.disrupt("ReleaseFail") {
		return errors.New("write failed on purpose")
	}

	buf := bufPool.Get().([]byte)
	binary.LittleEndian.PutUint64(buf, t.status)
	_, err := t.wal.logFile.WriteAt(buf[:8], int64(t.headPage.offset))
	bufPool.Put(buf)
	if err != nil {
		return fmt.Errorf("couldn't write the page to file: %v", err)
	}
	if err := t.wal.fSync(); err != nil {
		return fmt.Errorf("couldn't write the page to file: %v", err)
	}

	// Update the wal's available pages
	t.wal.mu.Lock()
	for page := t.headPage; page != nil; page = page.nextPage {
		// Append the index of the freed page
		t.wal.availablePages = append(t.wal.availablePages, page.offset)
	}
	t.wal.mu.Unlock()

	// Decrease the number of active transactions
	if atomic.LoadInt64(&t.wal.numUnfinishedTxns) == 0 {
		panic("Sanity check failed. atomicUnfinishedTxns should never be negative")
	}
	atomic.AddInt64(&t.wal.numUnfinishedTxns, -1)
	return nil
}

// checksum calculate the BLAKE2b-256 hash of a Transaction
func (t *Transaction) checksum() []byte {
	h := sha3.NewLegacyKeccak256()
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	// write metadata and contents to the buffer
	binary.LittleEndian.PutUint64(buf[:], t.status)
	binary.LittleEndian.PutUint64(buf[8:], t.Id)
	_, _ = h.Write(buf[:16])
	// write pages
	for page := t.headPage; page != nil; page = page.nextPage {
		for i := range buf {
			buf[i] = 0
		}
		page.marshal(buf[:0])
		_, _ = h.Write(buf)
	}
	//fmt.Println(h.Sum(buf[:0]))
	//fmt.Println()
	var sum [checksumSize]byte
	copy(sum[:], h.Sum(buf[:0]))
	return sum[:]
}

// writeHeaderPage write the header page of a transaction to the logfile.
// The header page is different from the normal page since it has some additional meta data
// t.status | t.Id | t.checkSum | page.offset
// The input bool checksum indicates whether the checksum field to be filled
func (t *Transaction) writeHeaderPage(checksum bool) error {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	binary.LittleEndian.PutUint64(buf[:], t.status)
	binary.LittleEndian.PutUint64(buf[8:], t.Id)
	// According to input checksum, decide whether to create the hash.
	var hash []byte
	if checksum {
		hash = t.checksum()
	} else {
		hash = make([]byte, checksumSize, checksumSize)
	}
	copy(buf[16:], hash)
	binary.LittleEndian.PutUint64(buf[checksumSize+16:], t.headPage.nextOffset())
	copy(buf[checksumSize+24:], t.headPage.payload)
	_, err := t.wal.logFile.WriteAt(buf, int64(t.headPage.offset))
	return err
}

// writePage write the page to file. If the page is header page, just write offset+payload
func (t *Transaction) writePage(page *page) error {
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	offset := page.offset
	if page == t.headPage {
		const shift = txnMetaSize - PageMetaSize
		offset += shift
		buf = buf[:len(buf)-shift]
	}
	for i := range buf {
		buf[i] = 0
	}
	page.marshal(buf[:0])
	_, err := t.wal.logFile.WriteAt(buf, int64(offset))
	return err
}
