// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxdir

import (
	"os"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/storage"
)

var testDirSetDir = tempDir("dirset")

// newTestDir create a DirSet with an entry. Return the dirset and the entry
func newTestDirSet(t *testing.T) (*DirSet, *DirSetEntryWithID) {
	// Initialize
	depth := 3
	path := randomDxPath(depth)
	dxPath, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	ds, err := NewDirSet(testDirSetDir.Join(dxPath), newWal(t))
	if err != nil {
		t.Fatal(err)
	}
	// Add a new entry
	entry, err := ds.NewDxDir(path)
	if err != nil {
		t.Fatal(err)
	}
	// return the DirSet with the entry
	return ds, entry
}

// TestNewDirSet_Start test NewDirSet and Start function
func TestNewDirSet(t *testing.T) {
	ds, err := NewDirSet(testDirSetDir, newWal(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(string(ds.rootDir.Join(storage.RootDxPath()))); err != nil {
		t.Fatal("after start, the root dir is not initialized")
	}
	_, err = os.Stat(string(ds.rootDir.Join(storage.RootDxPath(), DirFileName)))
	if err != nil {
		t.Fatalf("file not exist: %v", ds.rootDir.Join(storage.RootDxPath(), DirFileName))
	}
	// Create a new DirSet with the same directory, no error should be reported
	_, err = NewDirSet(testDirSetDir, ds.wal)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

// TestDirSet_NewDxDir test the functionality of DirSet.NewDxDir
func TestDirSet_NewDxDir(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	// Create a new DxDir as same as an existing entry. should return error
	if _, err := ds.NewDxDir(path); !os.IsExist(err) {
		t.Fatalf("creating an existing dir: %v", err)
	}
	// Create a new DxDir without collision, should not return error
	newEntry, err := ds.NewDxDir(randomDxPath(2))
	if err != nil {
		t.Fatal(err)
	}
	// Now two entries should be in use
	if len(ds.dirMap) != 2 {
		t.Errorf("dirMap should have two entries. Got: %d", len(ds.dirMap))
	}
	// Close the two entries
	if err = entry.Close(); err != nil {
		t.Fatal(err)
	}
	if err = newEntry.Close(); err != nil {
		t.Fatal(err)
	}
	// After closing all entries, the size of the map should be 0
	if len(ds.dirMap) != 0 {
		t.Errorf("After closing all files, ds.dirMap length %d", len(ds.dirMap))
	}
}

// TestNewDirSet_OpenClose test the process of opening and closing a DxDir
func TestNewDirSet_OpenClose(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	// Open an existing DxDir. It should exist
	newEntry, err := ds.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	exist := ds.Exists(path)
	if !exist {
		t.Errorf("file not exist")
	}
	// After close the newEntry, the previous entry is still stored in the map
	err = newEntry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 1 {
		t.Errorf("All dirs should have been closed")
	}
	// After close, the DxDir should still exist
	exist = ds.Exists(path)
	if !exist {
		t.Errorf("After close the file, should exist on disk")
	}
	// After closing all files, the map should have 0 length. And the file should still exist
	if err = entry.Close(); err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 0 {
		t.Errorf("After closing all entries, dir map should have length 0")
	}
	if exist = ds.Exists(path); !exist {
		t.Errorf("After close the file, should exist on disk")
	}
}

// TestDirSet_Delete test the functionality of deletion
func TestDirSet_Delete(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	// After deletion, the file should not exist, and marked as deleted
	if err := ds.Delete(path); err != nil {
		t.Fatal(err)
	}
	exist := ds.Exists(path)
	if exist {
		t.Errorf("the file should not exist")
	}
	if !entry.Deleted() {
		t.Errorf("After deletion, file not deleted")
	}
	// There are no more entries in the map
	if len(ds.dirMap) != 0 {
		t.Errorf("The file should have been deleted from ds,dirMap")
	}
	if err := entry.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestDirSet_UpdateMetadata test the functionality of Update
func TestDirSet_UpdateMetadata(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	newMeta := randomMetadata()
	err := ds.UpdateMetadata(path, *newMeta)
	if err != nil {
		t.Fatal(err)
	}
	newMeta.DxPath = path
	newMeta.RootPath = entry.metadata.RootPath
	// After update, the metadata should be the same expect for the time modify field
	newMeta.TimeModify = 0
	entry.metadata.TimeModify = 0
	if !reflect.DeepEqual(*newMeta, entry.Metadata()) {
		t.Errorf("After update metadata, not equal. \n\tGot %+v, \n\tExpect %+v", entry.metadata, newMeta)
	}
	// Close the entry, and reopen it. Recovered DxDir should have the same metadata
	if err := entry.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := ds.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	recovered.metadata.TimeModify = 0
	if !reflect.DeepEqual(recovered.Metadata(), *newMeta) {
		t.Errorf("Recovered DxDir's metadata not equal.\n\tGot %+v\n\tExpect %+v", recovered.metadata, newMeta)
	}
}
