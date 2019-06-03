// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"strconv"
	"testing"
)

func TestPublicFileSystemDebugAPI_CreateRandomFiles(t *testing.T) {
	tests := []struct {
		numFiles int
		missRate float32
		expectTotalSize uint64
		expectHealth uint32
		expectStuckHealth uint32
		expectRedundancy uint32
		expectNumStuckSegments uint32,
	}{
		{1, 0, 1 << 22 * 10 * 1, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 11*1},
		{10, 0, 1 << 22 * 10 * 10, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 11*10},
		{10, 1, 1 << 22 * 10 * 10, 0, 0, 0, 11*10},
		{100, 0, 1 << 22 * 10 * 100, dxdir.DefaultHealth, dxdir.DefaultHealth, 300, 11*100},
	}
	for i, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(i), &AlwaysSuccessContractor{}, newStandardDisrupter())
		goDeepRate, goWideRate, maxDepth, missRate := float32(0.7), float32(0.5), 3, float32(test.missRate)
		err := fs.createRandomFiles(test, goDeepRate, goWideRate, maxDepth, missRate)
		if err != nil {
			t.Fatal(err)
		}
		expectMd := dxdir.Metadata{
			NumFiles: test.numFiles,
			TotalSize: 1 << 22 * 10 * test.numFiles,

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
