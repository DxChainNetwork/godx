// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"errors"
	"io"
	"sync"
)

// where to write the downloaded data
type writeDestination interface {
	WriteAt(data []byte, offset int64) (int, error)
}

// downloadBuffer writes logical segment data to an in-memory buffer.
type downloadBuffer struct {
	buf        [][]byte
	sectorSize uint64
}

// NewDownloadBuffer create a new downloadBuffer
func newDownloadBuffer(length, sectorSize uint64) downloadBuffer {
	// Completion the length multiple of sector size(4MB)
	if length%sectorSize != 0 {
		length += sectorSize - length%sectorSize
	}

	ddb := downloadBuffer{
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
func (dw downloadBuffer) ReadFrom(r io.Reader) (int64, error) {
	var n int64
	for len(dw.buf) > 0 {
		read, err := io.ReadFull(r, dw.buf[0])

		if err == io.ErrUnexpectedEOF || err == io.EOF {
			n += int64(read)
			return n, nil
		}

		if err != nil {
			return n, err
		}

		dw.buf = dw.buf[1:]
		n += int64(read)
	}
	return n, nil
}

// writes the given data to downloadBuffer.
func (dw downloadBuffer) WriteAt(data []byte, offset int64) (int, error) {
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

// writes to an underlying data stream
type downloadWriter struct {
	closed bool
	mu     sync.Mutex

	// the amount of data written
	progress int64
	io.Writer

	// a list of write calls waiting for their turn, and corresponding locks.
	writeCalls   []int64
	writeSignals []*sync.Mutex
}

var (
	errClosedStream         = errors.New("unable to write because stream has been closed")
	errOffsetAlreadyWritten = errors.New("cannot write to that offset in stream, data already written")
)

// convert an io.Writer into a downloadWriter
func newDownloadWriter(w io.Writer) *downloadWriter {
	return &downloadWriter{Writer: w}
}

// iterator all the write calls, unlock those reached
func (ddw *downloadWriter) unblockNextWrites() {
	for i, offset := range ddw.writeCalls {
		if offset <= ddw.progress {
			ddw.writeSignals[i].Unlock()
			ddw.writeCalls = append(ddw.writeCalls[0:i], ddw.writeCalls[i+1:]...)
			ddw.writeSignals = append(ddw.writeSignals[0:i], ddw.writeSignals[i+1:]...)
		}
	}
}

// stop all the write calls
func (ddw *downloadWriter) Close() error {
	ddw.mu.Lock()
	if ddw.closed {
		ddw.mu.Unlock()
		return errClosedStream
	}
	ddw.closed = true
	for i := range ddw.writeSignals {
		ddw.writeSignals[i].Unlock()
	}
	ddw.mu.Unlock()
	return nil
}

// write data to stream at given offset
func (ddw *downloadWriter) WriteAt(data []byte, offset int64) (int, error) {
	write := func() (int, error) {
		if ddw.closed {
			return 0, errClosedStream
		}

		if offset < ddw.progress {
			ddw.mu.Unlock()
			return 0, errOffsetAlreadyWritten
		}

		n, err := ddw.Write(data)
		ddw.progress += int64(n)
		ddw.unblockNextWrites()
		return n, err
	}

	ddw.mu.Lock()

	// if progress reached offset, go on writing data
	if offset <= ddw.progress {
		n, err := write()
		ddw.mu.Unlock()
		return n, err
	}

	// if progress has not reached offset, should create a new write call with corresponding lock,
	// and block at now until another thread calls "unblockNextWrites" to unlock it.
	myMu := new(sync.Mutex)
	myMu.Lock()
	ddw.writeCalls = append(ddw.writeCalls, offset)
	ddw.writeSignals = append(ddw.writeSignals, myMu)
	ddw.mu.Unlock()
	myMu.Lock()
	ddw.mu.Lock()
	n, err := write()
	ddw.mu.Unlock()
	return n, err
}
