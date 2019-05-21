package dxdir

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var testDirSetDir = tempDir("dirset")

// newTestDir create a DirSet with an entry. Return the dirset and the entry
func newTestDirSet(t *testing.T) (*DirSet, *DirSetEntryWithID) {
	depth := 3
	path := randomDxPath(depth)
	ds := NewDirSet(testDirSetDir, newWal(t))
	entry, err := ds.NewDxDir(path)
	if err != nil {
		t.Fatal(err)
	}
	return ds, entry
}

// TestNewDirSet_Start test NewDirSet and Start function
func TestNewDirSet_Start(t *testing.T) {
	ds := NewDirSet(testDirSetDir, newWal(t))
	err := ds.Start()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(testDirSetDir, dirFileName))
	if err != nil {
		t.Fatalf("file not exist: %v", filepath.Join(testDirSetDir, dirFileName))
	}
	err = ds.Start()
	if err != os.ErrExist {
		t.Fatalf("file shoud exist: %v", err)
	}
}

func TestDirSet_NewDxDir(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	// Create a new DxDir as existing entry
	if _, err := ds.NewDxDir(path); !os.IsExist(err) {
		t.Fatalf("creating an existing dir: %v", err)
	}
	newEntry, err := ds.NewDxDir(randomDxPath(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 2 {
		t.Errorf("dirMap should have two entries. Got: %d", len(ds.dirMap))
	}
	if err = entry.Close(); err != nil {
		t.Fatal(err)
	}
	if err = newEntry.Close(); err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 0 {
		t.Errorf("After closing all files, ds.dirMap length %d", len(ds.dirMap))
	}
}

// TestNewDirSet_OpenClose test the process of opening and closing a DxDir
func TestNewDirSet_OpenClose(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	newEntry, err := ds.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	exist, err := ds.Exists(path)
	if err != nil {
		t.Fatal(err)
	}
	if !exist {
		t.Errorf("file not exist")
	}
	err = newEntry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 1 {
		t.Errorf("All dirs should have been closed")
	}
	exist, err = ds.Exists(path)
	if err != nil {
		t.Fatal(err)
	}
	if !exist {
		t.Errorf("After close the file, should exist on disk")
	}
	if err = entry.Close(); err != nil {
		t.Fatal(err)
	}
	if len(ds.dirMap) != 0 {
		t.Errorf("After closing all entries, dir map should have length 0")
	}
}

// TestDirSet_Delete test the functionality of deletion
func TestDirSet_Delete(t *testing.T) {
	ds, entry := newTestDirSet(t)
	path := entry.DxPath()
	if err := ds.Delete(path); err != nil {
		t.Fatal(err)
	}
	exist, err := ds.Exists(path)
	if err != nil && !os.IsNotExist(err) {
		t.Error(err)
	}
	if exist {
		t.Errorf("the file should not exist")
	}
	if !entry.Deleted() {
		t.Errorf("After deletion, file not deleted")
	}
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
	if !reflect.DeepEqual(*newMeta, entry.Metadata()) {
		t.Errorf("After update metadata, not equal. \n\tGot %+v, \n\tExpect %+v", entry.metadata, newMeta)
	}
	if err := entry.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := ds.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered.Metadata(), *newMeta) {
		t.Errorf("Recovered DxDir's metadata not equal.\n\tGot %+v\n\tExpect %+v", recovered.metadata, newMeta)
	}
}
