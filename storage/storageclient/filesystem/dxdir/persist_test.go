// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxdir

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
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
func tempDir(dirs ...string) storage.SysPath {
	path := filepath.Join(os.TempDir(), "dxfile", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v", path))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return storage.SysPath(path)
}

// newWal create a new wallet used for test
func newWal(t *testing.T) *writeaheadlog.Wal {
	wal, txns, err := writeaheadlog.New(filepath.Join(string(testDir), t.Name()+".wal"))
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

// TestDxDir_EncodeDecode test the rlp encode/decode rule
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

// TestDxDir_SaveLoad test save_load process. Test whether the original data could be recovered
// by save and load
func TestDxDir_SaveLoad(t *testing.T) {
	d := randomDxDir(t)
	err := d.save()
	if err != nil {
		t.Fatal(err)
	}
	newDir, err := load(d.dirFilePath, d.wal)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.metadata, newDir.metadata) {
		t.Errorf("metadata not equal\n\t%+v\n\t%+v", d.metadata, newDir.metadata)
	}
	if d.dirFilePath != newDir.dirFilePath {
		t.Errorf("dirPath not equal\n\t%s\n\t%s", d.dirFilePath, newDir.dirFilePath)
	}
}

// TestDxDir_SaveDeleteLoad test the process of save_delete_load process
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
	_, err = load(d.dirFilePath, d.wal)
	if !os.IsNotExist(err) {
		t.Errorf("file should not exist: %v", err)
	}
}

// randomDxDir is a helper function to create a random DxDir with random metadata
func randomDxDir(t *testing.T) *DxDir {
	dxPath, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	dir := testDir.Join(dxPath)
	if err := os.MkdirAll(string(dir), 0700); err != nil {
		t.Fatal(err)
	}
	wal := newWal(t)
	m := randomMetadata()
	m.DxPath = dxPath
	m.RootPath = testDir
	return &DxDir{
		metadata:    m,
		wal:         wal,
		dirFilePath: testDir.Join(dxPath, DirFileName),
	}
}

// randomUint64 return a random uint64 number
func randomUint64() uint64 {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint64()
}

// readnomUint32 return a random uint32 number
func randomUint32() uint32 {
	rand.Seed(time.Now().UnixNano())
	return rand.Uint32()
}

// randomDxPath creates a random DxPath which is a string of byte slice of length 16
func randomDxPath(depth int) storage.DxPath {
	path := ""
	b := make([]byte, 16)
	for i := 0; i != depth; i++ {
		if len(path) != 0 {
			path += "/"
		}
		_, err := rand.Read(b)
		if err != nil {
			panic(err)
		}
		path += common.Bytes2Hex(b)
	}
	dp, _ := storage.NewDxPath(path)
	return dp
}

// randomMetadata creates a total randomed metadata. Note that the DxFile field is not generated
// since this field is not updated in the ApplyUpdate functions
func randomMetadata() *Metadata {
	return &Metadata{
		NumFiles:            randomUint64(),
		TotalSize:           randomUint64(),
		Health:              randomUint32(),
		StuckHealth:         randomUint32(),
		MinRedundancy:       randomUint32(),
		TimeLastHealthCheck: randomUint64(),
		TimeModify:          randomUint64(),
		NumStuckSegments:    randomUint32(),
	}
}
