package dxfile

import (
	"bytes"
	"fmt"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
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
		seg := segment{
			sectors: make([][]*sector, test.numSectors),
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
		df.segments = append(df.segments, &seg)
		df.pruneSegment(0)
		// check whether all used sectors still there
		if test.fitIn {
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

func TestAddSector(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	df, err := newTestDxFile(t, SectorSize*64, 10, 30, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	newAddr := randomAddress()
	segmentIndex := rand.Intn(int(df.metadata.numSegments()))
	sectorIndex := rand.Intn(int(df.metadata.NumSectors))
	newHash := randomHash()
	err = df.AddSector(newAddr, uint64(segmentIndex), uint64(sectorIndex), newHash)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(testDir, t.Name())
	wal := df.wal
	recoveredDF, err := readDxFile(filename, wal)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkDxFileEqual(*df, *recoveredDF); err != nil {
		t.Error(err)
	}
	sectors := recoveredDF.segments[segmentIndex].sectors[sectorIndex]
	recoveredNewSector := sectors[len(sectors)-1]
	if !bytes.Equal(recoveredNewSector.merkleRoot[:], newHash[:]) {
		t.Errorf("new sector merkle root not expected. Expect %v, got %v", newHash, recoveredNewSector.merkleRoot)
	}
	if !bytes.Equal(recoveredNewSector.hostID[:], newAddr[:]) {
		t.Errorf("new sector host address not expected. Expect %v, got %v", newAddr, recoveredNewSector.hostID)
	}
}

func TestDelete(t *testing.T) {
	df, err := newTestDxFile(t, sectorSize*64, 10, 30, erasurecode.ECTypeStandard)
	if err != nil {
		t.Fatal(err)
	}
	if err = df.Delete(); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(testDir, t.Name())
	if _, err := os.Stat(filename); err == nil || !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if !df.Deleted() {
		t.Errorf("After deletion, file not deleted in stat")
	}
}
