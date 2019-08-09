// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

func TestShrinkFolderNormal(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisruptor())
	// create 3 random folders, each with size 65 sectors
	numSectorPerFolder := uint64(16)
	var path string
	size := uint64(numSectorPerFolder * storage.SectorSize)
	for i := 0; i != 3; i++ {
		path = randomFolderPath(t, "")
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatal(err)
		}
	}
	// Insert sectors. To ensure there are sectors in each folder, create 65 * 2 + 1 sectors
	numSectors := numSectorPerFolder*2 + 1
	type expect struct {
		root  common.Hash
		count int
		data  []byte
	}
	expects := make([]expect, 0, numSectors)
	for i := 0; i != int(numSectors); i++ {
		data := randomBytes(storage.SectorSize)
		root := merkle.Sha256MerkleTreeRoot(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatal(err)
		}
		expects = append(expects, expect{
			data:  data,
			root:  root,
			count: 1,
		})
	}
	// Random pick 10 sectors to add again
	indexes := rand.Perm(int(numSectors))
	for _, index := range indexes[:10] {
		if err := sm.AddSector(expects[index].root, expects[index].data); err != nil {
			t.Fatal(err)
		}
		expects[index].count++
	}
	// Create a new storage folder with empty sectors
	newPath := randomFolderPath(t, "")
	if err := sm.AddStorageFolder(newPath, size); err != nil {
		t.Fatal(err)
	}
	newSize := 8 * storage.SectorSize
	if err := sm.shrinkFolder(path, newSize); err != nil {
		t.Fatal(err)
	}
	// After the shrink, all sectors must still exist
	for _, expect := range expects {
		if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
			t.Fatal(err)
		}
	}
	// After the shrink, the folder should have expected size
	if err := checkFolderSize(sm, path, newSize); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

func TestShrinkFolderDisrupted(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"shrink folder prepare normal"},
		{"shrink folder process normal"},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, "", d)
		// create 3 random folders, each with size 16 sectors
		numSectorPerFolder := uint64(16)
		var path string
		size := uint64(numSectorPerFolder * storage.SectorSize)
		for i := 0; i != 3; i++ {
			path = randomFolderPath(t, "")
			if err := sm.AddStorageFolder(path, size); err != nil {
				t.Fatal(err)
			}
		}
		// Insert sectors. To ensure there are sectors in each folder, create 65 * 2 + 1 sectors
		numSectors := numSectorPerFolder*2 + 1
		type expect struct {
			root  common.Hash
			count int
			data  []byte
		}
		expects := make([]expect, 0, numSectors)
		for i := 0; i != int(numSectors); i++ {
			data := randomBytes(storage.SectorSize)
			root := merkle.Sha256MerkleTreeRoot(data)
			if err := sm.AddSector(root, data); err != nil {
				t.Fatal(err)
			}
			expects = append(expects, expect{
				data:  data,
				root:  root,
				count: 1,
			})
		}
		// Random pick 10 sectors to add again
		indexes := rand.Perm(int(numSectors))
		for _, index := range indexes[:10] {
			if err := sm.AddSector(expects[index].root, expects[index].data); err != nil {
				t.Fatal(err)
			}
			expects[index].count++
		}
		// Create a new storage folder with empty sectors
		newPath := randomFolderPath(t, "")
		if err := sm.AddStorageFolder(newPath, size); err != nil {
			t.Fatal(err)
		}
		newSize := 8 * storage.SectorSize
		if err := sm.shrinkFolder(path, newSize); err == nil {
			t.Fatal(err)
		} else {
			upErr := err.(*updateError)
			if upErr.isNil() {
				t.Fatal("should return disrupted error")
			}
		}
		// After the shrink, all sectors must still exist
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		// After the shrink, the folder should have expected size
		if err := checkFolderSize(sm, path, size); err != nil {
			t.Fatal(err)
		}
		if err := checkFuncTimeout(time.Second, func() { sm.lock.Lock(); sm.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}
		sm.shutdown(t, time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestShrinkStorageFolderStopped(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"shrink folder prepare normal stop"},
		{"shrink folder process normal stop"},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, "", d)
		// create 3 random folders, each with size 16 sectors
		numSectorPerFolder := uint64(16)
		var path string
		size := uint64(numSectorPerFolder * storage.SectorSize)
		for i := 0; i != 3; i++ {
			path = randomFolderPath(t, "")
			if err := sm.AddStorageFolder(path, size); err != nil {
				t.Fatal(err)
			}
		}
		// Insert sectors. To ensure there are sectors in each folder, create 65 * 2 + 1 sectors
		numSectors := numSectorPerFolder*2 + 1
		type expect struct {
			root  common.Hash
			count int
			data  []byte
		}
		expects := make([]expect, 0, numSectors)
		for i := 0; i != int(numSectors); i++ {
			data := randomBytes(storage.SectorSize)
			root := merkle.Sha256MerkleTreeRoot(data)
			if err := sm.AddSector(root, data); err != nil {
				t.Fatal(err)
			}
			expects = append(expects, expect{
				data:  data,
				root:  root,
				count: 1,
			})
		}
		// Random pick 10 sectors to add again
		indexes := rand.Perm(int(numSectors))
		for _, index := range indexes[:10] {
			if err := sm.AddSector(expects[index].root, expects[index].data); err != nil {
				t.Fatal(err)
			}
			expects[index].count++
		}
		// Create a new storage folder with empty sectors
		newPath := randomFolderPath(t, "")
		if err := sm.AddStorageFolder(newPath, size); err != nil {
			t.Fatal(err)
		}
		newSize := 8 * storage.SectorSize
		if err := sm.shrinkFolder(path, newSize); err != nil {
			upErr := err.(*updateError)
			if upErr.isNil() {
				t.Fatal("should return disrupted error")
			}
		}
		sm.shutdown(t, time.Second)
		newsm, err := New(sm.persistDir)
		if err != nil {
			t.Fatalf("cannot create a new sm: %v", err)
		}
		newSM := newsm.(*storageManager)
		if err = newSM.Start(); err != nil {
			t.Fatal(err)
		}
		// After the shrink, all sectors must still exist
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, newSM, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		// After the shrink, the folder should have expected size
		if err := checkFolderSize(newSM, path, size); err != nil {
			t.Fatal(err)
		}
		if err := checkFuncTimeout(time.Second, func() { newSM.lock.Lock(); newSM.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}

		newSM.shutdown(t, time.Second)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
	}
}
