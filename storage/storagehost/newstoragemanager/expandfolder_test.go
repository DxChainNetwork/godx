// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

func TestExpandFolderNormal(t *testing.T) {
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
	expandSize := uint64(storage.SectorSize * 65)
	if err := sm.expandFolder(path, expandSize); err != nil {
		t.Fatal(err)
	}
	for _, expect := range expects {
		if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFolderSize(sm, path, expandSize); err != nil {
		t.Fatal(err)
	}
	// check locks
	if err := checkFuncTimeout(1*time.Second, func() { sm.lock.Lock(); sm.lock.Unlock() }); err != nil {
		t.Fatal(err)
	}
	if err := checkFuncTimeout(1*time.Second, func() { sm.folders.lock.Lock(); sm.folders.lock.Unlock() }); err != nil {
		t.Fatal(err)
	}
	// shutdown the sm and check wal
	sm.shutdown(t, 100*time.Millisecond)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
	// after test, delete the folder since it might be too large
	_ = os.Remove(filepath.Join(path, dataFileName))
}

func TestExpandFolderDisrupt(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"expand folder prepare normal"},
		{"expand folder process normal"},
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
		expandSize := uint64(storage.SectorSize * 65)
		err := sm.expandFolder(path, expandSize)
		fmt.Println(err)
		if err == nil {
			t.Fatal("disrupt does not give error")
		}
		fmt.Println(err)
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, sm, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		if err := checkFolderSize(sm, path, size); err != nil {
			t.Fatal(err)
		}
		// check locks
		if err := checkFuncTimeout(1*time.Second, func() { sm.lock.Lock(); sm.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}
		if err := checkFuncTimeout(1*time.Second, func() { sm.folders.lock.Lock(); sm.folders.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}
		// shutdown the sm and check wal
		sm.shutdown(t, 100*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
		// after test, delete the folder since it might be too large
		_ = os.Remove(filepath.Join(path, dataFileName))
	}
}

func TestExpandFolderStop(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"expand folder prepare normal stop"},
		{"expand folder process normal stop"},
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
		expandSize := uint64(storage.SectorSize * 65)
		err := sm.expandFolder(path, expandSize)
		if err != nil {
			upErr := err.(*updateError)
			if !upErr.isNil() {
				t.Fatal("stop should give nil error")
			}
		}
		fmt.Println(err)
		// shut down the sm
		sm.shutdown(t, time.Second)
		newSM, err := New(sm.persistDir)
		if err != nil {
			t.Fatal(err)
		}
		if err = newSM.Start(); err != nil {
			t.Fatal(err)
		}
		<-time.After(300 * time.Millisecond)
		for _, expect := range expects {
			if err := checkSectorExist(expect.root, newSM, expect.data, uint64(expect.count)); err != nil {
				t.Fatal(err)
			}
		}
		if err := checkFolderSize(newSM, path, size); err != nil {
			t.Fatal(err)
		}
		// check locks
		if err := checkFuncTimeout(1*time.Second, func() { newSM.lock.Lock(); newSM.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}
		if err := checkFuncTimeout(1*time.Second, func() { newSM.folders.lock.Lock(); newSM.folders.lock.Unlock() }); err != nil {
			t.Fatal(err)
		}
		// shutdown the sm and check wal
		newSM.shutdown(t, 100*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(newSM.persistDir, walFileName), 0); err != nil {
			t.Fatal(err)
		}
		// after test, delete the folder since it might be too large
		_ = os.Remove(filepath.Join(path, dataFileName))
	}
}

func checkFolderSize(sm *storageManager, folderPath string, size uint64) (err error) {
	// check the memory folder size
	sf, err := sm.folders.getWithoutLock(folderPath)
	if err != nil {
		return err
	}
	if sf.numSectors != sizeToNumSectors(size) {
		return fmt.Errorf("memory: num sectors not expected. expect %v, got %v", sizeToNumSectors(size), sf.numSectors)
	}
	usageSize := size / storage.SectorSize / bitVectorGranularity
	if size/storage.SectorSize%bitVectorGranularity != 0 {
		usageSize++
	}
	if uint64(len(sf.usage)) != usageSize {
		return fmt.Errorf("memory: usage size not expected. expect %v, got %v", usageSize, len(sf.usage))
	}
	if err = checkFuncTimeout(100*time.Millisecond, func() { sf.lock.Lock(); sf.lock.Unlock() }); err != nil {
		return err
	}
	// check the db folder size
	dbsf, err := sm.db.loadStorageFolder(folderPath)
	if err != nil {
		return err
	}
	if dbsf.numSectors != sizeToNumSectors(size) {
		return fmt.Errorf("db: num sectors not expected. expect %v, got %v", sizeToNumSectors(size), dbsf.numSectors)
	}
	if uint64(len(dbsf.usage)) != usageSize {
		return fmt.Errorf("db: usage size not expected")
	}
	// Lastly check the folder on disk
	fileInfo, err := os.Stat(filepath.Join(folderPath, dataFileName))
	if err != nil {
		return err
	}
	if fileInfo.Size() != int64(size) {
		return fmt.Errorf("file size not expected. expect %v, got %v", size, fileInfo.Size())
	}
	return nil
}

func checkFuncTimeout(timeout time.Duration, f func()) (err error) {
	waitChan := make(chan struct{})
	go func() {
		f()
		close(waitChan)
	}()
	select {
	case <-waitChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	}
}
