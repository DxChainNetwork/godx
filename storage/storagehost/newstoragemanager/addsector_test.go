// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
	"testing"
)

func TestAddSector(t *testing.T) {
	sm := newTestStorageManager(t, "", newDisrupter())
	path := randomFolderPath(t, "")
	size := uint64(1 << 25)
	if err := sm.addStorageFolder(path, size); err != nil {
		t.Fatal(err)
	}
	// Create the sector
	data := randomBytes(storage.SectorSize)
	root := merkle.Root(data)
	if err := sm.addSector(root, data); err != nil {
		t.Fatal(err)
	}
}

func randomBytes(size uint64) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
