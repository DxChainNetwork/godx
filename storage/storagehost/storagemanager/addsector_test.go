// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"

	"github.com/syndtr/goleveldb/leveldb/util"
)

func TestAddSector(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisruptor())
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.AddStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// Create the sector
	data := randomBytes(storage.SectorSize)
	root := merkle.Root(data)
	if err := sm.AddSector(root, data); err != nil {
		t.Fatal(err)
	}
	// Post add sector check
	// The sector shall be stored in db
	err := checkSectorExist(root, sm, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkFoldersHasExpectedSectors(sm, 1); err != nil {
		t.Fatal(err)
	}
	// Create a virtual sector
	if err := sm.AddSector(root, data); err != nil {
		t.Fatal(err)
	}
	err = checkSectorExist(root, sm, data, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkFoldersHasExpectedSectors(sm, 1); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, 10*time.Millisecond)
	if err = checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

// TestDisruptedPhysicalAddSector test the case of disrupted during add physical sectors
func TestDisruptedPhysicalAddSector(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"physical process normal"},
		{"physical prepare normal"},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, test.keyWord, d)
		path := randomFolderPath(t, test.keyWord)
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		// Create the sector
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err == nil {
			t.Fatalf("test %v: disrupting does not give error", test.keyWord)
		}
		id := sm.calculateSectorID(root)
		if err := checkSectorNotExist(id, sm); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		if err := checkFoldersHasExpectedSectors(sm, 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		sm.shutdown(t, 10*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
	}
}

// TestDisruptedVirtualAddSector test the case of disrupted during add virtual sectors
func TestDisruptedVirtualAddSector(t *testing.T) {
	tests := []struct {
		keyWord string
	}{
		{"virtual process normal"},
		{"virtual prepare normal"},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, test.keyWord, d)
		path := randomFolderPath(t, test.keyWord)
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		// Create the sector
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatalf("test %v: first add sector give error: %v", test.keyWord, err)
		}
		if err := sm.AddSector(root, data); err == nil {
			t.Fatalf("test %v: second add sector does not give error: %v", test.keyWord, err)
		}
		if err := checkSectorExist(root, sm, data, 1); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		if err := checkFoldersHasExpectedSectors(sm, 1); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		sm.shutdown(t, 10*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
	}
}

// TestAddSectorStop test the scenario of stop and recover
func TestAddSectorStopRecoverPhysical(t *testing.T) {
	tests := []struct {
		keyWord string
		numTxn  int
	}{
		{"physical prepare normal stop", 0},
		{"physical process normal stop", 1},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, test.keyWord, d)
		path := randomFolderPath(t, test.keyWord)
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatalf("test %v: errStop should not give error: %v", test.keyWord, err)
		}
		id := sm.calculateSectorID(root)
		sm.shutdown(t, 100*time.Millisecond)
		// The update should not be released
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), test.numTxn); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		newsm, err := New(sm.persistDir)
		if err != nil {
			t.Fatalf("cannot create a new sm: %v", err)
		}
		newSM := newsm.(*storageManager)
		if err = newSM.Start(); err != nil {
			t.Fatal(err)
		}
		// wait for the update to complete
		<-time.After(100 * time.Millisecond)
		if err := checkSectorNotExist(id, newSM); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		if err := checkFoldersHasExpectedSectors(newSM, 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		newSM.shutdown(t, 100*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
	}
}

// TestAddSectorsStopRecoverVirtual test stopped during adding virtual sectors
func TestAddSectorsStopRecoverVirtual(t *testing.T) {
	tests := []struct {
		keyWord string
		numTxn  int
	}{
		{"virtual process normal stop", 1},
		{"virtual prepare normal stop", 0},
	}
	for _, test := range tests {
		d := newDisruptor().register(test.keyWord, func() bool { return true })
		sm := newTestStorageManager(t, test.keyWord, d)
		path := randomFolderPath(t, test.keyWord)
		size := uint64(1 << 25)
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		data := randomBytes(storage.SectorSize)
		root := merkle.Root(data)
		if err := sm.AddSector(root, data); err != nil {
			t.Fatalf("test %v: add physical sector should not give error: %v", test.keyWord, err)
		}
		if err := sm.AddSector(root, data); err != nil {
			t.Fatalf("test %v: errStop should not give error: %v", test.keyWord, err)
		}
		sm.shutdown(t, 100*time.Millisecond)
		// The update should not be released
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), test.numTxn); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		newsm, err := New(sm.persistDir)
		if err != nil {
			t.Fatalf("cannot create a new sm: %v", err)
		}
		newSM := newsm.(*storageManager)
		if err = newSM.Start(); err != nil {
			t.Fatal(err)
		}
		// wait for the update to complete
		<-time.After(100 * time.Millisecond)
		if err := checkSectorExist(root, newSM, data, 1); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		if err := checkFoldersHasExpectedSectors(newSM, 1); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
		newSM.shutdown(t, 100*time.Millisecond)
		if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
			t.Fatalf("test %v: %v", test.keyWord, err)
		}
	}
}

// TestAddSectorConcurrent test the scenario of multiple goroutines add sector at the same time
func TestAddSectorConcurrent(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisruptor())
	size := uint64(1 << 25)
	// add three storage folders, each have 8 sectors
	for i := 0; i != 3; i++ {
		path := randomFolderPath(t, "")
		if err := sm.AddStorageFolder(path, size); err != nil {
			t.Fatalf("test %v: %v", "", err)
		}
	}
	// create 24 valid sectors, with a mix of virtual and invalid sectors
	var numSectors, numVirtual, numTotal uint32
	var wg sync.WaitGroup
	type expectSector struct {
		count uint64
		data  []byte
	}
	expect := make(map[common.Hash]expectSector)
	var expectLock sync.Mutex
	errChan := make(chan error, 1)
	stopChan := time.After(20 * time.Second)
	for atomic.LoadUint32(&numSectors) < 24 {
		// fatal err if timeout
		select {
		case <-stopChan:
			t.Fatalf("After 10 seconds, still not complete")
		case err := <-errChan:
			t.Fatalf("error happend %v", err)
		default:
		}
		// increment numSectors when a sector is successfully added
		wg.Add(1)
		go func() {
			atomic.AddUint32(&numTotal, 1)
			defer wg.Done()

			// The possibility of adding a virtual is 25% when numSectors is not 0
			virtual := atomic.LoadUint32(&numSectors) > 0 && (randomUint32()%4 == 0)
			if virtual {
				atomic.AddUint32(&numVirtual, 1)
				var rt common.Hash
				var data []byte
				expectLock.Lock()
				for sectorRoot, sector := range expect {
					rt = sectorRoot
					data = sector.data
					count := sector.count + 1
					expect[sectorRoot] = expectSector{
						count: count,
						data:  data,
					}
					break
				}
				expectLock.Unlock()
				if err := sm.AddSector(rt, data); err != nil {
					errChan <- fmt.Errorf("add virtual sector cause an error: %v", err)
				}
			} else {
				// create a random sector
				data := randomBytes(storage.SectorSize)
				root := merkle.Root(data)
				expectLock.Lock()
				if _, exist := expect[root]; exist {
					// The root exist. Thus unlock and return
					expectLock.Unlock()
					return
				}
				expectLock.Unlock()
				if err := sm.AddSector(root, data); err != nil {
					upErr, isUpErr := err.(*updateError)
					if !isUpErr {
						errChan <- err
						return
					}
					if upErr.isNil() || upErr.prepareErr == errAllFoldersFullOrUsed {
						// Currently busy
						return
					}
					// unexpected error
					if !upErr.isNil() {
						errChan <- err
						return
					}
				}
				// There is no error. Save the data to expect
				expectLock.Lock()
				expect[root] = expectSector{
					count: 1,
					data:  data,
				}
				expectLock.Unlock()
				atomic.AddUint32(&numSectors, 1)
			}
		}()
		<-time.After(50 * time.Millisecond)
	}
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()
	select {
	case err := <-errChan:
		t.Fatal(err)
	case <-waitChan:
	case <-time.After(60 * time.Second):
		t.Fatalf("time out")
	}

	// Check the result
	t.Logf("after %v tries, added %v virtual sectors, got %v sectors", numTotal, numVirtual, numSectors)
	for rt, expect := range expect {
		if err := checkSectorExist(rt, sm, expect.data, expect.count); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkFoldersHasExpectedSectors(sm, 24); err != nil {
		t.Fatal(err)
	}
	sm.shutdown(t, 1*time.Second)
	if err := checkWalTxnNum(filepath.Join(sm.persistDir, walFileName), 0); err != nil {
		t.Fatal(err)
	}
}

// checkSectorExist checks whether the sector exists
func checkSectorExist(root common.Hash, sm *storageManager, data []byte, count uint64) (err error) {
	id := sm.calculateSectorID(root)
	sector, err := sm.db.getSector(id)
	if err != nil {
		return err
	}
	if sector.count != count {
		return fmt.Errorf("db sector count not expected. Got %v, Expect %v", sector.count, count)
	}
	folderID := sector.folderID
	folderPath, err := sm.db.getFolderPath(folderID)
	if err != nil {
		return err
	}
	// DB folder should have expected data
	dbFolder, err := sm.db.loadStorageFolder(folderPath)
	if err != nil {
		return err
	}
	if dbFolder.storedSectors < 1 {
		return fmt.Errorf("folders has no stored sectors")
	}
	if err = dbFolder.setUsedSectorSlot(sector.index); err == nil {
		return fmt.Errorf("folder's %d entry shall be occupied", sector.index)
	}
	// DB folder should have the map from folder id to sector id
	key := makeFolderSectorKey(folderID, id)
	exist, err := sm.db.lvl.Has(key, nil)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("folder id to sector id not exist")
	}
	// memoryFolder
	mmFolder, err := sm.folders.getWithoutLock(folderPath)
	if err != nil {
		return err
	}
	if err = mmFolder.setUsedSectorSlot(sector.index); err == nil {
		return fmt.Errorf("folders %d entry shall be occupied", sector.index)
	}
	if mmFolder.storedSectors < 1 {
		return fmt.Errorf("folders has no occupied sector")
	}
	// check whether the sector data is saved correctly
	b := make([]byte, storage.SectorSize)
	n, err := mmFolder.dataFile.ReadAt(b, int64(sector.index*storage.SectorSize))
	if err != nil {
		return err
	}
	if uint64(n) != storage.SectorSize {
		return fmt.Errorf("read size not equal to sectorSize. Got %v, Expect %v", n, storage.SectorSize)
	}
	if !bytes.Equal(data, b) {
		return fmt.Errorf("data not correctly saved on disk.\n\tGot %x\n\texpect %x", b[0:10], data[0:10])
	}
	c := make(chan struct{})
	go func() {
		sm.sectorLocks.lockSector(sector.id)
		defer sm.sectorLocks.unlockSector(sector.id)

		close(c)
	}()
	<-time.After(10 * time.Millisecond)
	select {
	case <-c:
	default:
		err = errors.New("sector still locked")
	}
	if err != nil {
		return
	}
	// Lastly, use the ReadSector method to check the data equality
	b, err = sm.ReadSector(root)
	if err != nil {
		return
	}
	if !bytes.Equal(data, b) {
		return fmt.Errorf("data bytes not equal")
	}
	return nil
}

// checkFoldersHasExpectedSectors checks both in-memory folders and db folders
// has expected number of stored segments
func checkFoldersHasExpectedSectors(sm *storageManager, expect int) (err error) {
	if err = checkExpectStoredSectors(sm.folders.sfs, expect); err != nil {
		return fmt.Errorf("memory %v", err)
	}
	folders, err := sm.db.loadAllStorageFolders()
	if err != nil {
		return err
	}
	if err = checkExpectStoredSectors(folders, expect); err != nil {
		return fmt.Errorf("db: %v", err)
	}
	iter := sm.db.lvl.NewIterator(util.BytesPrefix([]byte(prefixFolderSector)), nil)
	var count int
	for iter.Next() {
		count++
	}
	if count != expect {
		return fmt.Errorf("folders has unexpected stored sectors. Expect %v, Got %v", expect, count)
	}
	return nil
}

// checkSectorNotExist checks whether the sector exists in storage folder
// if exist, return an error
func checkSectorNotExist(id sectorID, sm *storageManager) (err error) {
	exist, err := sm.db.hasSector(id)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("sector %x shall not exist in storage manager", id)
	}
	iter := sm.db.lvl.NewIterator(util.BytesPrefix([]byte(prefixFolderSector)), nil)
	for iter.Next() {
		key := string(iter.Key())
		if strings.HasSuffix(key, common.Bytes2Hex(id[:])) {
			return fmt.Errorf("sector is registered in folder: %v", key)
		}
	}
	return nil
}

// checkWalTxnNum checks the number of returned transactions is of the expect number
func checkWalTxnNum(path string, numTxn int) (err error) {
	wal, txns, err := writeaheadlog.New(path)
	if err != nil {
		return err
	}
	if len(txns) != numTxn {
		return fmt.Errorf("reopen wal give %v transactions. Expect %v", len(txns), numTxn)
	}
	_, err = wal.CloseIncomplete()
	if err != nil {
		return
	}
	return
}

// checkExpectNumSectors checks whether the total number of sectors in folders are
// consistent with totalNumSectors
func checkExpectStoredSectors(folders map[string]*storageFolder, totalStoredSectors int) (err error) {
	var sum uint64
	for _, sf := range folders {
		sum += sf.storedSectors
	}
	if sum != uint64(totalStoredSectors) {
		return fmt.Errorf("totalStoredSectors not expected. Expect %v, Got %v", totalStoredSectors, sum)
	}
	return nil
}
