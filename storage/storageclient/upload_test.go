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
	"strings"
	"sync"
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
func TestPushFileToSegmentHeap(t *testing.T) {
	sct := newStorageClientTester(t)

	entry := newFileEntry(t, sct.Client)
	dxPath := entry.DxPath()
	if err := entry.Close(); err != nil {
		t.Fatal(err)
	}

	hosts := map[string]struct{}{
		"111111": {},
		"222222": {},
		"333333": {},
		"444444": {},
		"555555": {},
	}

	sct.Client.pushDirOrFileToSegmentHeap(dxPath, false, hosts, targetUnstuckSegments)

	if sct.Client.uploadHeap.len() > 0 {
		t.Fatal("not enough workers, can't push segment upload heap")
	}
}

func TestCreatAndAssignToWorkers(t *testing.T) {
	sct := newStorageClientTester(t)
	entry := newFileEntry(t, sct.Client)
	defer func() {
		if err := entry.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	hosts := map[string]struct{}{
		"111111": {},
		"222222": {},
		"333333": {},
		"444444": {},
		"555555": {},
	}

	mockAddWorkers(3, sct.Client)

	nilHostHealthInfoTable := make(storage.HostHealthInfoTable)

	unfinishedSegments := sct.Client.createUnfinishedSegments(entry, hosts, targetUnstuckSegments, nilHostHealthInfoTable)
	if len(unfinishedSegments) <= 0 {
		t.Fatal("push heap failed")
	}

	var wg sync.WaitGroup
	for _, v := range sct.Client.workerPool {
		wg.Add(1)
		go func(w *worker) {
			<-w.uploadChan
			wg.Done()
		}(v)
	}

	sct.Client.dispatchSegment(unfinishedSegments[0])
	wg.Wait()
}

func TestReadFromLocalFile(t *testing.T) {
	filePath, fileSize, fileHash := generateFile(t, homeDir()+"/uploadtestfiles")

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

func generateFile(t *testing.T, localFilePath string) (string, int, common.Hash) {
	_, err := os.Stat(localFilePath)
	if os.IsNotExist(err) {
		os.Mkdir(localFilePath, os.ModePerm)
	}

	testUploadFile, err := ioutil.TempFile(localFilePath, "*"+storage.DxFileExt)

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

func getFileNameFromPath(path string) string {
	s := filepath.ToSlash(path)
	_, file := filepath.Split(s)
	return file
}

func getDxPathFromPath(root string, path string) storage.DxPath {
	if strings.HasPrefix(path, root) {
		s := strings.TrimPrefix(path, root)
		if strings.HasPrefix(s, "/") {
			p := strings.TrimSuffix(strings.TrimPrefix(s, "/"), storage.DxFileExt)
			return storage.DxPath{Path: p}
		}
		return storage.DxPath{Path: strings.TrimSuffix(s, storage.DxFileExt)}
	}
	return storage.RootDxPath()
}
