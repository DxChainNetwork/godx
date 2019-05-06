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

func readDxFile(path string, wal *writeaheadlog.Wal) (*DxFile, error) {
	var ID fileID
	_, err := rand.Read(ID[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create random ID: %v", err)
	}
	df := &DxFile{
		ID:       ID,
		filePath: path,
		wal:      wal,
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
	}
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

	return nil
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
	err = rlp.Decode(f, df.hostTable)
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
	for {
		_, err := f.Seek(int64(offset), io.SeekStart)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var seg *segment
		err = rlp.Decode(f, &seg)
		if err == io.EOF {
			break
		}
		if len(seg.sectors) != int(df.metadata.NumSectors) {
			return fmt.Errorf("segment does not have expected numSectors")
		}
		seg.offset = offset
		df.segments = append(df.segments, seg)
		offset += segmentSize
	}
	return nil
}
