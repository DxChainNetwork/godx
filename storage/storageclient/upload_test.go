// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"github.com/pborman/uuid"
)

// Upload test case has many dependencies modules. Now we test each critical function
func testUploadDirectory(t *testing.T) {
	rt := newStorageClientTester(t)

	rt.Client.Start(rt.Backend, rt.Backend)
	defer rt.Client.Close()

	localFilePath := filepath.Join(homeDir(), "uploadtestfiles")

	_, err := os.Stat(localFilePath)
	if os.IsNotExist(err) {
		os.Mkdir(localFilePath, os.ModePerm)
	}

	testUploadFile, err := ioutil.TempFile(localFilePath, "*")

	if _, err := testUploadFile.Write(generateRandomBytes(8)); err != nil {
		t.Fatal(err)
	}

	if err := testUploadFile.Close(); err != nil {
		t.Fatal(err)
	}

	ec, err := erasurecode.New(erasurecode.ECTypeStandard, 2, 3)
	if err != nil {
		t.Fatal(err)
	}

	params := storage.FileUploadParams{
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
func TestDirMetadata(t *testing.T) {
	storage.ENV = storage.EnvTest

	sct := newStorageClientTester(t)
	sc := sct.Client
	defer sc.Close()

	if dir, err := sc.dirMetadata(storage.RootDxPath()); err != nil {
		t.Fatal(err)
	} else if dir.Health != dxfile.CompleteHealthThreshold {
		t.Fatal("root meta health is not 200")
	}
}

func TestDoUpload(t *testing.T) {
	storage.ENV = storage.EnvTest

	sct := newStorageClientTester(t)
	sc := sct.Client
	defer sc.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		sc.uploadOrRepair()
		wg.Done()
	}()

	entry := newFileEntry(t, sct.Client)
	defer func() {
		if err := os.Remove(string(entry.LocalPath())); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(string(entry.FilePath())); err != nil {
			t.Fatal(err)
		}
		if err := entry.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	dxPath := entry.DxPath()
	defer sc.DeleteFile(dxPath)

	table := sc.contractManager.HostHealthMapByID(entry.HostIDs())
	if err := entry.MarkAllUnhealthySegmentsAsStuck(table); err != nil {
		t.Fatal(err)
	}

	if err := sc.fileSystem.InitAndUpdateDirMetadata(dxPath); err != nil {
		t.Fatal("update root metadata failed: ", err)
	}
	<-time.After(1 * time.Second)

	select {
	case sc.uploadHeap.segmentComing <- struct{}{}:
	default:
	}

	var heapLen = -1
	for heapLen != 0 {
		sc.uploadHeap.mu.Lock()
		heapLen = sc.uploadHeap.heap.Len()
		sc.uploadHeap.mu.Unlock()
	}

	if heapLen == 0 {
		sc.tm.Stop()
	}
	wg.Wait()

}

func TestPushFileToSegmentHeap(t *testing.T) {
	storage.ENV = storage.EnvTest

	sct := newStorageClientTester(t)
	defer sct.Client.Close()

	entry := newFileEntry(t, sct.Client)
	defer func() {
		if err := os.Remove(string(entry.LocalPath())); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(string(entry.FilePath())); err != nil {
			t.Fatal(err)
		}
		if err := entry.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	dxPath := entry.DxPath()
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

func TestRequiredContract(t *testing.T) {
	a := 9
	b := 10
	requiredContracts := math.Ceil(float64(a+b) / 2)
	if uint64(requiredContracts) != 10 {
		t.Fatal("not equal ceil value")
	}

}

func TestCreatAndAssignToWorkers(t *testing.T) {
	storage.ENV = storage.EnvTest

	sct := newStorageClientTester(t)
	defer sct.Client.Close()

	entry := newFileEntry(t, sct.Client)
	defer func() {
		if err := os.Remove(string(entry.LocalPath())); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(string(entry.FilePath())); err != nil {
			t.Fatal(err)
		}
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

	unfinishedSegments, _ := sct.Client.createUnfinishedSegments(entry, hosts, targetUnstuckSegments, nilHostHealthInfoTable)
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

func TestRetrieveData(t *testing.T) {
	storage.ENV = storage.EnvTest

	sct := newStorageClientTester(t)
	defer sct.Client.Close()

	entry := newFileEntry(t, sct.Client)
	defer func() {
		if err := os.Remove(string(entry.LocalPath())); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(string(entry.FilePath())); err != nil {
			t.Fatal(err)
		}

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

	unfinishedSegments, _ := sct.Client.createUnfinishedSegments(entry, hosts, targetUnstuckSegments, nilHostHealthInfoTable)
	if len(unfinishedSegments) <= 0 {
		t.Fatal("push heap failed")
	}

	segment := unfinishedSegments[0]

	if err := sct.Client.retrieveLogicalSegmentData(segment); err != nil {
		t.Fatal("retrieveLogicalSegmentData failed", err)
	}

	var segmentBytes []byte
	num := 0
	for _, b := range segment.logicalSegmentData {
		num += len(b)
		segmentBytes = append(segmentBytes, b...)
	}

	var err error
	segment.physicalSegmentData, err = segment.fileEntry.ErasureCode().Encode(segmentBytes)
	if err != nil {
		t.Fatal("ErasureCode encode failed", err)
	}

	if len(segment.physicalSegmentData) < len(segment.sectorSlotsStatus) {
		t.Fatal("not enough physical sectors to match the upload sector slots of the file")
	}

	// Loop through the sectorSlots and encrypt any that are needed
	// If the sector has been used, set physicalSegmentData nil and gc routine will collect this memory
	for i := 0; i < len(segment.sectorSlotsStatus); i++ {
		if segment.sectorSlotsStatus[i] {
			segment.physicalSegmentData[i] = nil
		} else {
			cipherData, err := segment.fileEntry.CipherKey().Encrypt(segment.physicalSegmentData[i])
			if err != nil {
				segment.physicalSegmentData[i] = nil
				t.Fatal("encrypt segment after erasure encode failed", "err", err)
			} else {
				segment.physicalSegmentData[i] = cipherData
			}
		}
	}

	for _, data := range segment.physicalSegmentData {
		if uint64(len(data)) != dxfile.SectorSize {
			t.Fatal("Physical data length is not equal sector size")
		}
	}

}

func TestReadFromLocalFile(t *testing.T) {
	// generate how many MB data
	mb := 9
	filePath, fileSize, fileHash := generateFile(t, homeDir()+"/uploadtestfiles", mb)

	osFile, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}

	buf := newDownloadBuffer(uint64(fileSize), storage.SectorSize)
	sr := io.NewSectionReader(osFile, 0, int64(fileSize))
	_, err = buf.ReadFrom(sr)
	if err != nil {
		t.Fatal(err)
	}

	if remainder := mb % 4; remainder != 0 {
		if fileHash == crypto.Keccak256Hash(buf.buf...) {
			t.Fatal("write and read content is not the same")
		}

		index := mb / 4
		sector := buf.buf[index]
		if len(sector) != int(storage.SectorSize) {
			t.Fatal("completion data length not equal sector size")
		}

		for start := remainder << 20; start < len(sector); start++ {
			if sector[start] != 0 {
				t.Fatal("completion data not equal zero")
			}
		}
	} else {
		if fileHash != crypto.Keccak256Hash(buf.buf...) {
			t.Fatal("write and read content is not the same")
		}
	}

	if err := osFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
}

func generateFile(t *testing.T, localFilePath string, mb int) (string, int, common.Hash) {
	_, err := os.Stat(localFilePath)
	if os.IsNotExist(err) {
		os.Mkdir(localFilePath, os.ModePerm)
	}

	testUploadFile, err := ioutil.TempFile(localFilePath, "*.dat")

	bytes := generateRandomBytes(mb)
	if _, err := testUploadFile.Write(bytes); err != nil {
		t.Fatal(err)
	}

	if err := testUploadFile.Close(); err != nil {
		t.Fatal(err)
	}

	return testUploadFile.Name(), len(bytes), crypto.Keccak256Hash(bytes)
}

func generateRandomBytes(mb int) []byte {
	var bytes []byte
	for i := 0; i < (1 << 20); i++ {
		tmp := make([]byte, mb)
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
