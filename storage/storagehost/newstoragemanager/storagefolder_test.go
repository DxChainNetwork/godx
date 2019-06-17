// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import "testing"

// TestFreeSectorIndex test the function of freeSectorIndex
func TestFreeSectorIndex(t *testing.T) {
	tests := []struct {
		numSectors uint64
	}{
		{1},
		{64},
		{64 * 16},
		{64*16 + 32},
	}
	for i, test := range tests {
		sf := &storageFolder{
			numSectors: test.numSectors,
			usage:      emptyUsage(numSectorsToSize(test.numSectors)),
		}
		for sectorIndex := 0; sectorIndex != int(test.numSectors); sectorIndex++ {
			selected, err := sf.freeSectorIndex()
			if err != nil {
				t.Fatalf("Test %d: in %d iteration, err: %v", i, sectorIndex, err)
			}
			if err = sf.setUsedSectorSlot(selected); err != nil {
				t.Fatalf("Test %d: in %d iteration, err: %v", i, sectorIndex, err)
			}
		}
		if _, err := sf.freeSectorIndex(); err == nil {
			t.Fatalf("All sectors set to used, still can pick a free sector")
		}
		// clear all usages
		for sectorIndex := 0; sectorIndex != int(test.numSectors); sectorIndex++ {
			if err := sf.setFreeSectorSlot(uint64(sectorIndex)); err != nil {
				t.Fatalf("Test %d: in %d iteration, err: %v", i, sectorIndex, err)
			}
		}
		for index, bv := range sf.usage {
			if bv != 0 {
				t.Errorf("After clearing all usage, still %d usage not 0: %v", index, bv)
			}
		}
	}
}
