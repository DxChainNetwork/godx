package dxfile

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/rlp"
)

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
		iu := insertUpdate{
			Filename: test.filename,
			Offset:   test.offset,
			Data:     test.data,
		}
		op, err := iu.encodeToWalOp()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		var recoveredUpdate insertUpdate
		if err = rlp.DecodeBytes(op.Data, &recoveredUpdate); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reflect.DeepEqual(recoveredUpdate, iu) {
			t.Errorf("not equal: \n\tExpect %+v\n\tGot %+v", iu, recoveredUpdate)
		}
	}
}

func TestInsertUpdate_Apply(t *testing.T) {
	tests := []insertUpdate{
		{filepath.Join(testDir, t.Name()), 0, randomBytes(10)},
		{filepath.Join(testDir, t.Name()), 10, randomBytes(128)},
		{filepath.Join(testDir, t.Name()), 256, randomBytes(512)},
		{filepath.Join(testDir, t.Name()), 1024, randomBytes(30)},
	}
	for _, test := range tests {
		err := test.apply()
		if err != nil {
			t.Fatalf(err.Error())
		}
	}
	f, _ := os.OpenFile(tests[0].Filename, os.O_RDONLY, 0777)
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

func TestDeleteUpdate_Apply(t *testing.T) {
	filename := filepath.Join(testDir, t.Name())
	f, _ := os.OpenFile(filename, os.O_RDWR, 0600)
	f.Write(randomBytes(1024))
	f.Sync()
	f.Close()

	test := deleteUpdate{filename}
	err := test.apply()
	if err != nil {
		t.Fatalf(err.Error())
	}
	_, err = os.Stat(filename)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("file not deleted")
	}
}

func TestDxFileUpdate_Encode_Decode(t *testing.T) {
	tests := []dxfileUpdate{
		&insertUpdate{filepath.Join(testDir, t.Name()), 0, randomBytes(10)},
		&deleteUpdate{filepath.Join(testDir, t.Name())},
	}
	for i, test := range tests {
		op, err := test.encodeToWalOp()
		if err != nil {
			t.Fatalf(err.Error())
		}
		recovered, err := decodeFromWalOp(op)
		if err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(recovered, test) {
			t.Errorf("%d Not equal\n\tExpect %+v\n\tGot %+v", i, test, recovered)
		}
	}
}
