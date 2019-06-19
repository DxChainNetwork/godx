// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

// TestAddBatchConcurrent test adding batch normally as concurrent
func TestAddBatchConcurrent(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisrupter())
	// Create one folder
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.AddStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// insert three sectors
	type expect struct {
		root  common.Hash
		count int
		data  []byte
	}
	sectors := size / storage.SectorSize
	expects := make([]expect, 0, sectors)
	var lock sync.Mutex
	for i := 0; i != int(sectors); i++ {
		// Add 8 sectors
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatal(err)
		}
		expects = append(expects, expect{
			data:  data,
			root:  root,
			count: 1,
		})
	}
	// Randomly select 3 as a batch each time
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	for numBatch := 0; numBatch != 10; numBatch++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch := make([]common.Hash, 0, 3)
			selectedIndex := rand.Perm(int(sectors))[:3]
			lock.Lock()
			for _, i := range selectedIndex {
				batch = append(batch, expects[i].root)
			}
			lock.Unlock()
			if err := sm.AddSectorBatch(batch); err != nil {
				errChan <- err
			}
			lock.Lock()
			for _, i := range selectedIndex {
				prev := expects[i]
				prev.count++
				expects[i] = prev
			}
			lock.Unlock()
		}()
	}
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()
	select {
	case <-waitChan:
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatalf("time out")
	}
	for _, expect := range expects {
		if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFoldersHasExpectedSectors(sm, int(sectors)); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

// TestAddBatchError test AddBatch with error input
func TestAddBatchError(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisrupter())
	// Create one folder
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.AddStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// insert three sectors
	type expect struct {
		root  common.Hash
		count int
		data  []byte
	}
	sectors := size / storage.SectorSize
	expects := make([]expect, 0, sectors)
	for i := 0; i != int(sectors); i++ {
		// Add 8 sectors
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatal(err)
		}
		expects = append(expects, expect{
			data:  data,
			root:  root,
			count: 1,
		})
	}
	batch := make([]common.Hash, 0, 4)
	selectedIndex := rand.Perm(int(sectors))[:3]
	for _, i := range selectedIndex {
		batch = append(batch, expects[i].root)
	}
	var hash common.Hash
	rand.Read(hash[:])
	batch = append(batch, hash)
	if err := sm.AddSectorBatch(batch); err != nil {
		upErr, ok := err.(*updateError)
		if !ok {
			t.Fatalf("returned error not updateError type")
		}
		if upErr.prepareErr == nil {
			t.Fatalf("does not return prepareErr")
		}
	} else {
		t.Fatal("adding a non-exist root should give error")
	}
	// Check the values
	for _, expect := range expects {
		if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFoldersHasExpectedSectors(sm, 8); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, 10*time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

// TestAddBatchRecover test the add batch for recover
// After restart, should not exist
func TestAddBatchRecover(t *testing.T) {
	tests := []struct {
		keyWord string
		numTxn  int
	}{
		{"add batch process stop", 1},
		{"add batch prepare stop", 0},
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
		// insert three sectors
		type expect struct {
			root  common.Hash
			count int
			data  []byte
		}
		sectors := size / storage.SectorSize
		expects := make([]expect, 0, sectors)
		for i := 0; i != int(sectors); i++ {
			// Add 8 sectors
			data := randomBytes(storage.SectorSize)
			root := merkle.Root(data)
			if err := sm.AddSector(root, data); err != nil {
				t.Fatal(err)
			}
			expects = append(expects, expect{
				data:  data,
				root:  root,
				count: 1,
			})
		}
		batch := make([]common.Hash, 0, 3)
		selectedIndex := rand.Perm(int(sectors))[:3]
		for _, i := range selectedIndex {
			batch = append(batch, expects[i].root)
		}
		if err := sm.AddSectorBatch(batch); err != nil {
			upErr, ok := err.(*updateError)
			if !ok {
				t.Fatalf("returned error not updateError type")
			}
			if !upErr.isNil() {
				t.Fatalf("errStopped should not return error")
			}
		}
		// Check the values
		if err := checkFoldersHasExpectedSectors(sm, 8); err != nil {
			t.Fatal(err)
		}
		sm.shutdown(t, 10*time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), test.numTxn); err != nil {
			t.Fatal(err)
		}
		// Reopen the storage manager
		newsm, err := New(sm.persistDir)
		if err != nil {
			t.Fatalf("cannot create a new sm: %v", err)
		}
		newSM := newsm.(*storageManager)
		if err = newSM.Start(); err != nil {
			t.Fatal(err)
		}
		// wait for the updates to complete
		<-time.After(300 * time.Millisecond)
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, newSM, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		if err := checkFoldersHasExpectedSectors(newSM, 8); err != nil {
			t.Fatal(err)
		}
		newSM.shutdown(t, 10*time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAddBatchError test AddBatch with error input
func TestAddBatchProcessDisrupt(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"add batch process"},
		{"add batch prepare"},
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
		// insert three sectors
		type expect struct {
			root  common.Hash
			count int
			data  []byte
		}
		sectors := size / storage.SectorSize
		expects := make([]expect, 0, sectors)
		for i := 0; i != int(sectors); i++ {
			// Add 8 sectors
			data := randomBytes(storage.SectorSize)
			root := merkle.Root(data)
			if err := sm.AddSector(root, data); err != nil {
				t.Fatal(err)
			}
			expects = append(expects, expect{
				data:  data,
				root:  root,
				count: 1,
			})
		}
		batch := make([]common.Hash, 0, 3)
		selectedIndex := rand.Perm(int(sectors))[:3]
		for _, i := range selectedIndex {
			batch = append(batch, expects[i].root)
		}
		if err := sm.AddSectorBatch(batch); err != nil {
			upErr, ok := err.(*updateError)
			if !ok {
				t.Fatalf("returned error not updateError type")
			}
			if upErr.processErr == nil && upErr.prepareErr == nil {
				t.Fatalf("disrupt should give error")
			}
		} else {
			t.Fatal("disrupt should give error")
		}
		// Check the values
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		if err := checkFoldersHasExpectedSectors(sm, 8); err != nil {
			t.Fatal(err)
		}
		sm.shutdown(t, 10*time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}
