// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// TestFileSystem_UpdatesUnderSameDirectory test the scenario of updating a single file or multiple files
// under different contractor under the same directory.
func TestFileSystem_UpdatesUnderSameDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped for short tests")
	}
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
			contractor: &AlwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   10,
			contractor: &AlwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  10,
				TotalSize: fileSize * 10,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   1,
			contractor: &AlwaysSuccessContractor{},
			markStuck:  true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  1,
				TotalSize: fileSize,
				DxPath:    storage.RootDxPath(),
			},
		},
		{
			numFiles:   1,
			contractor: &AlwaysSuccessContractor{},
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
			numFiles: 100,
			contractor: &randomContractor{
				missRate:         0.1,
				onlineRate:       0.9,
				goodForRenewRate: 0.9,
			},
			markStuck: true,
			rootMetadata: &dxdir.Metadata{
				NumFiles:  100,
				TotalSize: fileSize * 100,
				DxPath:    storage.RootDxPath(),
			},
		},
	}
	for index, test := range tests {
		fs := newEmptyTestFileSystem(t, strconv.Itoa(index), test.contractor, make(standardDisrupter))
		test.rootMetadata.RootPath = fs.rootDir
		commonPath := randomDxPath(t, 2)
		ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
		if err != nil {
			t.Fatal(err)
		}
		var healthLock sync.Mutex
		var health, stuckHealth, numStuckSegments, minRedundancy uint32
		health, stuckHealth, numStuckSegments, minRedundancy = 200, 200, 0, 300
		wg := sync.WaitGroup{}
		for fileIndex := 0; fileIndex != test.numFiles; fileIndex++ {
			// multiple goroutine updating at the same time
			wg.Add(1)
			go func() {
				path, err := commonPath.Join(randomDxPath(t, 1).Path)
				if err != nil {
					t.Fatal(err)
				}
				df, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
				if err != nil {
					t.Fatal(err)
				}
				if test.markStuck {
					if err = df.MarkAllUnhealthySegmentsAsStuck(fs.contractor.HostHealthMapByID(df.HostIDs())); err != nil {
						t.Fatalf("test %d: cannot markAllUnhealthySegmentsAsStuck: %v", index, err)
					}
				}
				healthLock.Lock()
				health, stuckHealth, numStuckSegments, minRedundancy = fs.healthParamsUpdate(df, health, stuckHealth, numStuckSegments, minRedundancy)
				healthLock.Unlock()
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
				wg.Done()
			}()
		}
		wg.Wait()
		// wait until updates complete
		err = fs.waitForUpdatesComplete()
		if err != nil {
			t.Fatal(err)
		}

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

// TestFileSystem_RedoProcess test the process of redo process. That is, when updating a directory,
// a second goroutine make the same request, at last
func TestFileSystem_RedoProcess(t *testing.T) {
	tests := []struct {
		disruptKeyword string
	}{
		{"cmaa1"},
		{"cmaa2"},
		{"cmaa3"},
	}
	for index, test := range tests {
		// make the disrupter
		c := make(chan struct{})
		var dr disrupter
		dr = make(standardDisrupter).registerDisruptFunc(test.disruptKeyword,
			makeBlockDisruptFunc(c, func() bool { return false }))
		dr = newCounterDisrupter(dr)

		// create FileSystem and create a new DxFile
		ct := &alwaysFailContractor{}
		fs := newEmptyTestFileSystem(t, "", ct, dr)

		path := storage.RootDxPath()
		ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
		if err != nil {
			t.Fatal(err)
		}
		fileSize := uint64(1 << 22 * 10 * 10)
		path, err = path.Join(randomDxPath(t, 1).Path)
		df, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
		if err != nil {
			t.Fatal(err)
		}

		// calculate the metadata to expect at root
		var health, stuckHealth, numStuckSegments, minRedundancy uint32
		health, stuckHealth, numStuckSegments, minRedundancy = 200, 200, 0, 300

		health, stuckHealth, numStuckSegments, minRedundancy = fs.healthParamsUpdate(df, health, stuckHealth, numStuckSegments, minRedundancy)

		if err = df.Close(); err != nil {
			t.Fatal(err)
		}
		expectMd := &dxdir.Metadata{
			NumFiles:         1,
			TotalSize:        fileSize,
			Health:           health,
			StuckHealth:      stuckHealth,
			MinRedundancy:    minRedundancy,
			NumStuckSegments: numStuckSegments,
			DxPath:           storage.RootDxPath(),
			RootPath:         fs.rootDir,
		}

		// Create the first update. The update should be blocked right now
		err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
		if err != nil {
			t.Fatal(err)
		}
		// Create the second update. The second update should return right away since there is
		// already a thread updating the metadata
		err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
		if err != nil {
			t.Fatal(err)
		}

		// Check the redo field of the metadata, it should be redoNeeded
		<-time.After(100 * time.Millisecond)
		func() {
			fs.lock.Lock()
			defer fs.lock.Unlock()
			if len(fs.unfinishedUpdates) != 1 {
				t.Errorf("test %d: unfinishedUpdates should have length 1. but got %v", index, len(fs.unfinishedUpdates))
			}
			update, exist := fs.unfinishedUpdates[storage.RootDxPath()]
			if !exist {
				t.Fatalf("test %d: during update, the dxdir is not in the root", index)
			}
			if atomic.LoadUint32(&update.redo) != redoNeeded {
				t.Fatalf("test %d: redo should be redoNeeded", index)
			}
		}()
		// unblock the first update and wait the updates to complete
		close(c)
		if err = fs.waitForUpdatesComplete(); err != nil {
			t.Fatal(err)
		}
		cdr := dr.(counterDisrupter)
		num, exist := cdr.counter[test.disruptKeyword]
		if !exist {
			t.Fatalf("test %d: not disrupted for keyword: %v", index, test.disruptKeyword)
		}
		if num != 2 {
			t.Errorf("test %d: not twice execution of the disrupt: %v", index, num)
		}
		fs.postTestCheck(t, true, true, expectMd)
	}
}

// TestFileSystem_SingleFail test the senario for a single fail for an update.
// After the process, the root metadata should be updated as expected
func TestFileSystem_SingleFail(t *testing.T) {
	// make the disrupter that will only block for once
	var once sync.Once
	var dr disrupter
	dr = make(standardDisrupter).registerDisruptFunc("cmaa1", func() bool {
		block := false
		once.Do(func() {
			block = true
		})
		return block
	})
	dr = newCounterDisrupter(dr)

	// create FileSystem and create a new DxFile
	ct := &alwaysFailContractor{}
	fs := newEmptyTestFileSystem(t, "", ct, dr)
	path := storage.RootDxPath()
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	fileSize := uint64(1 << 22 * 10 * 10)
	path, err = path.Join(randomDxPath(t, 1).Path)
	df, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}

	// calculate the metadata to expect at root
	var health, stuckHealth, numStuckSegments, minRedundancy uint32
	health, stuckHealth, numStuckSegments, minRedundancy = 200, 200, 0, 300
	health, stuckHealth, numStuckSegments, minRedundancy = fs.healthParamsUpdate(df, health, stuckHealth, numStuckSegments, minRedundancy)
	if err = df.Close(); err != nil {
		t.Fatal(err)
	}
	expectMd := &dxdir.Metadata{
		NumFiles:         1,
		TotalSize:        fileSize,
		Health:           health,
		StuckHealth:      stuckHealth,
		MinRedundancy:    minRedundancy,
		NumStuckSegments: numStuckSegments,
		DxPath:           storage.RootDxPath(),
		RootPath:         fs.rootDir,
	}
	// start the dir update
	err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
	if err != nil {
		t.Fatal(err)
	}
	// Now the update should be in the unfinishedUpdates and waiting to be executed again
	func() {
		fs.lock.Lock()
		defer fs.lock.Unlock()

		if len(fs.unfinishedUpdates) != 1 {
			t.Fatalf("unfinishedUpdates have length expect %v, got %v", 1, len(fs.unfinishedUpdates))
		}

		if _, exist := fs.unfinishedUpdates[storage.RootDxPath()]; !exist {
			t.Fatal("update not exist in fs.unfinishedUpdates")
		}
	}()
	// This might take some time to wait for the loop repair to complete
	if err = fs.waitForUpdatesComplete(); err != nil {
		t.Fatal(err)
	}

	// Check that the disrupter has been accessed twice
	cdr := dr.(counterDisrupter)
	num := cdr.count("cmaa1")
	if num != 2 {
		t.Errorf("disrupt should be accessed twice. But instead got %d", num)
	}
	fs.postTestCheck(t, true, true, expectMd)
}

// TestFileSystem_ConsecutiveFails test the scenario of when updating a file, the file fails
// three times. In this case, the dxdir metadata will not be updated and have the default value
func TestFileSystem_ConsecutiveFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skip for short")
	}
	// make the disrupter. Always fails
	dr := make(standardDisrupter).registerDisruptFunc("cmaa1", func() bool { return true })
	cdr := newCounterDisrupter(dr)

	// create FileSystem and create a new DxFile
	ct := &alwaysFailContractor{}
	fs := newEmptyTestFileSystem(t, "", ct, cdr)
	path := storage.RootDxPath()
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	fileSize := uint64(1 << 22 * 10 * 10)
	path, err = path.Join(randomDxPath(t, 1).Path)
	df, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = df.Close(); err != nil {
		t.Fatal(err)
	}

	// Since it is expected not to be updated successfully, the root metadata should be of default value
	expectMd := &dxdir.Metadata{
		NumFiles:         0,
		TotalSize:        0,
		Health:           dxdir.DefaultHealth,
		StuckHealth:      dxdir.DefaultHealth,
		MinRedundancy:    math.MaxUint32,
		NumStuckSegments: 0,
		DxPath:           storage.RootDxPath(),
		RootPath:         fs.rootDir,
	}
	// start the dir update
	err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
	if err != nil {
		t.Fatal(err)
	}
	// Now the update should be in the unfinishedUpdates and waiting to be executed again
	func() {
		fs.lock.Lock()
		defer fs.lock.Unlock()

		if len(fs.unfinishedUpdates) != 1 {
			t.Fatalf("unfinishedUpdates have length expect %v, got %v", 1, len(fs.unfinishedUpdates))
		}

		if _, exist := fs.unfinishedUpdates[storage.RootDxPath()]; !exist {
			t.Fatal("update not exist in fs.unfinishedUpdates")
		}
	}()
	// This might take some time to wait for the loop repair to complete
	if err = fs.waitForUpdatesComplete(); err != nil {
		t.Fatal(err)
	}

	// Check that the disrupter has been accessed twice
	num := cdr.count("cmaa1")
	if num != numConsecutiveFailRelease {
		t.Errorf("disrupt should be accessed twice. But instead got %d", num)
	}
	fs.postTestCheck(t, true, true, expectMd)
}

// TestFileSystem_FailedRecover test the scenario of closing the file system when the update is
// on going. It is supposed to be unclean shut down. And later open that file system again. Finally,
// the result shall be as expected.
func TestFileSystem_FailedRecover(t *testing.T) {
	// make the disrupter. Always fails
	dr := make(standardDisrupter).registerDisruptFunc("cmaa1", func() bool { return true })

	// create FileSystem and create a new DxFile
	ct := &alwaysFailContractor{}
	fs := newEmptyTestFileSystem(t, "", ct, dr)
	path := storage.RootDxPath()
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	fileSize := uint64(1 << 22 * 10 * 10)
	path, err = path.Join(randomDxPath(t, 1).Path)
	df, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}

	persistDir := fs.persistDir
	if err = df.Close(); err != nil {
		t.Fatal(err)
	}
	// Since it is expected not to be updated successfully, the root metadata should be of default value
	defaultMd := &dxdir.Metadata{
		NumFiles:         0,
		TotalSize:        0,
		Health:           dxdir.DefaultHealth,
		StuckHealth:      dxdir.DefaultHealth,
		MinRedundancy:    math.MaxUint32,
		NumStuckSegments: 0,
		DxPath:           storage.RootDxPath(),
		RootPath:         fs.rootDir,
	}
	// start the dir update
	err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
	if err != nil {
		t.Fatal(err)
	}

	// Check that the disrupter has been accessed twice
	fs.postTestCheck(t, true, false, defaultMd)

	// Restart the filesystem with always success contractor. The metadata should be updated as expected
	newFs := newFileSystem(string(persistDir), &AlwaysSuccessContractor{}, make(standardDisrupter))
	if err = newFs.Start(); err != nil {
		t.Fatal(err)
	}
	expectMd := &dxdir.Metadata{
		NumFiles:         1,
		TotalSize:        fileSize,
		Health:           200,
		StuckHealth:      200,
		MinRedundancy:    300,
		NumStuckSegments: 0,
		DxPath:           storage.RootDxPath(),
		RootPath:         fs.rootDir,
	}
	if err = newFs.waitForUpdatesComplete(); err != nil {
		t.Fatal(err)
	}
	newFs.postTestCheck(t, true, true, expectMd)
}

// TestFileSystem_CorruptedFiles test the scenario of corrupted files.
// Create three files:
//  1. file with corrupted dxfile
//  2. file with corrupted dxdir
//  3. file with all good files
func TestFileSystem_CorruptedFiles(t *testing.T) {
	// create the disrupter
	dr := make(standardDisrupter)

	// create FileSystem and create a new DxFile
	ct := &AlwaysSuccessContractor{}
	fs := newEmptyTestFileSystem(t, "", ct, dr)
	ck, err := crypto.GenerateCipherKey(crypto.GCMCipherCode)
	if err != nil {
		t.Fatal(err)
	}
	fileSize := uint64(1 << 22 * 10 * 10)

	// create three files
	// 1. corrupted dxfile
	path := randomDxPath(t, 1)
	corruptedDxFile, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = corruptedDxFile.Close(); err != nil {
		t.Fatal(err)
	}
	// 2. corrupted dxdir
	path = randomDxPath(t, 2)
	corruptedDirFile, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = corruptedDirFile.Close(); err != nil {
		t.Fatal(err)
	}
	// 3. Good file
	path = randomDxPath(t, 1)
	goodFile, err := fs.FileSet.NewRandomDxFile(path, 10, 30, erasurecode.ECTypeStandard, ck, fileSize, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = goodFile.Close(); err != nil {
		t.Fatal(err)
	}

	// update dxdir metadata
	dirParent, err := corruptedDirFile.DxPath().Parent()
	if err != nil {
		t.Fatal(err)
	}
	err = fs.InitAndUpdateDirMetadata(dirParent)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the updates to complete
	if err = fs.waitForUpdatesComplete(); err != nil {
		t.Fatal(err)
	}

	// corrupt the files
	file, err := os.OpenFile(string(corruptedDxFile.FilePath()), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("!@#*&^%$#"), 0); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	corruptDxDirFile := filepath.Join(filepath.Dir(string(corruptedDirFile.FilePath())), dxdir.DirFileName)
	file, err = os.OpenFile(corruptDxDirFile, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("!@#*&^%$#"), 0); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	// update again
	err = fs.InitAndUpdateDirMetadata(storage.RootDxPath())
	if err != nil {
		t.Fatal(err)
	}
	if err = fs.waitForUpdatesComplete(); err != nil {
		t.Fatal(err)
	}

	// expectMd is supposed to be the metadata of just the goodFile
	expectMd := &dxdir.Metadata{
		NumFiles:         1,
		TotalSize:        fileSize,
		Health:           dxdir.DefaultHealth,
		StuckHealth:      dxdir.DefaultHealth,
		MinRedundancy:    300,
		NumStuckSegments: 0,
		DxPath:           storage.RootDxPath(),
		RootPath:         fs.rootDir,
	}
	// start the dir update
	fs.postTestCheck(t, true, true, expectMd)
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

// postTestCheck check the post test status. Checks whether could be closed in 1 seconds,
// whether fileWal should be empty, updateWal is empty, and the rootDir's metadata is as expected
func (fs *FileSystem) postTestCheck(t *testing.T, fileWalShouldEmpty bool, updateWalShouldEmpty bool, md *dxdir.Metadata) {
	c := make(chan struct{})
	go func() {
		err := fs.Close()
		if err != nil {
			if fileWalShouldEmpty && updateWalShouldEmpty {
				t.Error(err)
			}
		}
		close(c)
	}()
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Cannot close after certain period")
	}
	persistDir := fs.persistDir
	// check fileWal
	fileWal := filepath.Join(string(persistDir), fileWalName)
	wal, txns, err := writeaheadlog.New(fileWal)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) == 0 != fileWalShouldEmpty {
		t.Errorf("fileWal should be empty: %v, but got %v unapplied txns.", fileWalShouldEmpty, len(txns))
	}
	wal.CloseIncomplete()
	// check updateWal
	updateWal := filepath.Join(string(persistDir), updateWalName)
	wal, txns, err = writeaheadlog.New(updateWal)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) == 0 != updateWalShouldEmpty {
		t.Errorf("updateWal should be empty: %v, but got %v unapplied txns.", updateWalShouldEmpty, len(txns))
	}
	wal.CloseIncomplete()
	// check root dxdir file have the expected metadata
	if md != nil {
		d, err := fs.DirSet.Open(storage.RootDxPath())
		if err != nil {
			t.Fatalf("cannot open the root dxdir: %v", err)
		}
		if err = checkMetadataEqual(d.Metadata(), *md); err != nil {
			t.Error(err)
		}
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
