package dxfile

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"testing"
)

func TestSegmentPersistNumPages(t *testing.T) {
	tests := []struct {
		numSectors uint32
	}{
		{0},
		{1},
		{100},
		{100000},
	}
	for i, test := range tests {
		numPages:= segmentPersistNumPages(test.numSectors)
		seg := randomSegment(test.numSectors)
		segBytes, _ := rlp.EncodeToBytes(seg)
		if PageSize * int(numPages) < len(segBytes) {
			t.Errorf("Test %d: pages not enough to hold data: %d * %d < %d", i, PageSize, numPages, len(segBytes))
		}
	}
}

// randomSector is the helper function to create a random sector
func randomSector(addr common.Address, root common.Hash) PersistSector {
	rand.Read(addr[:])
	rand.Read(root[:])
	return PersistSector{
		HostAddress: addr,
		MerkleRoot:  root,
	}
}

func randomSegment(numSectors uint32) PersistSegment {
	var addr common.Address
	var root common.Hash
	sectors := make([][]PersistSector, numSectors)
	for i := 0; i != int(numSectors); i++ {
		sectors[i] = append(sectors[i], randomSector(addr, root))
	}
	return PersistSegment{
		Sectors: sectors,
		Stuck:   false,
	}
}
