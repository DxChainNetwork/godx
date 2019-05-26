// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"testing"
	"time"
)

// TestNewFileSystemEmptyStart test the situation of creating a new file system in
// an empty directory
func TestNewFileSystemEmptyStartClose(t *testing.T) {
	fs := newEmptyTestFileSystem(t, nil, make(disrupter))
	if fs.rootDir != tempDir(t.Name()) {
		t.Errorf("fs.rootDir not expected. Expect %v, Got %v", tempDir(t.Name()), fs.rootDir)
	}
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
