// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPublicFileSystemDebugAPI_CreateRandomFiles(t *testing.T) {
	tests := []struct {
		numFiles               uint64
		missRate               float32
		expectTotalSize        uint64
		expectHealth           uint32
		expectStuckHealth      uint32
		expectRedundancy       uint32
		expectNumStuckSegments uint32
	}{
		{1, 0, 1 << 22 * 10 * 10 * 1, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 0},
		{10, 0, 1 << 22 * 10 * 10 * 10, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 0},
		{10, 1, 1 << 22 * 10 * 10 * 10, 200, 0, 0, 11 * 10},
		{1000, 0, 1 << 22 * 10 * 10 * 1000, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 0},
	}
	if testing.Short() {
		tests = tests[:len(tests)-1]
	}
	for i, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), &AlwaysSuccessContractManager{}, newStandardDisrupter())
		goDeepRate, goWideRate, maxDepth, missRate := float32(0.7), float32(0.5), 3, float32(test.missRate)
		err := fs.createRandomFiles(int(test.numFiles), goDeepRate, goWideRate, maxDepth, missRate)
		if err != nil {
			t.Fatal(err)
		}
		expectMd := dxdir.Metadata{
			NumFiles:         test.numFiles,
			TotalSize:        test.expectTotalSize,
			Health:           test.expectHealth,
			StuckHealth:      test.expectStuckHealth,
			MinRedundancy:    test.expectRedundancy,
			NumStuckSegments: test.expectNumStuckSegments,
			DxPath:           storage.RootDxPath(),
			RootPath:         fs.fileRootDir,
		}
		if err := fs.waitForUpdatesComplete(20 * time.Second); err != nil {
			t.Fatal(err)
		}
		// Check the metadata of the root directory
		dxdir, err := fs.dirSet.Open(storage.RootDxPath())
		if err != nil {
			t.Fatal(err)
		}
		md := dxdir.Metadata()
		if err = checkMetadataEqual(md, expectMd); err != nil {
			t.Errorf("Test %d: metadata not equal:%v", i, err)
		}
	}
}

// TestPublicFileSystemAPI_FileList test the functionality of PublicFileSystemAPI.FileList
func TestPublicFileSystemAPI_FileList(t *testing.T) {
	tests := []int{1, 10, 100}
	if testing.Short() {
		tests = tests[:1]
	}
	for i, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), &AlwaysSuccessContractManager{}, newStandardDisrupter())
		api := NewPublicFileSystemAPI(fs)
		// create random files
		goDeepRate, goWideRate, maxDepth, missRate := float32(0.7), float32(0.5), 3, float32(0.1)
		err := fs.createRandomFiles(test, goDeepRate, goWideRate, maxDepth, missRate)
		if err != nil {
			t.Fatal(err)
		}
		res := api.FileList()
		if len(res) != test {
			t.Errorf("Test %d: size of file brief info unexpected. Expect %v, Got %v", i, test, len(res))
		}
	}
}

// TestPublicFileSystemAPI_Rename test the rename functionality.
// The rename function should also update the metadata in all related directories
func TestPublicFileSystemAPI_Rename(t *testing.T) {
	fs := newEmptyTestFileSystem(t, "", &AlwaysSuccessContractManager{}, newStandardDisrupter())
	api := NewPublicFileSystemAPI(fs)
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	prevPath, newPath := randomDxPath(t, 3), randomDxPath(t, 3)
	df, err := fs.fileSet.NewRandomDxFile(prevPath, 10, 30, erasurecode.ECTypeStandard, ck, 1<<22*100, 0)
	if err != nil {
		t.Fatal(err)
	}
	table := fs.contractManager.HostHealthMapByID(df.HostIDs())
	health, stuckHealth, numStuckSegments := df.Health(table)
	redundancy := df.Redundancy(table)
	if err = df.Close(); err != nil {
		t.Fatal(err)
	}

	res := api.Rename(prevPath.Path, newPath.Path)
	if !strings.Contains(res, "File") || !strings.Contains(res, "renamed to") {
		t.Fatalf("unexpected response message: %v", res)
	}
	if err := fs.waitForUpdatesComplete(1 * time.Second); err != nil {
		t.Fatal(err)
	}
	// three dxdirs to check: 1. old path dxdir 2. new path dxdir 3. root dxdir
	expectMd := dxdir.Metadata{
		NumFiles:         1,
		TotalSize:        1 << 22 * 100,
		Health:           health,
		StuckHealth:      stuckHealth,
		MinRedundancy:    redundancy,
		NumStuckSegments: numStuckSegments,
		RootPath:         fs.fileRootDir,
	}
	emptyMd := dxdir.Metadata{
		NumFiles:         0,
		TotalSize:        0,
		Health:           dxdir.DefaultHealth,
		StuckHealth:      dxdir.DefaultHealth,
		MinRedundancy:    math.MaxUint32,
		NumStuckSegments: 0,
		RootPath:         fs.fileRootDir,
	}
	// expected root metadata
	expectRootMd := expectMd
	expectRootMd.DxPath = storage.RootDxPath()
	// expected dir metadata of new path
	newDirPath, err := newPath.Parent()
	if err != nil {
		t.Fatal(err)
	}
	expectNewDirMd := expectMd
	expectNewDirMd.DxPath = newDirPath
	// expected dir metadata of prev path
	prevDirPath, err := prevPath.Parent()
	if err != nil {
		t.Fatal(err)
	}
	expectPrevDirMd := emptyMd
	expectPrevDirMd.DxPath = prevDirPath

	// Check the results
	if err = fs.checkDxDirMetadata(prevDirPath, expectPrevDirMd); err != nil {
		t.Errorf("previous directory metadata not expected: %v", err)
	}
	if err = fs.checkDxDirMetadata(storage.RootDxPath(), expectRootMd); err != nil {
		t.Errorf("root directory metadata not expected: %v", err)
	}
	if err = fs.checkDxDirMetadata(newDirPath, expectNewDirMd); err != nil {
		t.Errorf("new directory metadata not expected: %v", err)
	}
}

// TestPublicFileSystemAPI_Rename test the rename functionality.
// The rename function should also update the metadata in all related directories
func TestPublicFileSystemAPI_Delete(t *testing.T) {
	fs := newEmptyTestFileSystem(t, "", &AlwaysSuccessContractManager{}, newStandardDisrupter())
	api := NewPublicFileSystemAPI(fs)
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	path := randomDxPath(t, 3)
	df, err := fs.fileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, 1<<22*100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = df.Close(); err != nil {
		t.Fatal(err)
	}
	// delete the file
	res := api.Delete(path.Path)
	if !strings.Contains(res, "File") || !strings.Contains(res, "deleted") {
		t.Fatalf("unexpected response message: %v", res)
	}
	if err := fs.waitForUpdatesComplete(1 * time.Second); err != nil {
		t.Fatal(err)
	}
	// check for metadata: 1. the directory the deleted file resides 2. the root directory
	emptyMd := dxdir.Metadata{
		NumFiles:         0,
		TotalSize:        0,
		Health:           dxdir.DefaultHealth,
		StuckHealth:      dxdir.DefaultHealth,
		MinRedundancy:    math.MaxUint32,
		NumStuckSegments: 0,
		RootPath:         fs.fileRootDir,
	}
	parDir, err := path.Parent()
	if err != nil {
		t.Fatal(err)
	}
	expectParDirMd := emptyMd
	expectParDirMd.DxPath = parDir

	expectRootDirMd := emptyMd
	expectRootDirMd.DxPath = storage.RootDxPath()

	if err = fs.checkDxDirMetadata(storage.RootDxPath(), expectRootDirMd); err != nil {
		t.Errorf("root directory metadata not expected: %v", err)
	}
	if err = fs.checkDxDirMetadata(parDir, expectParDirMd); err != nil {
		t.Errorf("parent directory metadata not expected: %v", err)
	}
}

// checkDxDirMetadata checks whether the dxdir with path has the expected metadata
func (fs *FileSystem) checkDxDirMetadata(path storage.DxPath, expectMd dxdir.Metadata) error {
	dir, err := fs.dirSet.Open(path)
	if err != nil {
		return err
	}
	got := dir.Metadata()
	if err = checkMetadataEqual(got, expectMd); err != nil {
		return err
	}
	return nil
}
