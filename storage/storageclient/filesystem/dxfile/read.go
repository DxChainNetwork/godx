// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

//
//import (
//	"crypto/rand"
//	"fmt"
//	"github.com/DxChainNetwork/godx/common"
//	"github.com/DxChainNetwork/godx/common/writeaheadlog"
//	"github.com/DxChainNetwork/godx/rlp"
//	"io"
//	"os"
//)
//
//func readDxFile(path string, wal *writeaheadlog.Wal) (*DxFile, error) {
//	var ID fileID
//	_, err := rand.Read(ID[:])
//	if err != nil {
//		return nil, fmt.Errorf("cannot create random ID: %v", err)
//	}
//	df := &DxFile{
//		ID:       ID,
//		filePath: path,
//		wal:      wal,
//	}
//	f, err := os.OpenFile(path, os.O_RDONLY, 0)
//	if err != nil {
//		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
//	}
//	df.metaData, err = readMetadata(f)
//	if err != nil {
//		return nil, err
//	}
//	err = df.loadHostAddresses(f)
//	if err != nil {
//		return nil, err
//	}
//	err = df.loadSegments(f)
//	if err != nil {
//		return nil, err
//	}
//	// New erasure code
//	df.erasureCode, err = df.metaData.newErasureCode()
//	if err != nil {
//		return nil, err
//	}
//	return df, nil
//}
//
//// readMetadata is the helper function to read metadata from reader
//func readMetadata(r io.Reader) (*Metadata, error) {
//	var md *Metadata
//	err := rlp.Decode(r, &md)
//	if err != nil {
//		return nil, fmt.Errorf("cannot decode metadata: %v", err)
//	}
//	return md, nil
//}
//
//// loadHostAddresses is the helper function to load DxFile.hostAddresses
//func (df *DxFile) loadHostAddresses(f *os.File) error {
//	if df.metaData == nil {
//		return fmt.Errorf("cannot load host addresses: metadata not ready")
//	}
//	offset := df.metaData.HostTableOffset
//	off, err := f.Seek(int64(offset), io.SeekStart)
//	if err != nil {
//		return fmt.Errorf("cannot load host addresses: %v", err)
//	}
//	if off%PageSize != 0 {
//		return fmt.Errorf("offset not devisible by page size")
//	}
//	pht, err := readPersistHostTable(f)
//	if err != nil {
//		return fmt.Errorf("cannot decide persist host addresses: %v", err)
//	}
//	df.hostAddresses = make(map[common.Address]*hostAddress)
//	for _, pha := range pht {
//		df.hostAddresses[pha.HostAddress.Address] = &pha.HostAddress
//	}
//	return nil
//}
//
//// readHostTable is the helper function to read host table from reader
//func readPersistHostTable(r io.Reader) (persistHostTable, error) {
//	var pht persistHostTable
//	err := rlp.Decode(r, &pht)
//	if err != nil {
//		return nil, fmt.Errorf("cannot decode hostTable: %v", err)
//	}
//	return pht, nil
//}
//
//func (df *DxFile) loadSegments(f *os.File) error {
//	if df.metaData == nil {
//		return fmt.Errorf("cannot load segments: metadata not ready")
//	}
//	offset := uint64(df.metaData.SegmentOffset)
//	segmentSize := segmentPersistNumPages(df.metaData.NumSectors)
//	for {
//		_, err := f.Seek(int64(offset), io.SeekStart)
//		if err == io.EOF {
//			break
//		}
//		if err != nil {
//			return fmt.Errorf("cannot load segments: %v", err)
//		}
//		ps, err := readPersistSegment(f)
//		if err == io.EOF {
//			break
//		}
//		if len(ps.Sectors) != int(df.metaData.NumSectors) {
//			return fmt.Errorf("segment does not have expected numSectors")
//		}
//		dataSegment := &segment{make([][]*sector, df.metaData.NumSectors), false}
//		for i, pSectors := range ps.Sectors {
//			for _, pSector := range pSectors {
//				dataSegment.sectors[i] = append(dataSegment.sectors[i], &pSector.Sector)
//			}
//		}
//		offset += segmentSize
//	}
//	return nil
//}
//
//// readSegment is a helper function to read a segment from reader
//func readPersistSegment(r io.Reader) (*persistSegment, error) {
//	var seg *persistSegment
//	err := rlp.Decode(r, seg)
//	if err != nil {
//		return nil, fmt.Errorf("cannot decode segment: %v", err)
//	}
//	return seg, nil
//}
