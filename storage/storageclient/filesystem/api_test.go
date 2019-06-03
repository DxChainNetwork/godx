// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"strconv"
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
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), &AlwaysSuccessContractor{}, newStandardDisrupter())
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
			RootPath:         fs.rootDir,
		}
		if err := fs.waitForUpdatesComplete(20 * time.Second); err != nil {
			t.Fatal(err)
		}
		// Check the metadata of the root directory
		dxdir, err := fs.DirSet.Open(storage.RootDxPath())
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
	for i, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), &AlwaysSuccessContractor{}, newStandardDisrupter())
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
