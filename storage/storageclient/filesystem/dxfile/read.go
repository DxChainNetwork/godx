// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
	"os"
)

// readDxFile create a new DxFile with a random ID, then open and read the dxfile from filepath
// and load all params from the file.
func readDxFile(filepath string, wal *writeaheadlog.Wal) (*DxFile, error) {
	var ID fileID
	_, err := rand.Read(ID[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create random ID: %v", err)
	}
	df := &DxFile{
		ID:       ID,
		filePath: filepath,
		wal:      wal,
	}
	f, err := os.OpenFile(filepath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", filepath, err)
	}
	defer f.Close()
	err = df.loadMetadata(f)
	if err != nil {
		return nil, fmt.Errorf("cannot load metadata: %v", err)
	}
	err = df.loadHostAddresses(f)
	if err != nil {
		return nil, fmt.Errorf("cannot load host addresses: %v", err)
	}
	err = df.loadSegments(f)
	if err != nil {
		return nil, fmt.Errorf("cannot load segments: %v", err)
	}
	// New erasure code
	df.erasureCode, err = df.metadata.newErasureCode()
	if err != nil {
		return nil, fmt.Errorf("cannot new erasureCode: %v", err)
	}
	// New cipher key
	df.cipherKey, err = df.metadata.newCipherKey()
	if err != nil {
		return nil, fmt.Errorf("cannot new cipherKey: %v", err)
	}
	return df, nil
}

// readMetadata load metadata from the file
func (df *DxFile) loadMetadata(f io.Reader) error {
	err := rlp.Decode(f, &df.metadata)
	if err != nil {
		return err
	}
	// sanity check
	return df.metadata.validate()
}

// loadHostAddresses load DxFile.hostTable from the file f
func (df *DxFile) loadHostAddresses(f io.ReadSeeker) error {
	if df.metadata == nil {
		return fmt.Errorf("metadata not ready")
	}
	offset := df.metadata.HostTableOffset
	off, err := f.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return err
	}
	if off%PageSize != 0 {
		return fmt.Errorf("offset not divisible by page size")
	}
	df.hostTable = make(hostTable)
	err = rlp.Decode(f, &df.hostTable)
	if err != nil {
		return err
	}
	return nil
}

// loadSegments loads all segments to df.segments from the file f
func (df *DxFile) loadSegments(f io.ReadSeeker) error {
	if df.metadata == nil {
		return fmt.Errorf("metadata not ready")
	}
	offset := uint64(df.metadata.SegmentOffset)
	segmentSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)
	df.segments = make([]*segment, df.metadata.numSegments())
	for i := 0 ; uint64(i) < df.metadata.numSegments(); i++{
		seg, err := df.readSegment(f, offset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to load segment at %d: %v", offset, err)
		}
		seg.offset = offset
		if df.segments[seg.index] != nil {
			return fmt.Errorf("duplicate segment %d at %d", seg.index, seg.offset)
		}
		df.segments[seg.index] = seg
		offset += segmentSize
	}
	return nil
}

func (df *DxFile) readSegment(f io.ReadSeeker, offset uint64) (*segment, error) {
	if int64(offset) < 0 {
		return nil, fmt.Errorf("int64 overflow")
	}
	_, err := f.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}
	var seg *segment
	err = rlp.Decode(f, &seg)
	if err != nil {
		return nil, err
	}
	if len(seg.sectors) != int(df.metadata.NumSectors) {
		return nil, fmt.Errorf("segment does not have expected numSectors")
	}
	return seg, nil
}
