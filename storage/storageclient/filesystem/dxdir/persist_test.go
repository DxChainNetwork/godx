package dxdir

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

var testDir = tempDir("dxdir")

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "dxfile", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

func newWal(t *testing.T) *writeaheadlog.Wal {
	wal, txns, err := writeaheadlog.New(filepath.Join(testDir, t.Name() + ".wal"))
	if err != nil {
		panic(err)
	}
	for _, txn := range txns {
		err := txn.Release()
		if err != nil {
			panic(err)
		}
	}
	return wal
}

func TestDxDir_EncodeDecode(t *testing.T) {
	d := randomDxDir(t)
	data, err := rlp.EncodeToBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	var newDir *DxDir
	err = rlp.DecodeBytes(data, &newDir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.metadata, newDir.metadata) {
		t.Errorf("metadata not equal\n\t%+v\n\t%+v", d.metadata, newDir.metadata)
	}
}

func TestDxDir_SaveLoad(t *testing.T) {
	d := randomDxDir(t)
	err := d.save()
	if err != nil {
		t.Fatal(err)
	}
	newDir, err := load(d.dirPath, d.wal)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.metadata, newDir.metadata) {
		t.Errorf("metadata not equal\n\t%+v\n\t%+v", d.metadata, newDir.metadata)
	}
	if d.dirPath != newDir.dirPath {
		t.Errorf("dirPath not equal\n\t%s\n\t%s", d.dirPath, newDir.dirPath)
	}
}

func TestDxDir_SaveDeleteLoad(t *testing.T) {
	d := randomDxDir(t)
	err := d.save()
	if err != nil {
		t.Fatal(err)
	}
	err = d.delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = load(d.dirPath, d.wal)
	if !os.IsNotExist(err) {
		t.Errorf("file should not exist: %v", err)
	}
}

func randomDxDir(t *testing.T) *DxDir {
	dxPath := t.Name()
	dirPath := filepath.Join(testDir, dxPath)
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		t.Fatal(err)
	}
	wal := newWal(t)
	m := &Metadata {
		NumFiles:randomUint64(),
		TotalSize:randomUint64(),
		Health:randomUint32(),
		StuckHealth:randomUint32(),
		MinRedundancy:randomUint32(),
		TimeLastHealthCheck:randomUint64(),
		TimeModify:randomUint64(),
		NumStuckSegments:randomUint64(),
		DxPath:dxPath,
	}
	return &DxDir {
		metadata: m,
		wal: wal,
		dirPath: dirPath,
	}
}

func randomUint64() uint64 {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint64()
}

func randomUint32() uint32 {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint32()
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
