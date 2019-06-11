// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"encoding/binary"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/pborman/uuid"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// Upload test case has many dependencies modules. Now we test each critical function
func testUploadDirectory(t *testing.T) {
	rt := newStorageClientTester(t)
	server := newTestServer()

	rt.Client.Start(rt.Backend, server, rt.Backend)
	defer rt.Client.Close()

	localFilePath := filepath.Join(homeDir(), "uploadtestfiles")

	_, err := os.Stat(localFilePath)
	if os.IsNotExist(err) {
		os.Mkdir(localFilePath, os.ModePerm)
	}

	testUploadFile, err := ioutil.TempFile(localFilePath, "*")

	if _, err := testUploadFile.Write(generateRandomBytes()); err != nil {
		t.Fatal(err)
	}

	if err := testUploadFile.Close(); err != nil {
		t.Fatal(err)
	}

	ec, err := erasurecode.New(erasurecode.ECTypeStandard, 2, 3)
	if err != nil {
		t.Fatal(err)
	}

	params := FileUploadParams{
		Source:      testUploadFile.Name(),
		DxPath:      storage.DxPath{Path: uuid.New()},
		ErasureCode: ec,
	}

	err = rt.Client.Upload(params)
	if err == nil {
		t.Fatal("expected Upload to fail with empty directory as source")
	}
}

/***************** Upload Business Logic Test Case For Each Critical Function ***********************/
func TestPushDirToSegmentHeap(t *testing.T) {

}

func TestCreateUnfinishedSegments(t *testing.T) {

}

func TestNextUploadSegment(t *testing.T) {

}

func TestPreProcessUploadSegment(t *testing.T) {

}

func TestCleanupUploadSegment(t *testing.T) {

}

func TestReadFromLocalFile(t *testing.T) {
	filePath, fileSize, fileHash := generateFile(t)

	osFile, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}

	buf := NewDownloadBuffer(uint64(fileSize), 1<<22)
	sr := io.NewSectionReader(osFile, 0, int64(fileSize))
	_, err = buf.ReadFrom(sr)
	if err != nil {
		t.Fatal(err)
	}

	if fileHash != crypto.Keccak256Hash(buf.buf...) {
		t.Fatal("write and read content is not the same")
	}

	if err := osFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
}

func generateFile(t *testing.T) (string, int, common.Hash) {
	localFilePath := filepath.Join(homeDir(), "uploadtestfiles")

	_, err := os.Stat(localFilePath)
	if os.IsNotExist(err) {
		os.Mkdir(localFilePath, os.ModePerm)
	}

	testUploadFile, err := ioutil.TempFile(localFilePath, "*")

	bytes := generateRandomBytes()
	if _, err := testUploadFile.Write(bytes); err != nil {
		t.Fatal(err)
	}

	if err := testUploadFile.Close(); err != nil {
		t.Fatal(err)
	}

	return testUploadFile.Name(), len(bytes), crypto.Keccak256Hash(bytes)
}

func generateRandomBytes() []byte {
	var bytes []byte
	for i := 0; i < (1 << 20); i++ {
		tmp := make([]byte, 8)
		n := uint64(rand.Int63())
		binary.BigEndian.PutUint64(tmp, n)
		bytes = append(bytes, tmp...)
	}
	return bytes
}
