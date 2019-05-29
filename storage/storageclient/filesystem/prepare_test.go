// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

// prepare_test.go contain functions that is used for building up the test environment.

import (
	"fmt"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) storage.SysPath {
	path := filepath.Join(os.TempDir(), "filesystem", filepath.Join(dirs...))
	if err := os.RemoveAll(path); err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v: %v", path, err))
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		panic(fmt.Sprintf("cannot create directory %v: %v", path, err))
	}
	return storage.SysPath(path)
}

// newEmptyTestFileSystem creates an empty file system used for testing
func newEmptyTestFileSystem(t *testing.T, extraNaming string, contractor contractor, disrupter disrupter) *FileSystem {
	var rootDir storage.SysPath
	if len(extraNaming) == 0 {
		rootDir = tempDir(t.Name())
	} else {
		rootDir = tempDir(t.Name(), extraNaming)
	}
	fs := newFileSystem(rootDir, contractor, disrupter)
	err := fs.Start()
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

// alwaysSuccessContractor is the contractor that always return good condition for all host keys
type alwaysSuccessContractor struct{}

// HostHealthMapByID always return good condition
func (c *alwaysSuccessContractor) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      false,
			GoodForRenew: true,
		}
	}
	return table
}

// alwaysSuccessContractor is the contractor that always return wrong condition for all host keys
type alwaysFailContractor struct{}

// HostHealthMapByID always return bad condition
func (c *alwaysFailContractor) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		table[id] = storage.HostHealthInfo{
			Offline:      true,
			GoodForRenew: false,
		}
	}
	return table
}

// randomContractor is the contractor that return condition is random possibility
// rate is the possibility between 0 and 1 for specified conditions
type randomContractor struct {
	missRate         float32 // missRate is the rate that the input id is not in the table
	onlineRate       float32 // onlineRate is the rate the the id is online
	goodForRenewRate float32 // goodForRenewRate is the rate of goodForRenew

	missed map[enode.ID]struct{}       // missed node should be forever missed
	table  storage.HostHealthInfoTable // If previously stored the table, do not random again
	once   sync.Once                   // Only initialize the HostHealthInfoTable once
	lock   sync.Mutex                  // lock is the mutex to protect the table field
}

func (c *randomContractor) HostHealthMapByID(ids []enode.ID) storage.HostHealthInfoTable {
	c.once.Do(func() {
		c.table = make(storage.HostHealthInfoTable)
		c.missed = make(map[enode.ID]struct{})
	})
	rand.Seed(time.Now().UnixNano())
	c.lock.Lock()
	defer c.lock.Unlock()
	table := make(storage.HostHealthInfoTable)
	for _, id := range ids {
		// previously missed id will be forever missed
		if _, exist := c.missed[id]; exist {
			continue
		}
		if _, exist := c.table[id]; exist {
			table[id] = c.table[id]
			continue
		}
		num := rand.Float32()
		if num < c.missRate {
			c.missed[id] = struct{}{}
			continue
		}
		num = rand.Float32()
		var offline, goodForRenew bool
		if num >= c.onlineRate {
			offline = true
		}
		num = rand.Float32()
		if num < c.goodForRenewRate {
			goodForRenew = true
		}
		c.table[id] = storage.HostHealthInfo{
			Offline:      offline,
			GoodForRenew: goodForRenew,
		}
		table[id] = c.table[id]
	}
	return table
}
