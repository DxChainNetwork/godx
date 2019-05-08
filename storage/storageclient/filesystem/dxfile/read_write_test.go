package dxfile

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"path/filepath"
	"reflect"
	"testing"
)

// TestPersist write a dxFile and then read from the file try to get exactly the same dxfile.
func TestPersist(t *testing.T) {
	df, err := newTestDxFile(t, SectorSize << 6, 10, 30)
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

// TestDxFile_SaveHostTableUpdate test the shift functionality of persistence
func TestDxFile_SaveHostTableUpdate(t *testing.T) {
	//Two scenarios has to be tested:
	//1. Shift size smaller than segment persist sizes,
	//2. Shift size larger than segment persist size.
	sectorSize := SectorSize - uint64(crypto.Overhead(crypto.GCMCipherCode))
	tests := []struct {
		numSegments uint64 // size of the file, determine how many segments
		numHostTablePages uint64 // size of added host key
	}{
		{
			numSegments: 1,
			numHostTablePages: 1,
		},
		{
			numSegments: 1,
			numHostTablePages: 10,
		},
		{
			numSegments: 3,
			numHostTablePages: 1,
		},
		{
			numSegments: 3,
			numHostTablePages: 10,
		},
	}
	for i, test := range tests {
		fmt.Println("=============================")
		minSectors := uint32(10)
		numSectors := uint32(30)
		fileSize := sectorSize * uint64(minSectors) * test.numSegments
		df, err := newTestDxFile(t, fileSize, minSectors, numSectors)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		hostTableSize := PageSize * test.numHostTablePages
		for i := 0; uint64(i) != hostTableSize / 23; i++ {
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

// newTestDxFile generate a random DxFile used for testing.
// The DxFile use erasure code params minSector = 10; numSector=30 with shardECType
// All segments are generated randomly.
func newTestDxFile(t *testing.T, fileSize uint64, minSectors, numSectors uint32) (*DxFile, error){
	ec, _ := erasurecode.New(erasurecode.ECTypeShard, minSectors, numSectors, 64)
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
	for i :=0; uint64(i) != df.metadata.numSegments(); i++{
		seg := randomSegment(df.metadata.NumSectors)
		seg.index = uint64(i)
		df.segments[i] = seg
		for _, sectors := range seg.sectors {
			for _, sector := range sectors {
				df.hostTable[sector.hostAddress] = true
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
	if err := checkMetadataEqual(df1.metadata, df2.metadata); err != nil{
		return err
	}
	if !reflect.DeepEqual(df1.segments, df2.segments) {
		return fmt.Errorf("segments not equal:\n\t%+v\n\t%+v", df1.segments, df2.segments)
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
	if md1.PagesPerSegment != md2.PagesPerSegment {
		return fmt.Errorf("md.PagesPerSegment not equal:\n\t%+v\n\t%+v", md1.PagesPerSegment, md2.PagesPerSegment)
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
	if md1.TimeUpdate != md2.TimeUpdate {
		return fmt.Errorf("md.TimeUpdate not equal:\n\t%+v\n\t%+v", md1.TimeUpdate, md2.TimeUpdate)
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
	if md1.LastHealthCheck != md2.LastHealthCheck {
		return fmt.Errorf("md.LastHealthCheck not equal:\n\t%+v\n\t%+v", md1.LastHealthCheck, md2.LastHealthCheck)
	}
	if md1.NumStuckChunks != md2.NumStuckChunks {
		return fmt.Errorf("md.NumStuckChunks not equal:\n\t%+v\n\t%+v", md1.NumStuckChunks, md2.NumStuckChunks)
	}
	if md1.RecentRepairTime != md2.RecentRepairTime {
		return fmt.Errorf("md.RecentRepairTime not equal:\n\t%+v\n\t%+v", md1.RecentRepairTime, md2.RecentRepairTime)
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
