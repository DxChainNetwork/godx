// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

// TestCmpHealth test CmpRepairPriority
func TestCmpHealth(t *testing.T) {
	tests := []struct {
		h1  uint32
		h2  uint32
		res int
	}{
		{200, 175, 0},
		{200, 100, -1},
		{100, 175, 1},
		{100, 99, 1},
		{99, 100, -1},
		{174, 100, -1},
		{100, 180, 1},
		{0, 99, 0},
		{99, 0, 0},
	}
	for _, test := range tests {
		res := CmpRepairPriority(test.h1, test.h2)
		if res != test.res {
			t.Errorf("compare health unexpected value: %d, %d -> %d", test.h1, test.h2, res)
		}
	}
}

// TestSegmentHealth test DxFile.SegmentHealth
func TestSegmentHealth(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	tests := []struct {
		minSectors     uint32
		numSectors     uint32
		numMiss        int
		numOffline     int
		numBadForRenew int
		expectMin      uint32
		expectMax      uint32
		repeat         int
	}{
		{1, 2, 1, 0, 0, 100, 100, 1},
		{10, 30, 3, 7, 7, 115, 150, 10},
		{10, 30, 5, 10, 10, 50, 125, 10},
		{10, 30, 0, 0, 0, 200, 200, 10},
		{10, 30, 30, 0, 0, 0, 0, 10},
		{10, 30, 15, 0, 0, 125, 125, 10},
		{10, 30, 5, 20, 20, 0, 50, 10},
	}
	for index, test := range tests {
		for i := 0; i != test.repeat; i++ {
			seg := randomSegment(test.numSectors)
			df := DxFile{
				metadata: &Metadata{
					NumSectors: test.numSectors,
					MinSectors: test.minSectors,
				},
				segments: []*Segment{seg},
			}
			table := make(storage.HostHealthInfoTable)
			for _, sectors := range seg.Sectors {
				for _, sector := range sectors {
					table[sector.HostID] = storage.HostHealthInfo{
						Offline:      false,
						GoodForRenew: true,
					}
				}
			}
			for i := 0; i != test.numMiss; i++ {
				for k := range table {
					delete(table, k)
					break
				}
			}
			for i := 0; i != test.numOffline; i++ {
				for k := range table {
					if table[k].Offline {
						continue
					}
					table[k] = storage.HostHealthInfo{
						Offline:      true,
						GoodForRenew: table[k].GoodForRenew,
					}
					break
				}
			}
			for i := 0; i != test.numBadForRenew; i++ {
				for k := range table {
					if !table[k].GoodForRenew {
						continue
					}
					table[k] = storage.HostHealthInfo{
						Offline:      table[k].Offline,
						GoodForRenew: false,
					}
					break
				}
			}
			health := df.SegmentHealth(0, table)
			if health < test.expectMin || health > test.expectMax {
				t.Errorf("test %d: health %d not between %d ~ %d", index, health, test.expectMin, test.expectMax)
			}
		}
	}
}

// TestHealth test DxFile.Health
func TestHealth(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	numSectors := uint32(30)
	minSectors := uint32(10)
	for i := 0; i != 10; i++ {
		df, table := newTestDxFileWithMaps(t, sectorSize*20*uint64(minSectors), minSectors, numSectors, erasurecode.ECTypeStandard, 2, 10, 3, 3)
		var numExpectStuck int
		health, stuckHealth, numStuckSegments := df.Health(table)
		var minStuckSegmentFound, minUnstuckSegmentFound bool
		var haveStuckSegment, haveUnstuckSegment bool
		for i, seg := range df.segments {
			segHealth := df.SegmentHealth(i, table)
			if seg.Stuck {
				if !haveStuckSegment {
					haveStuckSegment = true
				}
				numExpectStuck++
				if segHealth < stuckHealth {
					t.Errorf("Stuck Segment health larger than dxfile Stuck health: %d > %d", segHealth, stuckHealth)
				}
				if segHealth == stuckHealth {
					minStuckSegmentFound = true
				}
			} else {
				if !haveUnstuckSegment {
					haveUnstuckSegment = true
				}
				if segHealth < health {
					t.Errorf("unstuck Segment health larger than dxfile health: %d > %d", segHealth, health)
				}
				if segHealth == health {
					minUnstuckSegmentFound = true
				}
			}
		}
		if haveStuckSegment && !minStuckSegmentFound {
			t.Errorf("min Stuck Segment not found %v", stuckHealth)
		}
		if haveUnstuckSegment && !minUnstuckSegmentFound {
			t.Errorf("min unstuck Segment not found %v", health)
		}
		if numExpectStuck != int(numStuckSegments) {
			t.Errorf("numExpectStuck not equal. Expect %d Got %d", numExpectStuck, numStuckSegments)
		}
	}
}

// newTestDxFileWithMaps create a new DxFile along with offlineMao and goodForRenewMap for test purpose.
// The offlineMap, goodForRenewMap, or stuck is random selected by stuckRate, absentRate, offlineRate, and badForRenewMap
func newTestDxFileWithMaps(t *testing.T, fileSize uint64, minSectors, numSectors uint32, ecCode uint8, stuckRate, absentRate, offlineRate, badForRenewRate int) (*DxFile, storage.HostHealthInfoTable) {
	rand.Seed(time.Now().UnixNano())
	df, err := newTestDxFile(t, fileSize, minSectors, numSectors, ecCode)
	if err != nil {
		t.Fatal(err)
	}
	numSegments := df.metadata.numSegments()
	table := make(storage.HostHealthInfoTable)
	for j := 0; j != int(numSegments); j++ {
		seg := randomSegment(numSectors)
		seg.Index = uint64(j)
		if stuckRate != 0 && rand.Intn(stuckRate) == 0 {
			seg.Stuck = true
		} else {
			seg.Stuck = false
		}
		df.segments[j] = seg
		for _, sectors := range seg.Sectors {
			for _, sector := range sectors {
				df.hostTable[sector.HostID] = true
				if absentRate != 0 && rand.Intn(absentRate) == 0 {
					continue
				}
				var info storage.HostHealthInfo
				if offlineRate != 0 && rand.Intn(offlineRate) == 0 {
					info.Offline = true
				}
				if badForRenewRate == 0 || rand.Intn(badForRenewRate) != 0 {
					info.GoodForRenew = true
				}
				table[sector.HostID] = info
			}
		}
	}
	err = df.saveAll()
	if err != nil {
		t.Fatal(err)
	}
	return df, table
}
