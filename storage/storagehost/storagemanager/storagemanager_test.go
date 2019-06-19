// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "storagemanager", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v: %v", path, err))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

// newTestDatabase create the database for testing
func newTestDatabase(t *testing.T, extra string) (db *database) {
	var dbPath string
	if len(extra) != 0 {
		dbPath = tempDir(t.Name(), extra, databaseFileName)
	} else {
		dbPath = tempDir(t.Name(), databaseFileName)
	}

	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// newTestStorageManager creates a new storageManager for testing
func newTestStorageManager(t *testing.T, extra string, d *disruptor) (sm *storageManager) {
	sm, err := newStorageManager(tempDir(t.Name(), extra), d)
	if err != nil {
		t.Fatal(err)
	}
	if err = sm.Start(); err != nil {
		t.Fatal(err)
	}
	return sm
}

// checkFastShutdown shutdown the storage manager.
// The function is only used in test, and should be close within the timeout
func (sm *storageManager) shutdown(t *testing.T, timeout time.Duration) {
	c := make(chan struct{})
	var err error
	go func() {
		err = sm.Close()
		close(c)
	}()
	select {
	case <-c:
	case <-time.After(timeout):
		t.Fatalf("After %v, storage manager still not closed", timeout)
	}
	if err != nil {
		t.Fatalf("close return error: %v", err)
	}
}

// TestEmptyStorageManager test the open-close process of an empty storageManager
func TestEmptyStorageManager(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisrupter())
	prevSalt := sm.sectorSalt
	if sm.sectorSalt == [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} {
		t.Fatalf("salt shall not be empty")
	}
	if sm.folders.size() != 0 {
		t.Fatalf("folders size not empty: %v", sm.folders.size())
	}
	if len(sm.sectorLocks.locks) != 0 {
		t.Fatalf("sector locks not emmpty: %v", len(sm.sectorLocks.locks))
	}
	sm.shutdown(t, 100*time.Millisecond)
	// Create a new storage manager, which should have the same sectorSalt
	newsm, err := New(sm.persistDir)
	if err != nil {
		t.Fatalf("cannot create a new sm: %v", err)
	}
	newSM := newsm.(*storageManager)
	if err = newSM.Start(); err != nil {
		t.Fatal(err)
	}

	if newSM.sectorSalt != prevSalt {
		t.Fatalf("reopened storage manage not having the same sector salt\n\tprevious %v\n\tgot %v", prevSalt, newSM.sectorSalt)
	}
	if newSM.folders.size() != 0 {
		t.Fatalf("folders size not empty: %v", newSM.folders.size())
	}
	if len(newSM.sectorLocks.locks) != 0 {
		t.Fatalf("sector locks not emmpty: %v", len(newSM.sectorLocks.locks))
	}
}

// randomFolderPath create a random folder path under the testing directory
func randomFolderPath(t *testing.T, extra string) (path string) {
	path = filepath.Join(os.TempDir(), "storagemanager", filepath.Join(t.Name()), extra)
	b := make([]byte, 16)
	rand.Read(b)
	folderName := common.Bytes2Hex(b)
	path = filepath.Join(path, folderName)
	return path
}

// randomBytes create a random byte slice of specified size
func randomBytes(size uint64) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func randomUint32() uint32 {
	b := make([]byte, 4)
	rand.Read(b)
	return binary.LittleEndian.Uint32(b)
}
