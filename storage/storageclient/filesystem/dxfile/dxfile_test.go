package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"os"
	"path/filepath"
	"testing"
)

var testDir = tempDir()

func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "dxfile", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

func TestPruneSegment(t *testing.T) {
	tests := []struct {
		numSectors int
		usedNumSectorsPerIndex int
		unusedNumSectorsPerIndex int
	}{
		{10, 1, 1},
		{10, 3, 50},
		{40, 1, 20},
	}
	for _, test := range tests {
		df := &DxFile{
			metadata: &Metadata{ NumSectors: uint32(test.numSectors) },
			hostTable: make(hostTable),
		}
		seg := segment{
			sectors: make([][]*sector,test.numSectors),
		}
		usedSectors := make(map[enode.ID]bool)
		for i := range seg.sectors {
			for j := 0; j < test.usedNumSectorsPerIndex; j++ {
				sec := randomSector()
				seg.sectors[i] = append(seg.sectors[i], sec)
				df.hostTable[sec.hostID] = true
				usedSectors[sec.hostID] = false
			}
			for j := 0; j < test.unusedNumSectorsPerIndex; j++ {
				sec := randomSector()
				seg.sectors[i] = append(seg.sectors[i], sec)
				df.hostTable[sec.hostID] = false
			}
		}
		df.pruneSegment(&seg)
		// check whether all used sectors still there
		for _, sectors := range seg.sectors {
			for _, sector := range sectors {
				if _, exist := usedSectors[sector.hostID]; exist {
					usedSectors[sector.hostID] = true
				}
			}
		}
		for id, checked := range usedSectors {
			if !checked {
				t.Errorf("Key %x not found", id)
			}
		}
		b, err := rlp.EncodeToBytes(&seg)
		if err != nil {
			t.Error(err)
		}
		if allowed := segmentPersistNumPages(uint32(test.numSectors))* PageSize; uint64(len(b)) > allowed {
			t.Errorf("after purning, size large than allowed %d > %d", len(b), allowed)
		}
	}
}

