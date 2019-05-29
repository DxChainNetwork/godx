// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestNewFileSystemEmptyStart test the situation of creating a new file system in
// an empty directory
func TestNewFileSystemEmptyStartClose(t *testing.T) {
	fs := newEmptyTestFileSystem(t, "", nil, make(disrupter))
	fs.disrupter.registerDisruptFunc("InitAndUpdateDirMetadata", makeBlockDisruptFunc(fs.tm.StopChan(),
		func() bool { return false }))
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

// TestFileSystem_UpdatesUnderSameDirectory test the scenario of updating a single file or multiple files
// under different contractor under the same directory.
func TestFileSystem_UpdatesUnderSameDirectory(t *testing.T) {
	// fileSize: 11 segments for erasure params 10 / 30
	fileSize := uint64(1 << 22 * 10 * 10)
	tests := []struct {
		numFiles        int // numFiles is the number of the files under the same directory
		contractor      contractor
		markStuck       bool // flag indicates whether markAllUnhealthyAsStuck called
		rootMetadata    *dxdir.Metadata
		cmpMetadataFunc func(got dxdir.Metadata, expect dxdir.Metadata) error
	}{
		{
			numFiles:   1,
			contractor: &alwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   10,
			contractor: &alwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  10,
				TotalSize: fileSize * 10,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   1,
			contractor: &alwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   1,
			contractor: &alwaysSuccessContractor{},
			markStuck:  false,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   10,
			contractor: &alwaysFailContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  10,
				TotalSize: fileSize * 10,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles: 1,
			contractor: &randomContractor{
				missRate:         0.1,
				onlineRate:       0.9,
				goodForRenewRate: 0.9,
			},
			markStuck: true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles: 10,
			contractor: &randomContractor{
				missRate:         0.1,
				onlineRate:       0.9,
				goodForRenewRate: 0.9,
			},
			markStuck: true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  10,
				TotalSize: fileSize * 10,
				DxPath:    storage.RootDxPath(),
			},
		},
	}
	for index, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(index), test.contractor, make(disrupter))
		test.rootMetadata.RootPath = fs.rootDir
		commonPath := randomDxPath(t, 2)
		ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
		if err != nil {
			t.Fatal(err)
		}
		var health, stuckHealth, numStuckSegments, minRedundancy uint32
		health, stuckHealth, numStuckSegments, minRedundancy = 200, 200, 0, 300
		for fileIndex := 0; fileIndex != test.numFiles; fileIndex++ {
			path, err := commonPath.Join(randomDxPath(t, 1).Path)
			if err != nil {
				t.Fatal(err)
			}
			df := fs.FileSet.NewRandomDxFile(t, path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize)
			if test.markStuck {
				if err = df.MarkAllUnhealthySegmentsAsStuck(fs.contractor.HostHealthMapByID(df.HostIDs())); err != nil {
					t.Fatalf("test %d: cannot markAllUnhealthySegmentsAsStuck: %v", index, err)
				}
			}
			health, stuckHealth, numStuckSegments, minRedundancy = fs.healthParamsUpdate(df, health, stuckHealth, numStuckSegments, minRedundancy)
			if err = df.Close(); err != nil {
				t.Fatal(err)
			}
			par, err := path.Parent()
			if err != nil {
				t.Fatal(err)
			}
			err = fs.InitAndUpdateDirMetadata(par)
			if err != nil {
				t.Fatal(err)
			}
		}
		// wait until updates complete
		fs.waitForExecution(t)

		// Check the metadata of the root Path
		rootPath := storage.RootDxPath()
		test.rootMetadata.DxPath = rootPath

		if !fs.DirSet.Exists(rootPath) {
			t.Fatalf("test %d path %s not exist", index, rootPath.Path)
		}
		dir, err := fs.DirSet.Open(rootPath)
		if err != nil {
			t.Fatalf("test %d cannot open dir path %v: %v", index, rootPath.Path, err)
		}
		md := dir.Metadata()
		if err = checkMetadataSimpleEqual(md, *test.rootMetadata); err != nil {
			t.Fatal(err)
		}
		if md.Health != health {
			t.Errorf("test %d Health unexpected. Expect %v, got %v", index, health, md.Health)
		}
		if md.StuckHealth != stuckHealth {
			t.Errorf("test %d StuckHealth unexpected. Expect %v, got %v", index, stuckHealth, md.StuckHealth)
		}
		if md.NumStuckSegments != numStuckSegments {
			t.Errorf("test %d numStuckSegmetns unexpected. Expect %v, Got %v", index, numStuckSegments, md.NumStuckSegments)
		}
		if md.MinRedundancy != minRedundancy {
			t.Errorf("test %d minRedundancy unexpected. Expect %v, Got %v", index, minRedundancy, md.MinRedundancy)
		}
		if err = dir.Close(); err != nil {
			t.Fatal(err)
		}
		fs.postTestCheck(t, true, true, nil)
	}
}

// healthParamsUpdate calculate and update the health parameters
func (fs *FileSystem) healthParamsUpdate(df *dxfile.FileSetEntryWithID, health, stuckHealth, numStuckSegments, minRedundancy uint32) (uint32, uint32, uint32, uint32) {
	fHealth, fStuckHealth, fNumStuckSegments := df.Health(fs.contractor.HostHealthMapByID(df.HostIDs()))
	fMinRedundancy := df.Redundancy(fs.contractor.HostHealthMapByID(df.HostIDs()))
	if dxfile.CmpHealthPriority(fHealth, health) > 0 {
		health = fHealth
	}
	if dxfile.CmpHealthPriority(fStuckHealth, stuckHealth) > 0 {
		stuckHealth = fStuckHealth
	}
	numStuckSegments += fNumStuckSegments
	if fMinRedundancy < minRedundancy {
		minRedundancy = fMinRedundancy
	}
	return health, stuckHealth, numStuckSegments, minRedundancy
}

// waitForExecution is the helper function that wait for update execution
func (fs *FileSystem) waitForExecution(t *testing.T) {
	c := make(chan struct{})
	// Wait until update complete
	go func() {
		defer close(c)
		for {
			if len(fs.unfinishedUpdates) == 0 {
				// There might be case the child directory completed update while
				// the parent update is not in unfinishedUpdates
				<-time.After(10 * time.Millisecond)
				if len(fs.unfinishedUpdates) == 0 {
					return
				}
				continue
			}
			<-time.After(10 * time.Millisecond)
		}
	}()
	select {
	case <-time.After(time.Second):
		t.Fatal("after one second, update still not completed")
	case <-c:
	}
}

// postTestCheck check the post test status. Checks whether could be closed in 1 seconds,
// whether fileWal should be empty, updateWal is empty, and the rootDir's metadata is as expected
func (fs *FileSystem) postTestCheck(t *testing.T, fileWalShouldEmpty bool, updateWalShouldEmpty bool, md *dxdir.Metadata) {
	c := make(chan struct{})
	go func() {
		err := fs.Close()
		if err != nil {
			t.Fatal(err)
		}
		close(c)
	}()
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Cannot close after certain period")
	}
	rootDir := tempDir(t.Name())
	// check fileWal
	fileWal := filepath.Join(string(rootDir), fileWalName)
	_, txns, err := writeaheadlog.New(fileWal)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) == 0 != fileWalShouldEmpty {
		t.Errorf("fileWal should be empty: %v, but got %v unapplied txns.", fileWalShouldEmpty, len(txns))
	}
	// check updateWal
	updateWal := filepath.Join(string(rootDir), updateWalName)
	_, txns, err = writeaheadlog.New(updateWal)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) == 0 != updateWalShouldEmpty {
		t.Errorf("updateWal should be empty: %v, but got %v unapplied txns.", updateWalShouldEmpty, len(txns))
	}
	// check root dxdir file have the expected metadata
	if md != nil {
		d, err := fs.DirSet.Open(storage.RootDxPath())
		if err != nil {
			t.Fatalf("cannot open the root dxdir: %v", err)
		}
		d.Metadata()
	}
}

// checkMetadataEqual checks whether two metadata are the same.
// Fields not to be checked: time fields
// health fields to be checked is determined by checkHealth boolean
func checkMetadataEqual(got, expect dxdir.Metadata) error {
	if err := checkMetadataSimpleEqual(got, expect); err != nil {
		return err
	}
	if got.Health != expect.Health {
		return fmt.Errorf("health not expected. Expect %d, got %d", expect.Health, got.Health)
	}
	if got.StuckHealth != expect.StuckHealth {
		return fmt.Errorf("stuck health not expected. Expect %d, Got %d", expect.StuckHealth, got.StuckHealth)
	}
	if got.MinRedundancy != expect.MinRedundancy {
		return fmt.Errorf("min redundancy not expected. Expect %d, got %d", expect.MinRedundancy, got.MinRedundancy)
	}
	if got.NumStuckSegments != expect.NumStuckSegments {
		return fmt.Errorf("num stuck segments not expected. Expect %d, got %d", expect.NumStuckSegments, got.NumStuckSegments)
	}
	return nil
}

// checkMetadataSimpleEqual is the simplified version of checkMetadataEqual.
// It only checks for the equality of NumFiles, TotalSize, DxPath, and RootPath
func checkMetadataSimpleEqual(got, expect dxdir.Metadata) error {
	if got.NumFiles != expect.NumFiles {
		return fmt.Errorf("num files not expected. expect %d, got %d", expect.NumFiles, got.NumFiles)
	}
	if got.TotalSize != expect.TotalSize {
		return fmt.Errorf("total size not expected. Expect %d, got %d", expect.TotalSize, got.TotalSize)
	}
	if got.DxPath != expect.DxPath {
		return fmt.Errorf("dxpath not expected. Expect %s, got %s", expect.DxPath.Path, got.DxPath.Path)
	}
	if got.RootPath != expect.RootPath {
		return fmt.Errorf("root path not expected. Expect %s, got %s", expect.RootPath, got.RootPath)
	}
	return nil
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
