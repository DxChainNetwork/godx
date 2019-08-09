// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/rlp"
)

var testDir = tempDir("storage")

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

func TestInsertUpdate_EncodeToWalOp(t *testing.T) {
	tests := []struct {
		filename string
		offset   uint64
		data     []byte
	}{
		{testDir, rand.Uint64(), []byte{}},
		{testDir, rand.Uint64(), randomBytes(4096)},
	}
	for _, test := range tests {
		iu := InsertUpdate{
			FileName: test.filename,
			Offset:   test.offset,
			Data:     test.data,
		}
		op, err := iu.EncodeToWalOp()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		var recoveredUpdate InsertUpdate
		if err = rlp.DecodeBytes(op.Data, &recoveredUpdate); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reflect.DeepEqual(recoveredUpdate, iu) {
			t.Errorf("not equal: \n\tExpect %+v\n\tGot %+v", iu, recoveredUpdate)
		}
	}
}

// TestInsertUpdate_Apply test insertUpdate.apply
func TestInsertUpdate_Apply(t *testing.T) {
	tests := []InsertUpdate{
		{filepath.Join(testDir, t.Name()), 0, randomBytes(10)},
		{filepath.Join(testDir, t.Name()), 10, randomBytes(128)},
		{filepath.Join(testDir, t.Name()), 256, randomBytes(512)},
		{filepath.Join(testDir, t.Name()), 1024, randomBytes(30)},
	}
	for _, test := range tests {
		err := test.Apply()
		if err != nil {
			t.Fatalf(err.Error())
		}
	}
	f, _ := os.OpenFile(tests[0].FileName, os.O_RDONLY, 0777)
	for _, test := range tests {
		b := make([]byte, len(test.Data))
		_, err := f.ReadAt(b, int64(test.Offset))
		if err != nil {
			t.Fatalf(err.Error())
		}
		if !bytes.Equal(test.Data, b) {
			t.Errorf("Bytes not equal: \n\tExpect %x\n\tGot %x", test.Data, b)
		}
	}
}

// TestDeleteUpdate_Apply test deleteUpdate.apply
func TestDeleteUpdate_Apply(t *testing.T) {
	filename := filepath.Join(testDir, t.Name())
	f, _ := os.OpenFile(filename, os.O_RDWR, 0600)
	f.Write(randomBytes(1024))
	f.Sync()
	f.Close()

	test := DeleteUpdate{filename}
	err := test.Apply()
	if err != nil {
		t.Fatalf(err.Error())
	}
	_, err = os.Stat(filename)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("file not deleted")
	}
}

// TestDxFileUpdate_Encode_Decode test the process of encoding the dxFileUpdate to operation and
// operation to dxFileUpdate
func TestDxFileUpdate_Encode_Decode(t *testing.T) {
	tests := []FileUpdate{
		&InsertUpdate{filepath.Join(testDir, t.Name()), 0, randomBytes(10)},
		&DeleteUpdate{filepath.Join(testDir, t.Name())},
	}
	for i, test := range tests {
		op, err := test.EncodeToWalOp()
		if err != nil {
			t.Fatalf(err.Error())
		}
		recovered, err := OpToUpdate(op)
		if err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(recovered, test) {
			t.Errorf("%d Not equal\n\tExpect %+v\n\tGot %+v", i, test, recovered)
		}
	}
}

// randomBytes create a random bytes of size input num
func randomBytes(num int) []byte {
	b := make([]byte, num)
	rand.Read(b)
	return b
}
