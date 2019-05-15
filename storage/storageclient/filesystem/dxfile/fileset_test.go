package dxfile

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"path/filepath"
	"reflect"
	"testing"
)

func newTestFileSet(t *testing.T) (*FileSetEntryWithId, *FileSet) {
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

func newWal(t *testing.T) (*writeaheadlog.Wal, string) {
	walPath := filepath.Join(testDir, t.Name() + "wal")
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

func randomDxPath() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return common.Bytes2Hex(b)
}

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
	if err := checkDxFileEqual(*recovered.fileSetEntry.DxFile, *entry.fileSetEntry.DxFile); err != nil {
		t.Error(err)
	}
	emptyDxPath := randomDxPath()
	_, err = fs.Open(emptyDxPath)
	if err != ErrUnknownFile {
		t.Errorf("not expect error. Expect %v, Got %v", ErrUnknownFile, err)
	}
}

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
	// Open the file now, the file should be deleted
	_, err = fs.Open(dxPath)
	if err != ErrUnknownFile {
		t.Errorf("open a deleted file does not give ErrUnknownFile. got %v", err)
	}
	err = entry.Close()
	if err != nil {
		t.Fatal(err)
	}
	exist = fs.Exists(dxPath)
	if exist {
		t.Errorf("File should have been deleted")
	}
}

func TestFileSet_Rename(t *testing.T) {
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
	err = entry.Close()
	if err != nil {
		t.Fatal(err)
	}

}
