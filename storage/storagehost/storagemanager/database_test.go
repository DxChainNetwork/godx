// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"bytes"
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"strconv"
	"testing"
)

// TestDatabase_getSectorSalt test database.getOrCreateSectorSalt
func TestDatabase_getSectorSalt(t *testing.T) {
	db, err := openDB(tempDir(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	salt, err := db.getOrCreateSectorSalt()
	if err != nil {
		t.Fatal(err)
	}
	salt2, err := db.getOrCreateSectorSalt()
	if err != nil {
		t.Fatal(err)
	}
	if salt != salt2 {
		t.Errorf("salt not equal. Prev %x, Later %x", salt, salt2)
	}
	saltFromDB, err := db.lvl.Get([]byte(sectorSaltKey), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(salt[:], saltFromDB) {
		t.Errorf("salt from db not equal. Got %x, Expect %x", saltFromDB, salt)
	}
}

// TestDatabase_PutGetStorageFolder test the save-load process for the storage folder
func TestDatabase_PutGetStorageFolder(t *testing.T) {
	db := newTestDatabase(t, "")
	sf := randomStorageFolder(t, "", db)
	if err := db.saveStorageFolder(sf); err != nil {
		t.Fatal(err)
	}
	recoveredSF, err := db.loadStorageFolder(sf.path)
	if err != nil {
		t.Fatal(err)
	}
	checkStorageFolderEqual(t, "", recoveredSF, sf)
}

func TestDatabase_PutGetStorageFolderByID(t *testing.T) {
	db := newTestDatabase(t, "")
	sf := randomStorageFolder(t, "", db)
	if err := db.saveStorageFolder(sf); err != nil {
		t.Fatal(err)
	}
	recoveredSF, err := db.loadStorageFolderByID(sf.id)
	if err != nil {
		t.Fatal(err)
	}
	checkStorageFolderEqual(t, "", recoveredSF, sf)
}

// TestDatabase_PutLoadAllStorageFolder test the save-loadAll process for the storage folder
func TestDatabase_PutLoadAllStorageFolder(t *testing.T) {
	db := newTestDatabase(t, "")
	// create 10 random folders
	numFolders := 10
	folders := make([]*storageFolder, 0, numFolders)
	for i := 0; i != numFolders; i++ {
		sf := randomStorageFolder(t, "", db)
		if err := db.saveStorageFolder(sf); err != nil {
			t.Fatal(err)
		}
		folders = append(folders, sf)
	}

	// load from db and compare
	recovered, err := db.loadAllStorageFolders()
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != len(folders) {
		t.Fatalf("loaded folders not equal in size. Expect %v, Got %v", len(folders), len(recovered))
	}
	// Loop over the folders to check whether all entries exists.
	for i, folder := range folders {
		recoveredSF, exist := recovered[folder.path]
		if !exist {
			t.Errorf("storage folder with id not exist in recovered")
		}
		checkStorageFolderEqual(t, strconv.Itoa(i), recoveredSF, folder)
	}
}

// checkStorageFolderEqual checks equality of two storageFolder. Only persist fields are checked.
// If error happened, directory error the testing.T
func checkStorageFolderEqual(t *testing.T, testName string, got, want *storageFolder) {
	if got.id != want.id {
		t.Errorf("Test %v %v: expect id %v, got %v", t.Name(), testName, want.id, got.id)
	}
	if got.path != want.path {
		t.Errorf("Test %v %v: expect Path %v, got %v", t.Name(), testName, want.path, got.path)
	}
	if got.numSectors != want.numSectors {
		t.Errorf("Test %v %v: expect NumSectors %v, got %v", t.Name(), testName, want.numSectors, got.numSectors)
	}
	if len(got.usage) != len(want.usage) {
		t.Fatalf("Test %v %v: Usage not having the same length. expect %v, got %v", t.Name(), testName,
			want.usage, got.usage)
	}
	for i := range got.usage {
		if got.usage[i] != want.usage[i] {
			t.Errorf("Test %v %v: Usage[%d] not equal. expect %v, got %v", t.Name(), testName, i, want.usage[i], got.usage[i])
		}
	}
	if got.storedSectors != want.storedSectors {
		t.Errorf("Test %v %v: expect storedSectors %v, got %v", t.Name(), testName, want.storedSectors, got.storedSectors)
	}
}

func randomStorageFolder(t *testing.T, extra string, db *database) (sf *storageFolder) {
	path := tempDir(t.Name(), extra, randomString(16))
	id, err := db.randomFolderID()
	if err != nil {
		t.Fatal(err)
	}
	sf = &storageFolder{
		id:            id,
		path:          path,
		usage:         []bitVector{1 << 32},
		numSectors:    1,
		storedSectors: randomUint64(),
	}
	return sf
}

func randomString(size int) (s string) {
	b := make([]byte, size)
	rand.Read(b)
	s = common.Bytes2Hex(b)
	return
}
