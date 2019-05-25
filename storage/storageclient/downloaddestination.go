// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"errors"
	"io"
	"sync"
)

// downloadDestination is a wrapper for the different types of writing that we
// can do when recovering and writing the logical data of a file.
type downloadDestination interface {
	WriteAt(data []byte, offset int64) (int, error)
}

// downloadDestinationBuffer writes logical segment data to an in-memory buffer.
// This buffer is primarily used when performing repairs on uploads.
type downloadDestinationBuffer struct {
	buf        [][]byte
	sectorSize uint64
}

// allocates the necessary number of shards for the downloadDestinationBuffer and returns the new buffer
func NewDownloadDestinationBuffer(length, sectorSize uint64) downloadDestinationBuffer {
	// Round length up to next multiple of SectorSize.
	if length%sectorSize != 0 {
		length += sectorSize - length%sectorSize
	}
	// Create buffer
	ddb := downloadDestinationBuffer{
		buf:        make([][]byte, 0, length/sectorSize),
		sectorSize: sectorSize,
	}
	for length > 0 {
		ddb.buf = append(ddb.buf, make([]byte, sectorSize))
		length -= sectorSize
	}
	return ddb
}

// reads data from a io.Reader until the buffer is full.
func (dw downloadDestinationBuffer) ReadFrom(r io.Reader) (int64, error) {
	var n int64
	for len(dw.buf) > 0 {
		read, err := io.ReadFull(r, dw.buf[0])
		if err != nil {
			return n, err
		}
		dw.buf = dw.buf[1:]
		n += int64(read)
	}
	return n, nil
}

// writes the provided data to the downloadDestinationBuffer.
func (dw downloadDestinationBuffer) WriteAt(data []byte, offset int64) (int, error) {
	if uint64(len(data))+uint64(offset) > uint64(len(dw.buf))*dw.sectorSize || offset < 0 {
		return 0, errors.New("write at specified offset exceeds buffer size")
	}
	written := len(data)
	for len(data) > 0 {
		shardIndex := offset / int64(dw.sectorSize)
		sliceIndex := offset % int64(dw.sectorSize)
		n := copy(dw.buf[shardIndex][sliceIndex:], data)
		data = data[n:]
		offset += int64(n)
	}
	return written, nil
}

// downloadDestinationWriter is a downloadDestination that writes to an underlying data stream
type downloadDestinationWriter struct {
	closed    bool
	mu        sync.Mutex // Protects the underlying data structures.
	progress  int64      // How much data has been written yet.
	io.Writer            // The underlying writer.

	// A list of write calls and their corresponding locks. When one write call
	// completes, it'll search through the list of write calls for the next one.
	// The next write call can be unblocked by unlocking the corresponding mutex
	// in the next array.
	blockingWriteCalls   []int64 // A list of write calls that are waiting for their turn
	blockingWriteSignals []*sync.Mutex
}

var (
	// errClosedStream gets returned if the stream was closed but we are trying
	// to write.
	errClosedStream = errors.New("unable to write because stream has been closed")

	// errOffsetAlreadyWritten gets returned if a call to WriteAt tries to write
	// to a place in the stream which has already had data written to it.
	errOffsetAlreadyWritten = errors.New("cannot write to that offset in stream, data already written")
)

// newDownloadDestinationWriter takes an io.Writer and converts it
// into a downloadDestination.
func newDownloadDestinationWriter(w io.Writer) *downloadDestinationWriter {
	return &downloadDestinationWriter{Writer: w}
}

// unblockNextWrites will iterate over all of the blocking write calls and
// unblock any whose offsets have been reached by the current progress of the
// stream.
func (ddw *downloadDestinationWriter) unblockNextWrites() {
	for i, offset := range ddw.blockingWriteCalls {
		if offset <= ddw.progress {
			ddw.blockingWriteSignals[i].Unlock()
			ddw.blockingWriteCalls = append(ddw.blockingWriteCalls[0:i], ddw.blockingWriteCalls[i+1:]...)
			ddw.blockingWriteSignals = append(ddw.blockingWriteSignals[0:i], ddw.blockingWriteSignals[i+1:]...)
		}
	}
}

// Close will unblock any hanging calls to WriteAt, and then call Close on the
// underlying WriteCloser.
func (ddw *downloadDestinationWriter) Close() error {
	ddw.mu.Lock()
	if ddw.closed {
		ddw.mu.Unlock()
		return errClosedStream
	}
	ddw.closed = true
	for i := range ddw.blockingWriteSignals {
		ddw.blockingWriteSignals[i].Unlock()
	}
	ddw.mu.Unlock()
	return nil
}

// WriteAt will block until the stream has progressed to 'offset', and then it will write its own data.
func (ddw *downloadDestinationWriter) WriteAt(data []byte, offset int64) (int, error) {
	write := func() (int, error) {

		// Error if the stream has been closed.
		if ddw.closed {
			return 0, errClosedStream
		}

		// Error if the stream has progressed beyond 'offset'.
		if offset < ddw.progress {
			ddw.mu.Unlock()
			return 0, errOffsetAlreadyWritten
		}

		// write the data to the stream, update the progress and unblock the next write.
		n, err := ddw.Write(data)
		ddw.progress += int64(n)
		ddw.unblockNextWrites()
		return n, err
	}

	ddw.mu.Lock()

	// attempt to write if the stream progress is at or beyond the offset
	if offset <= ddw.progress {
		n, err := write()
		ddw.mu.Unlock()
		return n, err
	}

	// The stream has not yet progressed to 'offset'. We will block until the
	// stream has made progress. We perform the block by creating a
	// thread-specific mutex 'myMu' and adding it to the object's list of
	// blocking threads. When other threads successfully call WriteAt, they will
	// reference this list and unblock any which have enough progress. The
	// result is a somewhat strange construction where we lock myMu twice in a
	// row, but between those two calls to lock, we put myMu in a place where
	// another thread can unlock myMu.
	//
	// myMu will be unblocked when another thread calls 'unblockNextWrites'.
	myMu := new(sync.Mutex)
	myMu.Lock()
	ddw.blockingWriteCalls = append(ddw.blockingWriteCalls, offset)
	ddw.blockingWriteSignals = append(ddw.blockingWriteSignals, myMu)
	ddw.mu.Unlock()
	myMu.Lock()
	ddw.mu.Lock()
	n, err := write()
	ddw.mu.Unlock()
	return n, err
}
