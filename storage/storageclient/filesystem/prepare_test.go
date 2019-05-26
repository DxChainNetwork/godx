// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

// prepare_test.go contain functions that is used for building up the test environment.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DxChainNetwork/godx/storage"
)

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

// newEmptyTestFileSystem creates an empty file system used for testing
func newEmptyTestFileSystem(t *testing.T, contractor contractor, disrupter disrupter) *FileSystem {
	rootDir := tempDir(t.Name())

	fs := newFileSystem(rootDir, contractor, disrupter)
	err := fs.Start()
	if err != nil {
		t.Fatal(err)
	}
	return fs
}
