// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"fmt"
	"os"

	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

// Snapshot is the snapshot of a DxFile which contains necessary info for a DxFile.
// The data of Snapshot is deep copy of the Original DxFile and will not affect the
// Original DxFile
type Snapshot struct {
	fileSize    uint64
	sectorSize  uint64
	erasureCode erasurecode.ErasureCoder
	cipherKey   crypto.CipherKey
	fileMode    os.FileMode
	segments    []Segment
	hostTable   map[enode.ID]bool
	dxPath      storage.DxPath
}

// SnapshotReader is the structure that allow reading the raw DxFile content
type SnapshotReader struct {
	f  *os.File
	df *DxFile
}

// SnapshotReader creates a reader that could be used to read DxFile content
func (df *DxFile) SnapshotReader() (*SnapshotReader, error) {
	df.lock.RLock()

	if df.deleted {
		df.lock.RUnlock()
		return nil, fmt.Errorf("file has been deleted")
	}
	f, err := os.Open(string(df.filePath))
	if err != nil {
		df.lock.RUnlock()
		return nil, err
	}
	return &SnapshotReader{
		df: df,
		f:  f,
	}, nil
}

// Read read the content from the DxFile data file
func (sr *SnapshotReader) Read(b []byte) (int, error) {
	return sr.f.Read(b)
}

// Stat return the file info of the DxFile data file
func (sr *SnapshotReader) Stat() (os.FileInfo, error) {
	return sr.f.Stat()
}

// Close close the DxFile data file, and also release the lock of the DxFile
func (sr *SnapshotReader) Close() error {
	sr.df.lock.RUnlock()
	return sr.f.Close()
}

// Snapshot creates the Snapshot of the DxFile
func (df *DxFile) Snapshot() (*Snapshot, error) {
	ck, err := df.CipherKey()
	if err != nil {
		return nil, err
	}
	ec, err := df.ErasureCode()
	if err != nil {
		return nil, err
	}

	df.lock.RLock()
	defer df.lock.RUnlock()

	hostTable := make(map[enode.ID]bool)
	for key, value := range df.hostTable {
		hostTable[key] = value
	}

	segments := make([]Segment, 0, len(df.segments))
	for _, segment := range df.segments {
		segments = append(segments, copySegment(segment))
	}

	return &Snapshot{
		fileSize:    df.metadata.FileSize,
		sectorSize:  df.metadata.SectorSize,
		erasureCode: ec,
		cipherKey:   ck,
		fileMode:    df.metadata.FileMode,
		segments:    segments,
		hostTable:   hostTable,
		dxPath:      df.metadata.DxPath,
	}, nil
}

// SegmentIndexByOffset return the segment index and offset with the give offset of a file
func (s *Snapshot) SegmentIndexByOffset(offset uint64) (uint64, uint64) {
	index := offset / s.SegmentSize()
	off := offset % s.SegmentSize()
	return index, off
}

// SegmentSize return the size of a single segment
func (s *Snapshot) SegmentSize() uint64 {
	return s.sectorSize * uint64(s.erasureCode.MinSectors())
}

// ErasureCode return the erasure code
func (s *Snapshot) ErasureCode() erasurecode.ErasureCoder {
	return s.erasureCode
}

// CipherKey return the cipher key
func (s *Snapshot) CipherKey() crypto.CipherKey {
	return s.cipherKey
}

// FileMode return the file mode
func (s *Snapshot) FileMode() os.FileMode {
	return s.fileMode
}

// NumSegments return the number of segments for DxFile
func (s *Snapshot) NumSegments() uint64 {
	return uint64(len(s.segments))
}

// Sectors return the sectors of the segment index
func (s *Snapshot) Sectors(segmentIndex uint64) ([][]*Sector, error) {
	return copySectors(&s.segments[segmentIndex]), nil
}

// SectorSize return the sectorSize
func (s *Snapshot) SectorSize() uint64 {
	return s.sectorSize
}

// DxPath return the DxPath
func (s *Snapshot) DxPath() storage.DxPath {
	return s.dxPath
}

// FileSize return the file size
func (s *Snapshot) FileSize() uint64 {
	return uint64(s.fileSize)
}

// copySegment deep copy a segment
func copySegment(seg *Segment) Segment {
	copySeg := Segment{
		Index:  seg.Index,
		Stuck:  seg.Stuck,
		offset: seg.offset,
	}
	copySeg.Sectors = copySectors(seg)
	return copySeg
}

// copySectors copy the sectors of the segment
func copySectors(seg *Segment) [][]*Sector {
	copiedSectors := make([][]*Sector, len(seg.Sectors))
	for i, sectors := range seg.Sectors {
		for _, sector := range sectors {
			copySector := *sector
			copiedSectors[i] = append(copiedSectors[i], &copySector)
		}
	}
	return copiedSectors
}
