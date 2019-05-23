// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// newTestFileSet create a FileSet for test usage, and added a new DxFile to the FileSet.
// return the added DxFile and the new FileSet.
func newTestFileSet(t *testing.T) (*FileSetEntryWithID, *FileSet) {
	wal, _ := newWal(t)
	fs := NewFileSet(testDir, wal)
	ec, err := erasurecode.New(erasurecode.ECTypeStandard, 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := fs.NewDxFile(randomDxPath(), "", false, ec, ck, 1<<24, 0777)
	if err != nil {
		t.Fatal(err)
	}
	return entry, fs
}

// newWal create a new Wal object for testing purpose
func newWal(t *testing.T) (*writeaheadlog.Wal, string) {
	walPath := filepath.Join(testDir, t.Name()+"wal")
	wal, recoveredTxns, err := writeaheadlog.New(walPath)
	if err != nil {
		panic(err)
	}
	for _, txn := range recoveredTxns {
		err := txn.Release()
		if err != nil {
			panic(err)
		}
	}
	return wal, walPath
}

// randomDxPath creates a random DxPath which is a string of byte slice of length 16
func randomDxPath() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return common.Bytes2Hex(b)
}

// TestCopyEntry test FileSet.CopyEntry
func TestCopyEntry(t *testing.T) {
	entry, fs := newTestFileSet(t)
	copied := entry.CopyEntry()
	if !reflect.DeepEqual(entry.fileSetEntry, copied.fileSetEntry) {
		t.Errorf("fileSetEntry not equal: original: %+v, copied: %+v", entry.fileSetEntry, copied.fileSetEntry)
	}
	if entry.threadID == copied.threadID {
		t.Errorf("entry and copied share the same threadID")
	}
	err := copied.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 1 {
		t.Errorf("fileMap size Expect: 1, Got: %d", len(fs.filesMap))
	}
	err = entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 0 {
		t.Errorf("filesMap size Expect: 0, Got: %d", len(fs.filesMap))
	}
}

// TestFileSet_NewDxFile test FileSet.NewDxFile
func TestFileSet_NewDxFile(t *testing.T) {
	entry, fs := newTestFileSet(t)
	dxPath := entry.metadata.DxPath
	// create an existing DxFile
	ec, err := erasurecode.New(erasurecode.ECTypeStandard, 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.NewDxFile(dxPath, "", false, ec, ck, 1<<24, 0777)
	if err != ErrFileExist {
		t.Fatalf("New an existing DxFile, dont return file exist error: %v", err)
	}
	newEntry, err := fs.NewDxFile(randomDxPath(), "", false, ec, ck, 1<<24, 0777)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(testDir, newEntry.metadata.DxPath)); err != nil {
		t.Errorf("Creating a DxFile, the file does not exist: %v", err)
	}
}

// TestFileSet_CloseOpen test the FileSet Close and Open process.
// First close the file, and then open the file. The process should not give error.
func TestFileSet_CloseOpen(t *testing.T) {
	entry, fs := newTestFileSet(t)
	dxPath := entry.metadata.DxPath
	err := entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 0 {
		t.Errorf("After close, filesMap size Expect: 0, Got: %d", len(fs.filesMap))
	}
	recovered, err := fs.Open(dxPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkDxFileEqual(recovered.fileSetEntry.DxFile, entry.fileSetEntry.DxFile); err != nil {
		t.Error(err)
	}
}

// TestFileSet_OpenNonExist test the process of opening a not existed DxFile
func TestFileSet_OpenNonExist(t *testing.T) {
	entry, fs := newTestFileSet(t)
	err := entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	emptyDxPath := randomDxPath()
	exist := fs.Exists(emptyDxPath)
	if exist {
		t.Errorf("not expected non-existing dxpath give exist==true")
	}
	_, err = fs.Open(emptyDxPath)
	if err != ErrUnknownFile {
		t.Errorf("not expect error. Expect %v, Got %v", ErrUnknownFile, err)
	}
}

// TestFileSet_DeleteOpen test the process of delete and then open a file.
func TestFileSet_DeleteOpen(t *testing.T) {
	entry, fs := newTestFileSet(t)
	dxPath := entry.metadata.DxPath
	err := entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 0 {
		t.Errorf("After close, filesMap size Expect: 0, Got: %d", len(fs.filesMap))
	}
	err = fs.Delete(dxPath)
	if err != nil {
		t.Fatal(err)
	}
	exist := fs.Exists(dxPath)
	if exist {
		t.Errorf("File should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(testDir, dxPath)); !os.IsNotExist(err) {
		t.Errorf("After delete, previous dxPath file should not exist: %v", err)
	}
	// Open the file now, the file should be deleted
	_, err = fs.Open(dxPath)
	if err != ErrUnknownFile {
		t.Errorf("open a deleted file does not give ErrUnknownFile. got %v", err)
	}
	err = entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 0 {
		t.Errorf("After closing all entries, the size of filesMap is not 0: %d", len(fs.filesMap))
	}
	exist = fs.Exists(dxPath)
	if exist {
		t.Errorf("File should have been deleted")
	}
}

// TestFileSet_RenameOpen test the process of rename a DxFile and then open it.
func TestFileSet_RenameOpen(t *testing.T) {
	entry, fs := newTestFileSet(t)
	prevDxPath := entry.metadata.DxPath
	newDxPath := randomDxPath()
	err := fs.Rename(prevDxPath, newDxPath)
	if err != nil {
		t.Fatal(err)
	}
	exist := fs.Exists(prevDxPath)
	if exist {
		t.Errorf("After rename, the previous dxPath should not exist")
	}
	exist2 := fs.Exists(newDxPath)
	if !exist2 {
		t.Errorf("After rename, the new dxPath should exists")
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(testDir, prevDxPath)); !os.IsNotExist(err) {
		t.Errorf("After rename, previous dxPath file should not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(testDir, newDxPath)); err != nil {
		t.Errorf("After rename, the new dxPath file should exist: %v", err)
	}
	if len(fs.filesMap) != 1 {
		t.Errorf("after rename, the size of filesMap is not 1: %d", len(fs.filesMap))
	}
	err = entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.filesMap) != 0 {
		t.Errorf("After closing all entries, the size of filesMap is not 0: %d", len(fs.filesMap))
	}
	newEntry, err := fs.Open(newDxPath)
	if err != nil {
		t.Errorf("After rename, open the new file give error: %v", err)
	}
	if len(fs.filesMap) != 1 {
		t.Errorf("After open the renamed file, filesmap size not 1: %d", len(fs.filesMap))
	}
	err = newEntry.Close()
	if err != nil {
		t.Fatal(err)
	}
}
