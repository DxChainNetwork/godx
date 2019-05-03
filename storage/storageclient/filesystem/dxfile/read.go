// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	// PageSize is the page size of persist data
	PageSize = 4096

	// sectorPersistSize is the size of rlp encoded string of a sector
	sectorPersistSize = 56

	// Overhead for PersistSegment persist data. The value is larger than data actually used
	segmentPersistOverhead = 16

	// Duplication rate is the expected duplication of the size of the sectors
	redundancyRate float64 = 1.0
)

// TODO: how to create updates to create dxfile

// readDxFile read and create a DxFile from disk specified with path.
func readDxFile(path string, wal *writeaheadlog.Wal) (*DxFile, error) {
	var ID fileID
	_, err := rand.Read(ID[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create a random ID: %v", err)
	}
	df := &DxFile{
		ID:       ID,
		filename: path,
		wal:      wal,
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
	}
	header, segmentOffset, err := readPersistHeader(f)
	if err != nil {
		return nil, err
	}
	df.metaData = header.Metadata
	for _, hostAddr := range header.HostAddresses {
		df.hostAddresses[hostAddr.Address] = hostAddr.Used
	}
	df.erasureCode, err = header.newErasureCode()
	if err != nil {
		return nil, fmt.Errorf("cannot create erasure code: %v", err)
	}
	// read segments
	off, err := f.Seek(int64(segmentOffset), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot read first segment: %v", err)
	}
	if off%PageSize != 0 {
		return nil, fmt.Errorf("segment offset not allowed: %d", off)
	}
	segmentSize := segmentPersistNumPages(header.NumSectors)
	if err != nil {
		return nil, err
	}
	for {
		seg, err := readSegment(f, segmentSize)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read segment: %v", err)
		}
		df.dataSegments = append(df.dataSegments, seg)
	}
	return df, nil
}

// readPersistHeader is the helper function that read the header using rlp encoding,
// return the decoded header, segment offset, and error
func readPersistHeader(r io.Reader) (*PersistHeader, int32, error) {
	headerLength, segmentOffset, err := readOverhead(r)
	if err != nil {
		return nil, 0, err
	}
	if headerLength > segmentOffset {
		return nil, 0, fmt.Errorf("failed to read header: segmentOffset larger than header length")
	}
	headerBytes, err := readExactBytes(r, int64(headerLength))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %v", err)
	}
	var ph *PersistHeader
	if err = rlp.DecodeBytes(headerBytes, ph); err != nil {
		return nil, 0, fmt.Errorf("failed to decode header: %v", err)
	}
	return ph, segmentOffset, nil
}

// readOverhead read the first two uint64 as headerLength and segmentOffset
func readOverhead(r io.Reader) (int32, int32, error) {
	headerLength, err := readInt32(r)
	if err != nil {
		return 0, 0, fmt.Errorf("read headerLength: %v", err)
	}
	if headerLength < 0 {
		return 0, 0, fmt.Errorf("headerSize is negative: %v", headerLength)
	}
	segmentOffset, err := readInt32(r)
	if err != nil {
		return 0, 0, fmt.Errorf("read segmentOffset: %v", err)
	}
	if segmentOffset < 0 {
		return 0, 0, fmt.Errorf("segmentOffset is negative: %v", headerLength)
	}
	return headerLength, segmentOffset, nil
}

// readSegment is a helper function to read a segment from reader
func readSegment(r io.Reader, length int64) (*PersistSegment, error) {
	segmentBytes, err := readExactBytes(r, length)
	if err != nil {
		return nil, fmt.Errorf("failed to read segment: %v", err)
	}
	var seg *PersistSegment
	if err = rlp.DecodeBytes(segmentBytes, seg); err != nil {
		return nil, err
	}
	return seg, nil
}

// readExactBytes is the helper function to read exactly length of data from r.
// If read data length is not the same as length, return an error.
func readExactBytes(r io.Reader, length int64) ([]byte, error) {
	dataBytes := make([]byte, length)
	n, err := r.Read(dataBytes)
	if n == 0 || err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	if int64(n) != length {
		return nil, fmt.Errorf("not enough data from reader: %d < %d", n, length)
	}
	return dataBytes, nil
}

// readInt32 is a helper function which read an int32 from reader
func readInt32(r io.Reader) (int32, error) {
	var num int32
	err := binary.Read(r, binary.LittleEndian, &num)
	if err != nil {
		return 0, fmt.Errorf("cannot read int32: %v", err)
	}
	return num, nil
}
