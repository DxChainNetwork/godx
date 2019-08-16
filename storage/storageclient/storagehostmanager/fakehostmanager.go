// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"fmt"
	"os"

	"github.com/DxChainNetwork/godx/storage"
)

const fakeHostManagerPersist = "testdata/fakehmpersist"

// NewFakeHostManager create a fake StorageHostManager object used for testing purpose
func NewFakeHostManager() (*StorageHostManager, error) {
	// clear the previously saved data first
	clearOldData()

	// create and initialize new fake storage host  manager
	fhm := New(fakeHostManagerPersist)
	if err := fhm.Start(&storage.FakeClientBackend{}); err != nil {
		return nil, fmt.Errorf("error start storage host manager: %s", err.Error())
	}
	return fhm, nil
}

// clearOldData will clear all data under the fakempersist directory
func clearOldData() {
	_ = os.RemoveAll("testdata/fakehmpersist/")
}
