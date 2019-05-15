package dxfile

import (
	"bytes"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

var sectorSize = SectorSize - uint64(crypto.Overhead(crypto.GCMCipherCode))

// TestPersist write a dxFile and then read from the file try to get exactly the same dxfile.
func TestPersist(t *testing.T) {
	tests := []uint8{
		erasurecode.ECTypeStandard,
		erasurecode.ECTypeShard,
	}
	for _, test := range tests {
		df, err := newTestDxFileWithSegments(t, SectorSize<<6, 10, 30, test)
		err = df.saveAll()
		if err != nil {
			t.Fatalf(err.Error())
		}
		filename := filepath.Join(testDir, t.Name())
		wal := df.wal
		newDF, err := readDxFile(filename, wal)
		if err != nil {
			t.Fatalf(err.Error())
		}
		if err = checkDxFileEqual(*df, *newDF); err != nil {
			t.Error(err.Error())
		}
	}
}

// TestDxFile_SaveHostTableUpdate test the shift functionality of persistence
func TestDxFile_SaveHostTableUpdate(t *testing.T) {
	//Two scenarios has to be tested:
	//1. Shift size smaller than segment persist sizes,
	//2. Shift size larger than segment persist size.
	tests := []struct {
		numSegments       uint64 // size of the file, determine how many segments
		numHostTablePages uint64 // size of added host key
		minSectors        uint32
		numSectors        uint32
	}{
		{
			numSegments:       1,
			numHostTablePages: 1,
			minSectors:        10,
			numSectors:        30,
		},
		{
			numSegments:       1,
			numHostTablePages: 10,
			minSectors:        10,
			numSectors:        30,
		},
		{
			numSegments:       3,
			numHostTablePages: 1,
			minSectors:        10,
			numSectors:        30,
		},
		{
			numSegments:       3,
			numHostTablePages: 10,
			minSectors:        10,
			numSectors:        30,
		},
		{
			numSegments:       3,
			numHostTablePages: 1,
			minSectors:        1000,
			numSectors:        3000,
		},
		{
			numSegments:       3,
			numHostTablePages: 10,
			minSectors:        1000,
			numSectors:        3000,
		},
	}
	for i, test := range tests {
		minSectors := uint32(10)
		numSectors := uint32(30)
		fileSize := sectorSize * uint64(minSectors) * test.numSegments
		df, err := newTestDxFileWithSegments(t, fileSize, minSectors, numSectors, erasurecode.ECTypeStandard)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		hostTableSize := PageSize * test.numHostTablePages
		for i := 0; uint64(i) != hostTableSize/35; i++ {
			df.hostTable[randomAddress()] = false
		}
		err = df.saveHostTableUpdate()
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		filename := filepath.Join(testDir, t.Name())
		wal := df.wal
		newDF, err := readDxFile(filename, wal)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if err = checkDxFileEqual(*df, *newDF); err != nil {
			t.Errorf("test %d: %v", i, err)
		}
	}
}

// TestDxFile_SaveSegment test the functionality of saveSegment
func TestDxFile_SaveSegment(t *testing.T) {
	rand.Seed(time.Now().Unix())
	minSectors := uint32(10)
	numSectors := uint32(30)
	fileSize := sectorSize * uint64(minSectors) * 10
	df, err := newTestDxFileWithSegments(t, fileSize, minSectors, numSectors, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatalf(err.Error())
	}
	// Edit one of the segment
	modifyIndex := rand.Intn(len(df.segments))
	df.segments[modifyIndex].sectors[0][0].merkleRoot = common.Hash{}
	err = df.saveSegments([]int{modifyIndex})
	if err != nil {
		t.Fatal(err.Error())
	}
	filename := filepath.Join(testDir, t.Name())
	newDF, err := readDxFile(filename, df.wal)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err = checkDxFileEqual(*df, *newDF); err != nil {
		t.Errorf("%v", err)
	}
}

// newTestDxFile generate a random DxFile used for testing.
func newTestDxFile(t *testing.T, fileSize uint64, minSectors, numSectors uint32, ecCode uint8) (*DxFile, error) {
	ec, _ := erasurecode.New(ecCode, minSectors, numSectors, 64)
	ck, _ := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	filename := filepath.Join(testDir, t.Name())
	wal, txns, _ := writeaheadlog.New(filepath.Join(testDir, t.Name()+".wal"))
	for _, txn := range txns {
		txn.Release()
	}
	df, err := New(filename, t.Name(), filepath.Join("~/tmp", t.Name()), wal, ec, ck, fileSize, 0777)
	if err != nil {
		return nil, err
	}
	return df, nil
}

func newTestDxFileWithSegments(t *testing.T, fileSize uint64, minSectors, numSectors uint32, ecCode uint8) (*DxFile, error) {
	df, err := newTestDxFile(t, fileSize, minSectors, numSectors, ecCode)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; uint64(i) != df.metadata.numSegments(); i++ {
		seg := randomSegment(df.metadata.NumSectors)
		seg.index = uint64(i)
		df.segments[i] = seg
		for _, sectors := range seg.sectors {
			for _, sector := range sectors {
				df.hostTable[sector.hostID] = true
			}
		}
	}

	if err = df.saveAll(); err != nil {
		return nil, err
	}
	return df, nil
}

// Two DxFile are exactly the same if the all fields other than file id are the same
// (of course not including wal, lock, id, e.t.c
func checkDxFileEqual(df1, df2 DxFile) error {
	if err := checkMetadataEqual(df1.metadata, df2.metadata); err != nil {
		return err
	}
	if len(df1.segments) != len(df2.segments) {
		return fmt.Errorf("length of segments not equal: %d / %d", len(df1.segments), len(df2.segments))
	}
	for i := range df1.segments {
		if err := checkSegmentEqual(*df1.segments[i], *df2.segments[i]); err != nil {
			return fmt.Errorf("segment[%d]: %v", i, err)
		}
	}
	if !reflect.DeepEqual(df1.hostTable, df2.hostTable) {
		return fmt.Errorf("hostTable not equal:\n\t%+v\n\t%+v", df1.hostTable, df2.hostTable)
	}
	if !reflect.DeepEqual(df1.erasureCode, df2.erasureCode) {
		return fmt.Errorf("erasureCode not equal:\n\t%+v\n\t%+v", df1.erasureCode, df2.erasureCode)
	}
	if !reflect.DeepEqual(df1.cipherKey, df2.cipherKey) {
		return fmt.Errorf("cipherKey not equal:\n\t%+v\n\t%+v", df1.cipherKey, df2.cipherKey)
	}
	if !reflect.DeepEqual(df1.filePath, df2.filePath) {
		return fmt.Errorf("filePath not equal:\n\t%+v\n\t%+v", df1.filePath, df2.filePath)
	}
	if !reflect.DeepEqual(df1.deleted, df2.deleted) {
		return fmt.Errorf("deleted not equal:\n\t%+v\n\t%+v", df1.deleted, df2.deleted)
	}
	return nil
}

// checkMetadataEqual is the hard code function to check equality between two metadata
func checkMetadataEqual(md1, md2 *Metadata) error {
	if md1.HostTableOffset != md2.HostTableOffset {
		return fmt.Errorf("md.HostTableOffset not equal:\n\t%+v\n\t%+v", md1.HostTableOffset, md2.HostTableOffset)
	}
	if md1.SegmentOffset != md2.SegmentOffset {
		return fmt.Errorf("md.SegmentOffset not equal:\n\t%+v\n\t%+v", md1.SegmentOffset, md2.SegmentOffset)
	}
	if md1.FileSize != md2.FileSize {
		return fmt.Errorf("md.FileSize not equal:\n\t%+v\n\t%+v", md1.FileSize, md2.FileSize)
	}
	if md1.SectorSize != md2.SectorSize {
		return fmt.Errorf("md.SectorSize not equal:\n\t%+v\n\t%+v", md1.SectorSize, md2.SectorSize)
	}
	if md1.LocalPath != md2.LocalPath {
		return fmt.Errorf("md.LocalPath not equal:\n\t%+v\n\t%+v", md1.LocalPath, md2.LocalPath)
	}
	if md1.DxPath != md2.DxPath {
		return fmt.Errorf("md.DxPath not equal:\n\t%+v\n\t%+v", md1.DxPath, md2.DxPath)
	}
	if md1.CipherKeyCode != md2.CipherKeyCode {
		return fmt.Errorf("md.CipherKeyCode not equal:\n\t%+v\n\t%+v", md1.CipherKeyCode, md2.CipherKeyCode)
	}
	if !bytes.Equal(md1.CipherKey, md2.CipherKey) {
		return fmt.Errorf("md.CipherKey not equal:\n\t%+v\n\t%+v", md1.CipherKey, md2.CipherKey)
	}
	if md1.TimeModify != md2.TimeModify {
		return fmt.Errorf("md.TimeModify not equal:\n\t%+v\n\t%+v", md1.TimeModify, md2.TimeModify)
	}
	if md1.TimeAccess != md2.TimeAccess {
		return fmt.Errorf("md.TimeAccess not equal:\n\t%+v\n\t%+v", md1.TimeAccess, md2.TimeAccess)
	}
	if md1.TimeCreate != md2.TimeCreate {
		return fmt.Errorf("md.TimeCreate not equal:\n\t%+v\n\t%+v", md1.TimeCreate, md2.TimeCreate)
	}
	if md1.Health != md2.Health {
		return fmt.Errorf("md.Health not equal:\n\t%+v\n\t%+v", md1.Health, md2.Health)
	}
	if md1.StuckHealth != md2.StuckHealth {
		return fmt.Errorf("md.StuckHealth not equal:\n\t%+v\n\t%+v", md1.StuckHealth, md2.StuckHealth)
	}
	if md1.TimeLastHealthCheck != md2.TimeLastHealthCheck {
		return fmt.Errorf("md.TimeLastHealthCheck not equal:\n\t%+v\n\t%+v", md1.TimeLastHealthCheck, md2.TimeLastHealthCheck)
	}
	if md1.NumStuckSegments != md2.NumStuckSegments {
		return fmt.Errorf("md.NumStuckSegments not equal:\n\t%+v\n\t%+v", md1.NumStuckSegments, md2.NumStuckSegments)
	}
	if md1.TimeRecentRepair != md2.TimeRecentRepair {
		return fmt.Errorf("md.TimeRecentRepair not equal:\n\t%+v\n\t%+v", md1.TimeRecentRepair, md2.TimeRecentRepair)
	}
	if md1.LastRedundancy != md2.LastRedundancy {
		return fmt.Errorf("md.LastRedundancy not equal:\n\t%+v\n\t%+v", md1.LastRedundancy, md2.LastRedundancy)
	}
	if md1.FileMode != md2.FileMode {
		return fmt.Errorf("md.FileMode not equal:\n\t%+v\n\t%+v", md1.FileMode, md2.FileMode)
	}
	if md1.ErasureCodeType != md2.ErasureCodeType {
		return fmt.Errorf("md.ErasureCodeType not equal:\n\t%+v\n\t%+v", md1.ErasureCodeType, md2.ErasureCodeType)
	}
	if md1.MinSectors != md2.MinSectors {
		return fmt.Errorf("md.MinSectors not equal:\n\t%+v\n\t%+v", md1.MinSectors, md2.MinSectors)
	}
	if md1.NumSectors != md2.NumSectors {
		return fmt.Errorf("md.NumSectors not equal:\n\t%+v\n\t%+v", md1.NumSectors, md2.NumSectors)
	}
	if (len(md1.ECExtra) != 0 || len(md2.ECExtra) != 0) && !bytes.Equal(md1.ECExtra, md2.ECExtra) {
		return fmt.Errorf("md.ECExtra not equal:\n\t%+v\n\t%+v", md1.ECExtra, md2.ECExtra)
	}
	if md1.Version != md2.Version {
		return fmt.Errorf("md.Version not equal:\n\t%+v\n\t%+v", md1.Version, md2.Version)
	}
	return nil
}

func checkSegmentEqual(seg1, seg2 segment) error {
	if len(seg1.sectors) != len(seg2.sectors) {
		return fmt.Errorf("length of sectors not equal: %d != %d", len(seg1.sectors), len(seg2.sectors))
	}
	for i := range seg1.sectors {
		if ( seg1.sectors[i] == nil || len(seg1.sectors[i]) == 0 ) != ( seg2.sectors[i] == nil || len(seg2.sectors[i]) == 0 ) {
			return fmt.Errorf("%d: not equal", i)
		}
		if seg1.sectors[i] == nil {
			continue
		}
		if len(seg1.sectors[i]) != len(seg2.sectors[i]){
			return fmt.Errorf("%d: not equal", i)
		}
		for j := range seg1.sectors[i] {
			if (seg1.sectors[i][j] == nil) != (seg2.sectors[i][j] == nil) {
				return fmt.Errorf("%d/%d: not equal", i, j)
			}
			if seg1.sectors[i] == nil {
				continue
			}
			if !reflect.DeepEqual(seg1.sectors[i][j], seg2.sectors[i][j]) {
				return fmt.Errorf("%d/%d: not equal", i, j)
			}
		}
	}
	return nil
}
