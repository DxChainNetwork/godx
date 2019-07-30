// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"reflect"
	"testing"
	"time"
)

// TestSnapshotReader test the process of a SnapshotReader
func TestSnapshotReader(t *testing.T) {
	numSector := uint32(30)
	minSector := uint32(10)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*10, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := df.SnapshotReader()
	if err != nil {
		t.Fatal(err)
	}
	_, err = sr.Stat()
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, PageSize)
	_, err = sr.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(b)
	var metadata Metadata
	err = rlp.Decode(buf, &metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkMetadataEqual(df.metadata, &metadata); err != nil {
		t.Errorf("metadata not equal: %v", err)
	}
	wait := make(chan error)
	ErrTimeout := fmt.Errorf("time out")
	ErrSet := fmt.Errorf("set file mode complete")
	go func() {
		_ = df.SetFileMode(0777)
		wait <- ErrSet
	}()
	go func() {
		<-time.After(300 * time.Millisecond)
		wait <- ErrTimeout
	}()
	err = <-wait
	if err != ErrTimeout {
		t.Errorf("dxfile should be locked.")
	}
	err = sr.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = <-wait
	if err != ErrSet {
		t.Errorf("dxfile should have been completed: %v", err)
	}
}

// TestSnapshot test the creation of a Snapshot
func TestSnapshot(t *testing.T) {
	numSector := uint32(30)
	minSector := uint32(10)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*10, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := df.Snapshot()
	ec, err := df.ErasureCode()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.ErasureCode(), ec) {
		t.Errorf("erasure code not equal. Expect %+v, Got %+v", ec, s.ErasureCode())
	}
	cipherKey, err := df.CipherKey()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.CipherKey(), cipherKey) {
		t.Errorf("cipher key not equal. Expect %+v, Got %+v", cipherKey, s.CipherKey())
	}
	if s.FileMode() != df.metadata.FileMode {
		t.Errorf("file mode not equal. Expect %v, Got %+v", df.metadata.FileMode, s.FileMode())
	}
	if s.SegmentSize() != df.SegmentSize() {
		t.Errorf("segment size not equal. Expect %v, Got %v", df.SegmentSize(), s.SegmentSize())
	}
	if s.NumSegments() != uint64(len(df.segments)) {
		t.Errorf("NumSegments size not equal. Expect %v, Got %v", len(df.segments), s.NumSegments())
	}
	if s.SectorSize() != df.metadata.SectorSize {
		t.Errorf("SectorSize not equal. Expect %v, Got %v", df.metadata.SectorSize, s.SectorSize())
	}
	if s.DxPath() != df.metadata.DxPath {
		t.Errorf("DxPath not equal. Expect %v, Got %v", df.metadata.DxPath, s.DxPath())
	}
	if s.FileSize() != df.metadata.FileSize {
		t.Errorf("FileSize not equal. Expect %v, Got %v", df.metadata.FileSize, s.FileSize())
	}
	if len(s.segments) != len(df.segments) {
		t.Errorf("segment size not equal. Expect %v, Got %v", len(df.segments), len(s.segments))
	}
	for i := range df.segments {
		if err = checkSegmentEqualNotSame(&s.segments[i], df.segments[i]); err != nil {
			t.Errorf("segments[%d].%v", i, err)
		}
	}
}

// checkSegmentEqualNotSame checks whether two segments are same in value while different in pointers.
func checkSegmentEqualNotSame(got, expect *Segment) error {
	if got.Index != expect.Index {
		return fmt.Errorf("index not same. Expect %v, Got %v", expect.Index, got.Index)
	}
	if got.Stuck != expect.Stuck {
		return fmt.Errorf("stuck not same. Expect %v, Got %v", expect.Stuck, got.Stuck)
	}
	if got.offset != expect.offset {
		return fmt.Errorf("offset not same. Expect %v, Got %v", expect.offset, got.offset)
	}
	if len(got.Sectors) != len(expect.Sectors) {
		return fmt.Errorf("sectors size not same. Expect %v, Got %v", len(expect.Sectors), len(got.Sectors))
	}
	for i := range expect.Sectors {
		if len(expect.Sectors[i]) != len(got.Sectors[i]) {
			return fmt.Errorf("sectors[%d] size not same. Expect %d, Got %d", i, len(expect.Sectors[i]), len(got.Sectors[i]))
		}
		for j := range expect.Sectors[i] {
			if expect.Sectors[i][j] == got.Sectors[i][j] {
				return fmt.Errorf("sectors[%d][%d] point to the same address", i, j)
			}
			if !reflect.DeepEqual(expect.Sectors[i][j], got.Sectors[i][j]) {
				return fmt.Errorf("sectors[%d][%d] not exactly the same. Expect %v, Got %v", i, j, expect.Sectors[i][j], got.Sectors[i][j])
			}
		}
	}
	return nil
}
