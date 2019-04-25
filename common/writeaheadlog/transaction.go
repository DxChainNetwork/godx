package writeaheadlog

import (
	"encoding/binary"
	"golang.org/x/crypto/blake2b"
	"math"
	"sync"
)

const checksumSize = 16

const (
	txnStatusInvalid = iota
	txnStatusWritten
	txnStatusCommitted
	txnStatusApplied
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, pageSize)
	},
}

// Transaction is a batch of Operations to be committed as a whole
type Transaction struct {
	txnId      uint64       // Each transaction is assigned a unique id
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
}

// Operation is a single operation defined by a name and data
type Operation struct {
	Name string
	Data []byte
}

// checksum calculate the BLAKE2b-256 hash of a Transaction
func (t *Transaction) checksum() []byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic("Cannot initialize BLAKE2b-256")
	}
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	// write metadata and contents to the buffer
	binary.LittleEndian.PutUint64(buf[:], t.status)
	binary.LittleEndian.PutUint64(buf[8:], t.txnId)
	_, _ = h.Write(buf[:16])
	// write pages
	for page := t.headPage; page != nil; page = page.nextPage {
		payload := page.marshal(buf[:0])
		_, _  = h.Write(payload)
	}
	// write result to checksum
	var checksum [checksumSize]byte
	copy(checksum[:], h.Sum(buf[:0]))
	return checksum[:]
}



// verify check the name field of the operation.
//  * name should not be empty
//  * name should be longer than 255 bytes
func (ope *Operation) verify() {
	if len(ope.Name) == 0 {
		panic("Name cannot be empty")
	}
	if len(ope.Name) > math.MaxUint8 {
		panic("Name longer than 255 bytes not supported")
	}
}
