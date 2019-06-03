// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"crypto/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// TestNewFileSystemEmptyStart test the situation of creating a new file system in
// an empty directory
func TestNewFileSystemEmptyStartClose(t *testing.T) {
	fs := newEmptyTestFileSystem(t, "", nil, newStandardDisrupter())
	fs.disrupter.registerDisruptFunc("InitAndUpdateDirMetadata", makeBlockDisruptFunc(fs.tm.StopChan(),
		func() bool { return false }))
	if len(fs.unfinishedUpdates) != 0 {
		t.Errorf("empty start should have 0 unfinished updates. Got %v", len(fs.unfinishedUpdates))
	}
	c := make(chan error)
	go func(c chan error) {
		c <- fs.Close()
	}(c)
	select {
	case err := <-c:
		if err != nil {
			t.Errorf("close return an error")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("close not immediate shutdown")
	}
}

func randomDxPath(t *testing.T, depth int) storage.DxPath {
	var s string
	for i := 0; i != depth; i++ {
		var b [16]byte
		_, err := rand.Read(b[:])
		if err != nil {
			t.Fatal(err)
		}
		s = filepath.Join(s, common.Bytes2Hex(b[:]))
	}
	path, err := storage.NewDxPath(s)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
