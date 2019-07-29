// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// TestNewFileSystemEmptyStart test the situation of creating a new file system in
// an empty directory
func TestNewFileSystemEmptyStartClose(t *testing.T) {
	fs := newEmptyTestFileSystem(t, "", nil, newStandardDisrupter())
	fs.disrupter.registerDisruptFunc("InitAndUpdateDirMetadata", makeBlockDisruptFunc(fs.tm.StopChan(),
		func() bool { return false }))
	if len(fs.unfinishedUpdates) != 0 {
		t.Errorf("empty start should have 0 unfinished updates. Got %v", len(fs.unfinishedUpdates))
	}
	c := make(chan error)
	go func(c chan error) {
		c <- fs.Close()
	}(c)
	select {
	case err := <-c:
		if err != nil {
			t.Errorf("close return an error")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("close not immediate shutdown")
	}
}

// TestFileSystem_SelectDxFileToFix test the functionality of selecting a dxfile with the worst health
func TestFileSystem_SelectDxFileToFix(t *testing.T) {
	tests := []struct {
		numFiles int
	}{
		{1},
		{10},
		{100},
	}
	if testing.Short() {
		tests = tests[:1]
	}
	for i, test := range tests {
		// Create a random file system with random files, and random contractManager
		dr := newStandardDisrupter()
		ct := &randomContractManager{
			missRate:         0.1,
			onlineRate:       0.8,
			goodForRenewRate: 0.8,
		}
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), ct, dr)
		ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
		if err != nil {
			t.Fatal(err)
		}
		fileSize := uint64(1 << 22 * 10 * 10)
		var minHealth = dxdir.DefaultHealth
		for i := 0; i != test.numFiles; i++ {
			path := randomDxPath(t, 5)
			file, err := fs.fileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
			if err != nil {
				t.Fatal(err)
			}
			table := fs.contractManager.HostHealthMapByID(file.HostIDs())
			if err = file.MarkAllUnhealthySegmentsAsStuck(table); err != nil {
				t.Fatal(err)
			}
			if err = file.MarkAllHealthySegmentsAsUnstuck(table); err != nil {
				t.Fatal(err)
			}
			if err := fs.InitAndUpdateDirMetadata(file.DxPath()); err != nil {
				t.Fatal(err)
			}
			fHealth, _, _ := file.Health(table)
			if dxfile.CmpRepairPriority(fHealth, minHealth) >= 0 {
				minHealth = fHealth
			}
		}
		if err := fs.waitForUpdatesComplete(10 * time.Second); err != nil {
			t.Fatal(err)
		}
		f, err := fs.SelectDxFileToFix()
		if err == ErrNoRepairNeeded && dxfile.CmpRepairPriority(minHealth, dxdir.DefaultHealth) <= 0 {
			// no repair needed
			continue
		} else if err == nil {
			if f.GetHealth() != minHealth {
				t.Errorf("SelectDxFileToFix got file not with expected health. Got %v, Expect %v", f.GetHealth(), minHealth)
			}
		} else if err == errStopped {
			// No logging for stopped
		} else {
			t.Fatalf("SelectDxFileToFix return unexpected error: %v", err)
		}
	}
}

// TestFileSystem_RandomStuckDirectory test the functionality of TestFileSystem.RandomStuckDirectory
func TestFileSystem_RandomStuckDirectory(t *testing.T) {
	tests := []struct {
		numFiles  int
		missRate  float32
		expectErr error
	}{
		{10, 1, nil},
		{10, 0, ErrNoRepairNeeded},
	}
	for i, test := range tests {
		dr := newStandardDisrupter()
		ct := &AlwaysSuccessContractManager{}
		fs := newEmptyTestFileSystem(t, "", ct, dr)
		if err := fs.createRandomFiles(test.numFiles, 0.8, 0.3, 5, test.missRate); err != nil {
			t.Fatal(err)
		}
		dir, err := fs.RandomStuckDirectory()
		if err != test.expectErr {
			t.Fatalf("Test %d: Expect error %v, Got %v", i, test.expectErr, err)
		}
		if err != nil {
			continue
		}
		if err = dir.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// randomDxPath create a random DxPath for testing with a certain depth
func randomDxPath(t *testing.T, depth int) storage.DxPath {
	var s string
	for i := 0; i != depth; i++ {
		var b [16]byte
		_, err := rand.Read(b[:])
		if err != nil {
			t.Fatal(err)
		}
		s = filepath.Join(s, common.Bytes2Hex(b[:]))
	}
	path, err := storage.NewDxPath(s)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
