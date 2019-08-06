// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

// TestPruneSegment test df.pruneSegment
func TestPruneSegment(t *testing.T) {
	tests := []struct {
		numSectors               int
		usedNumSectorsPerIndex   int
		unusedNumSectorsPerIndex int
		fitIn                    bool
	}{
		{10, 1, 1, true},
		{10, 3, 500, true},
		{40, 1, 20, true},
		{40, 4, 100, false},
	}
	for _, test := range tests {
		df := &DxFile{
			metadata:  &Metadata{NumSectors: uint32(test.numSectors)},
			hostTable: make(hostTable),
		}
		seg := Segment{
			Sectors: make([][]*Sector, test.numSectors),
		}
		usedSectors := make(map[enode.ID]bool)
		for i := range seg.Sectors {
			for j := 0; j < test.usedNumSectorsPerIndex; j++ {
				sec := randomSector()
				seg.Sectors[i] = append(seg.Sectors[i], sec)
				df.hostTable[sec.HostID] = true
				usedSectors[sec.HostID] = false
			}
			for j := 0; j < test.unusedNumSectorsPerIndex; j++ {
				sec := randomSector()
				seg.Sectors[i] = append(seg.Sectors[i], sec)
				df.hostTable[sec.HostID] = false
			}
		}
		df.segments = append(df.segments, &seg)
		df.pruneSegment(0)
		// check whether all used Sectors still there
		if test.fitIn {
			for _, sectors := range seg.Sectors {
				for _, sector := range sectors {
					if _, exist := usedSectors[sector.HostID]; exist {
						usedSectors[sector.HostID] = true
					}
				}
			}
			for id, checked := range usedSectors {
				if !checked {
					t.Errorf("Key %x not found", id)
				}
			}
		}
		b, err := rlp.EncodeToBytes(&seg)
		if err != nil {
			t.Error(err)
		}
		if allowed := segmentPersistNumPages(uint32(test.numSectors)) * PageSize; uint64(len(b)) > allowed {
			t.Errorf("after purning, size large than allowed %d > %d", len(b), allowed)
		}
	}
}

// TestAddSector test DxFile.AddSector
func TestAddSector(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	df, err := newTestDxFileWithSegments(t, SectorSize*64, 10, 30, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	newAddr := randomAddress()
	segmentIndex := rand.Intn(int(df.metadata.numSegments()))
	sectorIndex := rand.Intn(int(df.metadata.NumSectors))
	newHash := randomHash()
	err = df.AddSector(newAddr, newHash, segmentIndex, sectorIndex)
	if err != nil {
		t.Fatal(err)
	}
	path, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	filename := testDir.Join(path)
	wal := df.wal
	recoveredDF, err := readDxFile(filename, wal)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkDxFileEqual(df, recoveredDF); err != nil {
		t.Error(err)
	}
	sectors := recoveredDF.segments[segmentIndex].Sectors[sectorIndex]
	recoveredNewSector := sectors[len(sectors)-1]
	if !bytes.Equal(recoveredNewSector.MerkleRoot[:], newHash[:]) {
		t.Errorf("new Sector merkle root not expected. Expect %v, got %v", newHash, recoveredNewSector.MerkleRoot)
	}
	if !bytes.Equal(recoveredNewSector.HostID[:], newAddr[:]) {
		t.Errorf("new Sector host address not expected. Expect %v, got %v", newAddr, recoveredNewSector.HostID)
	}
}

// TestDelete test DxFile.Delete function
func TestDelete(t *testing.T) {
	df, err := newTestDxFile(t, sectorSize*64, 10, 30, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	if err = df.Delete(); err != nil {
		t.Fatal(err)
	}
	path, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	filename := testDir.Join(path)
	if _, err := os.Stat(string(filename)); err == nil || !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if !df.Deleted() {
		t.Errorf("After deletion, file not deleted in os system")
	}
}

// TestMarkAllUnhealthySegmentsAsStuck test df.MarkAllUnhealthySegmentsAsStuck
func TestMarkAllUnhealthySegmentsAsStuck(t *testing.T) {
	for i := 0; i != 10; i++ {
		df, table := newTestDxFileWithMaps(t, sectorSize*10*20, 10, 30, erasurecode.ECTypeStandard,
			2, 1000, 3, 3)
		err := df.MarkAllUnhealthySegmentsAsStuck(table)
		if err != nil {
			t.Fatal(err)
		}
		for i, seg := range df.segments {
			segHealth := df.segmentHealth(i, table)
			if segHealth < 100 && !seg.Stuck {
				t.Errorf("Segment with health %d should have been marked as Stuck", segHealth)
			}
		}
		path, err := storage.NewDxPath(t.Name())
		if err != nil {
			t.Fatal(err)
		}
		filename := testDir.Join(path)
		wal := df.wal
		recoveredDF, err := readDxFile(filename, wal)
		if err != nil {
			t.Fatal(err)
		}
		if err = checkDxFileEqual(df, recoveredDF); err != nil {
			t.Error(err)
		}
	}
}

// TestMarkAllHealthySegmentsAsUnstuck test MarkAllHealthySegmentsAsUnstuck
func TestMarkAllHealthySegmentsAsUnstuck(t *testing.T) {
	for i := 0; i != 10; i++ {
		df, table := newTestDxFileWithMaps(t, sectorSize*10*20, 10, 30, erasurecode.ECTypeStandard,
			1, 100, 100, 100)
		err := df.MarkAllHealthySegmentsAsUnstuck(table)
		if err != nil {
			t.Fatal(err)
		}
		for i, seg := range df.segments {
			segHealth := df.segmentHealth(i, table)
			if segHealth == 200 && seg.Stuck {
				t.Errorf("Segment with health %d should have been marked as non-Stuck", segHealth)
			}
		}
		path, err := storage.NewDxPath(t.Name())
		if err != nil {
			t.Fatal(err)
		}
		filename := testDir.Join(path)
		wal := df.wal
		recoveredDF, err := readDxFile(filename, wal)
		if err != nil {
			t.Fatal(err)
		}
		if err = checkDxFileEqual(df, recoveredDF); err != nil {
			t.Error(err)
		}
	}
}

// TestRedundancy test DxFile.Redundancy
func TestRedundancy(t *testing.T) {
	tests := []struct {
		numSegments         uint64
		minSector           uint32
		numSector           uint32
		ecCodeType          uint8
		stuckRate           int
		absentRate          int
		offlineRate         int
		badForRenewRate     int
		expectMinRedundancy uint32
		expectMaxRedundancy uint32
	}{
		{1, 10, 30, erasurecode.ECTypeStandard, 2, 0, 1, 1, 0, 0},
		{1, 10, 30, erasurecode.ECTypeStandard, 2, 1, 0, 0, 0, 0},
		{1, 10, 30, erasurecode.ECTypeStandard, 2, 0, 1, 0, 0, 0},
		{1, 10, 30, erasurecode.ECTypeStandard, 2, 0, 0, 1, 100, 100},
		{1, 10, 30, erasurecode.ECTypeStandard, 2, 0, 0, 0, 300, 300},
		{10, 10, 30, erasurecode.ECTypeStandard, 2, 10, 4, 4, 0, 300},
	}
	for i, test := range tests {
		fileSize := sectorSize * uint64(test.minSector) * test.numSegments
		df, table := newTestDxFileWithMaps(t, fileSize, test.minSector, test.numSector,
			test.ecCodeType, test.stuckRate, test.absentRate, test.offlineRate, test.badForRenewRate)
		red := df.Redundancy(table)
		t.Logf("test %d give redundancy %d\n", i, red)
		if red < test.expectMinRedundancy || red > test.expectMaxRedundancy {
			t.Errorf("test %d: redundancy give value %d not between %d ~ %d", i, red, test.expectMinRedundancy, test.expectMaxRedundancy)
		}
	}
}

// TestSetStuckByIndex test DxFile.SetStuckByIndex
func TestSetStuckByIndex(t *testing.T) {
	tests := []struct {
		prevNumStuckSegment  uint32
		prevStuck            bool
		setStuck             bool
		afterNumStuckSegment uint32
	}{
		{0, false, false, 0},
		{0, false, true, 1},
		{1, true, false, 0},
		{1, true, true, 1},
	}
	for i, test := range tests {
		df, err := newTestDxFile(t, sectorSize*10*1, 10, 30, erasurecode.ECTypeStandard)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		df.metadata.NumStuckSegments = test.prevNumStuckSegment
		df.segments[0] = randomSegment(30)
		df.segments[0].Stuck = test.prevStuck
		err = df.saveAll()
		if err != nil {
			t.Fatal(err)
		}
		err = df.SetStuckByIndex(0, test.setStuck)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if df.metadata.NumStuckSegments != test.afterNumStuckSegment {
			t.Errorf("test %d: NumStuckSegments not expected. Expect %v Got %c", i, test.afterNumStuckSegment,
				df.metadata.NumStuckSegments)
		}
		if df.GetStuckByIndex(0) != test.setStuck {
			t.Errorf("test %d: Stuck not expected. Expect %v, Got %v", i, test.setStuck, df.segments[0].Stuck)
		}
		path, err := storage.NewDxPath(t.Name())
		if err != nil {
			t.Fatal(err)
		}
		filename := testDir.Join(path)
		recoveredDF, err := readDxFile(filename, df.wal)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if err = checkDxFileEqual(df, recoveredDF); err != nil {
			t.Errorf("test %d: %v", i, err)
		}
	}
}

// TestUploadProgress test the DxFile.UploadProgress
func TestUploadProgress(t *testing.T) {
	fileSegments := uint64(10)
	minSector := uint32(10)
	numSector := uint32(30)
	df, err := newTestDxFile(t, sectorSize*uint64(minSector)*fileSegments, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	prevProgress := float64(0)
	progress := float64(0)
	for i := range df.segments {
		for j := range df.segments[i].Sectors {
			addr := randomAddress()
			err := df.AddSector(addr, randomHash(), i, j)
			if err != nil {
				t.Fatalf("Sector %d %d: %v", i, j, err)
			}
			progress = df.UploadProgress()
			if progress <= prevProgress {
				t.Errorf("progress not incrementing %v -> %v", prevProgress, progress)
			}
			prevProgress = progress
		}
	}
	if progress != float64(100) {
		t.Errorf("after uploading, progress %v not 100", progress)
	}
}

// TestUpdateUsedHosts test DxFile.UpdateUsedHosts
func TestUpdateUsedHosts(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	fileSegments := uint64(10)
	minSector := uint32(10)
	numSector := uint32(30)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*fileSegments, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	hostLength := len(df.hostTable)
	var used []enode.ID
	usedMap := make(map[enode.ID]struct{})
	for key := range df.hostTable {
		if rand.Intn(4) == 0 {
			used = append(used, key)
			usedMap[key] = struct{}{}
		}
	}
	for i := 0; i != hostLength; i++ {
		used = append(used, randomAddress())
	}
	err = df.UpdateUsedHosts(used)
	if err != nil {
		t.Fatal(err)
	}
	for host, used := range df.hostTable {
		if _, exist := usedMap[host]; exist {
			if !used {
				t.Errorf("host %v should be used", host)
			}
		} else {
			if used {
				t.Errorf("host %v should not be used", host)
			}
		}
	}
}

// TestHostIDs test DxFile.HostIDs
func TestHostIDs(t *testing.T) {
	fileSegments := uint64(10)
	minSector := uint32(10)
	numSector := uint32(30)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*fileSegments, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	hostIds := df.HostIDs()
	for _, host := range hostIds {
		if _, exist := df.hostTable[host]; !exist {
			t.Errorf("host id %x not exist in df.hostTable", host)
		}
	}
}

// TestRename test DxFile.Rename
func TestRename(t *testing.T) {
	fileSegments := uint64(10)
	minSector := uint32(10)
	numSector := uint32(30)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*fileSegments, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	newDxFile, err := storage.NewDxPath(t.Name() + "2")
	if err != nil {
		t.Fatal(err)
	}
	newDxFilePath := testDir.Join(newDxFile)

	err = df.Rename(newDxFile, newDxFilePath)
	if err != nil {
		t.Fatal(err)
	}
	oldDxFile, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	oldDxFilePath := testDir.Join(oldDxFile)
	if _, err := os.Stat(string(oldDxFilePath)); err == nil {
		t.Errorf("file %v should have been deleted", oldDxFilePath)
	}

	recoveredDF, err := readDxFile(newDxFilePath, df.wal)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkDxFileEqual(recoveredDF, df); err != nil {
		t.Error(err)
	}
}

// TestApplyCachedHealthMetadata test DxFile.ApplyCachedHealthMetadata
func TestApplyCachedHealthMetadata(t *testing.T) {
	chm := CachedHealthMetadata{
		Health:      100,
		Redundancy:  100,
		StuckHealth: 100,
	}
	fileSegments := uint64(10)
	minSector := uint32(10)
	numSector := uint32(30)
	df, err := newTestDxFileWithSegments(t, sectorSize*uint64(minSector)*fileSegments, minSector, numSector, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	err = df.ApplyCachedHealthMetadata(chm)
	if err != nil {
		t.Fatal(err)
	}
	if df.metadata.Health != chm.Health {
		t.Errorf("health not updated: Expect %v, Got %v", chm.Health, df.metadata.Health)
	}
	if df.metadata.LastRedundancy != chm.Health {
		t.Errorf("redundancy not updated: expect %v, got %v", chm.Redundancy, df.metadata.LastRedundancy)
	}
	if df.metadata.StuckHealth != chm.StuckHealth {
		t.Errorf("stuckHealth not updated: expect %v, got %v", chm.StuckHealth, df.metadata.StuckHealth)
	}

	path, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	filename := testDir.Join(path)

	recoveredDF, err := readDxFile(filename, df.wal)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkDxFileEqual(recoveredDF, df); err != nil {
		t.Error(err)
	}
}
