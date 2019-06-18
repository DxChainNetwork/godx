package newstoragemanager

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

// TestDeleteSectorBatchNormal test the scenario of deleting an physical sector and deleting
// a virtual sector.
func TestDeleteSectorBatchNormal(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisrupter())
	// Create one folder
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.AddStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// create the virtual sector and add twice
	dataVirtual := randomBytes(storage.SectorSize)
	rootVirtual := merkle.Root(dataVirtual)
	if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
		t.Fatal(err)
	}
	if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
		t.Fatal(err)
	}
	// create the physical sector
	dataPhysical := randomBytes(storage.SectorSize)
	rootPhysical := merkle.Root(dataPhysical)
	if err := sm.AddSector(rootPhysical, dataPhysical); err != nil {
		t.Fatal(err)
	}
	// Delete the batch root Virtual and rootPhysical each for once
	if err := sm.DeleteSectorBatch([]common.Hash{rootVirtual, rootPhysical}); err != nil {
		t.Fatal(err)
	}
	// Check the results
	if err := checkSectorExist(rootVirtual, sm, dataVirtual, 1); err != nil {
		t.Fatal(err)
	}
	if err := checkSectorNotExist(sm.calculateSectorID(rootPhysical), sm); err != nil {
		t.Fatal(err)
	}
	if err := checkFoldersHasExpectedSectors(sm, 1); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, 10*time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

// TestDeleteSectorBatchStop test
func TestDeleteSectorBatchStop(t *testing.T) {
	tests := []struct {
		keyWord string
		numTxn  int
	}{
		{"delete batch prepare stop", 0},
		{"delete batch process stop", 1},
	}
	for _, test := range tests {
		d := newDisrupter().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, "", d)
		// Create one folder
		path := randomFolderPath(t, "")
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatal(err)
		}
		// create the virtual sector and add twice
		dataVirtual := randomBytes(storage.SectorSize)
		rootVirtual := merkle.Root(dataVirtual)
		if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
			t.Fatal(err)
		}
		if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
			t.Fatal(err)
		}
		// create the physical sector
		dataPhysical := randomBytes(storage.SectorSize)
		rootPhysical := merkle.Root(dataPhysical)
		if err := sm.AddSector(rootPhysical, dataPhysical); err != nil {
			t.Fatal(err)
		}
		// Delete the batch root Virtual and rootPhysical each for once
		if err := sm.DeleteSectorBatch([]common.Hash{rootVirtual, rootPhysical}); err != nil {
			t.Fatal(err)
		}
		sm.shutdown(t, time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), test.numTxn); err != nil {
			t.Fatal(err)
		}
		newsm, err := New(sm.persistDir)
		if err != nil {
			t.Fatalf("cannot create a new sm: %v", err)
		}
		newSM := newsm.(*storageManager)
		if err := newSM.Start(); err != nil {
			t.Fatal(err)
		}
		// Wait for the updates to complete
		<-time.After(300 * time.Millisecond)
		// Check the results
		if err := checkSectorExist(rootVirtual, newSM, dataVirtual, 2); err != nil {
			t.Fatal(err)
		}
		if err := checkSectorExist(rootPhysical, newSM, dataPhysical, 1); err != nil {
			t.Fatal(err)
		}
		if err := checkFoldersHasExpectedSectors(newSM, 2); err != nil {
			t.Fatal(err)
		}
		newSM.shutdown(t, time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeleteSectorDisrupt(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"delete batch prepare normal"},
		{"delete batch process normal"},
	}
	for _, test := range tests {
		d := newDisrupter().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, "", d)
		// Create one folder
		path := randomFolderPath(t, "")
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatal(err)
		}
		// create the virtual sector and add twice
		dataVirtual := randomBytes(storage.SectorSize)
		rootVirtual := merkle.Root(dataVirtual)
		if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
			t.Fatal(err)
		}
		if err := sm.AddSector(rootVirtual, dataVirtual); err != nil {
			t.Fatal(err)
		}
		// create the physical sector
		dataPhysical := randomBytes(storage.SectorSize)
		rootPhysical := merkle.Root(dataPhysical)
		if err := sm.AddSector(rootPhysical, dataPhysical); err != nil {
			t.Fatal(err)
		}
		// Delete the batch root Virtual and rootPhysical each for once
		if err := sm.DeleteSectorBatch([]common.Hash{rootVirtual, rootPhysical}); err == nil {
			t.Fatal(err)
		}
		// Check the results
		if err := checkSectorExist(rootPhysical, sm, dataPhysical, 1); err != nil {
			t.Fatal(err)
		}
		if err := checkSectorExist(rootVirtual, sm, dataVirtual, 2); err != nil {
			t.Fatal(err)
		}
		if err := checkFoldersHasExpectedSectors(sm, 2); err != nil {
			t.Fatal(err)
		}
		sm.shutdown(t, 10*time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}
