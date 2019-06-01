// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"gitlab.com/NebulousLabs/Sia/build"
	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/modules"
	"gitlab.com/NebulousLabs/Sia/modules/renter/siadir"
	"gitlab.com/NebulousLabs/Sia/modules/renter/siafile"
	"gitlab.com/NebulousLabs/fastrand"
)

// TODO - Adding testing for interruptions

// equalBubbledMetadata is a helper that checks for equality in the siadir
// metadata that gets bubbled
func equalBubbledMetadata(md1, md2 siadir.Metadata) error {
	// Check AggregateNumFiles
	if md1.AggregateNumFiles != md2.AggregateNumFiles {
		return fmt.Errorf("AggregateNumFiles not equal, %v and %v", md1.AggregateNumFiles, md2.AggregateNumFiles)
	}
	// Check Size
	if md1.AggregateSize != md2.AggregateSize {
		return fmt.Errorf("aggregate sizes not equal, %v and %v", md1.AggregateSize, md2.AggregateSize)
	}
	// Check Health
	if md1.Health != md2.Health {
		return fmt.Errorf("healths not equal, %v and %v", md1.Health, md2.Health)
	}
	// Check LastHealthCheckTimes
	if md2.LastHealthCheckTime != md1.LastHealthCheckTime {
		return fmt.Errorf("LastHealthCheckTimes not equal %v and %v", md2.LastHealthCheckTime, md1.LastHealthCheckTime)
	}
	// Check MinRedundancy
	if md1.MinRedundancy != md2.MinRedundancy {
		return fmt.Errorf("MinRedundancy not equal, %v and %v", md1.MinRedundancy, md2.MinRedundancy)
	}
	// Check Mod Times
	if md2.ModTime != md1.ModTime {
		return fmt.Errorf("ModTimes not equal %v and %v", md2.ModTime, md1.ModTime)
	}
	// Check NumFiles
	if md1.NumFiles != md2.NumFiles {
		return fmt.Errorf("NumFiles not equal, %v and %v", md1.NumFiles, md2.NumFiles)
	}
	// Check NumStuckChunks
	if md1.NumStuckChunks != md2.NumStuckChunks {
		return fmt.Errorf("NumStuckChunks not equal, %v and %v", md1.NumStuckChunks, md2.NumStuckChunks)
	}
	// Check NumSubDirs
	if md1.NumSubDirs != md2.NumSubDirs {
		return fmt.Errorf("NumSubDirs not equal, %v and %v", md1.NumSubDirs, md2.NumSubDirs)
	}
	// Check StuckHealth
	if md1.StuckHealth != md2.StuckHealth {
		return fmt.Errorf("stuck healths not equal, %v and %v", md1.StuckHealth, md2.StuckHealth)
	}
	return nil
}

// TestBubbleHealth tests to make sure that the health of the most in need file
// in a directory is bubbled up to the right levels and probes the supporting
// functions as well
func TestBubbleHealth(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Check to make sure bubble doesn't error on an empty directory
	rt.renter.threadedBubbleMetadata(modules.RootSiaPath())
	defaultMetadata := siadir.Metadata{
		Health:              siadir.DefaultDirHealth,
		StuckHealth:         siadir.DefaultDirHealth,
		LastHealthCheckTime: time.Now(),
		NumStuckChunks:      0,
	}
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		metadata, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Check Health
		if err = equalBubbledMetadata(metadata, defaultMetadata); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a test directory with the following healths
	//
	// root/ 1
	// root/SubDir1/ 1
	// root/SubDir1/SubDir1/ 1
	// root/SubDir1/SubDir2/ 4

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	subDir1_1, err := subDir1.Join(subDir1.String())
	if err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_1); err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}

	// Set Healths of all the directories so they are not the defaults
	//
	// NOTE: You cannot set the NumStuckChunks to a non zero number without a
	// file in the directory as this will create a developer error
	var siaPath modules.SiaPath
	checkTime := time.Now()
	metadataUpdate := siadir.Metadata{
		Health:              1,
		StuckHealth:         0,
		LastHealthCheckTime: checkTime,
	}
	if err := rt.renter.staticDirSet.UpdateMetadata(modules.RootSiaPath(), metadataUpdate); err != nil {
		t.Fatal(err)
	}
	siaPath = subDir1
	if err := rt.renter.staticDirSet.UpdateMetadata(siaPath, metadataUpdate); err != nil {
		t.Fatal(err)
	}
	siaPath = subDir1_1
	if err := rt.renter.staticDirSet.UpdateMetadata(siaPath, metadataUpdate); err != nil {
		t.Fatal(err)
	}
	// Set health of subDir1/subDir2 to be the worst and set the
	siaPath = subDir1_2
	metadataUpdate.Health = 4
	if err := rt.renter.staticDirSet.UpdateMetadata(siaPath, metadataUpdate); err != nil {
		t.Fatal(err)
	}

	// Bubble the health of the directory that has the worst pre set health
	// subDir1/subDir2, the health that gets bubbled should be the health of
	// subDir1/subDir1 since subDir1/subDir2 is empty meaning it's calculated
	// health will return to the default health, even through we set the health
	// to be the worst health
	//
	// Note: this tests the edge case of bubbling an empty directory and
	// directories with no files but do have sub directories since bubble will
	// execute on all the parent directories
	rt.renter.threadedBubbleMetadata(siaPath)
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		metadata, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Compare to metadata of subDir1/subDir1
		expectedHealth, err := rt.renter.managedDirectoryMetadata(subDir1_1)
		if err != nil {
			return err
		}
		if err = equalBubbledMetadata(metadata, expectedHealth); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add a file to the lowest level
	//
	// Worst health with current erasure coding is 2 = (1 - (0-1)/1)
	rsc, _ := siafile.NewRSCode(1, 1)
	siaPath, err = subDir1_2.Join("test")
	if err != nil {
		t.Fatal(err)
	}
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     siaPath,
		ErasureCode: rsc,
	}
	f, err := rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}
	// Since we are just adding the file, no chunks will have been uploaded
	// meaning the health of the file should be the worst case health. Now the
	// health that is bubbled up should be the health of the file added to
	// subDir1/subDir2
	//
	// Note: this tests the edge case of bubbling a directory with a file
	// but no sub directories
	offline, goodForRenew, _ := rt.renter.managedRenterContractsAndUtilities([]*siafile.SiaFileSetEntry{f})
	fileHealth, _, _ := f.Health(offline, goodForRenew)
	if fileHealth != 2 {
		t.Fatalf("Expected heath to be 2, got %v", fileHealth)
	}

	// Mark the file as stuck by marking one of its chunks as stuck
	f.SetStuck(0, true)

	// Now when we bubble the health and check for the worst health we should still see
	// that the health is the health of subDir1/subDir1 which was set to 1 again
	// and the stuck health will be the health of the stuck file
	rt.renter.threadedBubbleMetadata(siaPath)
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		metadata, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Compare to metadata of subDir1/subDir1
		expectedHealth, err := rt.renter.managedDirectoryMetadata(subDir1_1)
		if err != nil {
			return err
		}
		expectedHealth.StuckHealth = 2
		expectedHealth.NumStuckChunks++
		if err = equalBubbledMetadata(metadata, expectedHealth); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Mark the file as un-stuck
	f.SetStuck(0, false)

	// Now if we bubble the health and check for the worst health we should see
	// that the health is the health of the file
	rt.renter.threadedBubbleMetadata(siaPath)
	expectedHealth := siadir.Metadata{
		Health:      2,
		StuckHealth: 0,
	}
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		health, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Check Health
		if err = equalBubbledMetadata(health, expectedHealth); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update the RecentRepairTime of the file and confirm that the file's
	// health is now ignored
	if err := f.UpdateRecentRepairTime(); err != nil {
		t.Fatal(err)
	}
	expectedHealth.Health = 0
	rt.renter.threadedBubbleMetadata(siaPath)
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		health, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Check Health
		if err = equalBubbledMetadata(health, expectedHealth); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add a sub directory to the directory that contains the file that has a
	// worst health than the file and confirm that health gets bubbled up.
	//
	// Note: this tests the edge case of bubbling a directory that has both a
	// file and a sub directory
	subDir1_2_1, err := subDir1_2.Join(subDir1.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2_1); err != nil {
		t.Fatal(err)
	}
	// Reset metadataUpdate with expected values
	expectedHealth = siadir.Metadata{
		Health:              4,
		StuckHealth:         0,
		LastHealthCheckTime: time.Now(),
	}
	if err := rt.renter.staticDirSet.UpdateMetadata(subDir1_2_1, expectedHealth); err != nil {
		t.Fatal(err)
	}
	rt.renter.threadedBubbleMetadata(siaPath)
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		health, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Check Health
		if err = equalBubbledMetadata(health, expectedHealth); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestOldestHealthCheckTime probes managedOldestHealthCheckTime to verify that
// the directory with the oldest LastHealthCheckTime is returned
func TestOldestHealthCheckTime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a test directory with sub folders
	//
	// root/ 1
	// root/SubDir1/
	// root/SubDir1/SubDir2/
	// root/SubDir2/

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir2); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}

	// Set the LastHealthCheckTime of SubDir1/SubDir2 to be the oldest
	oldestCheckTime := time.Now().AddDate(0, 0, -1)
	oldestHealthCheckUpdate := siadir.Metadata{
		Health:              1,
		StuckHealth:         0,
		LastHealthCheckTime: oldestCheckTime,
	}
	if err := rt.renter.staticDirSet.UpdateMetadata(subDir1_2, oldestHealthCheckUpdate); err != nil {
		t.Fatal(err)
	}

	// Bubble the health of SubDir1 so that the oldest LastHealthCheckTime of
	// SubDir1/SubDir2 gets bubbled up
	rt.renter.threadedBubbleMetadata(subDir1)

	// Find the oldest directory, should be SubDir1/SubDir2
	build.Retry(100, 100*time.Millisecond, func() error {
		dir, lastCheck, err := rt.renter.managedOldestHealthCheckTime()
		if err != nil {
			return err
		}
		if dir.Equals(subDir1_2) {
			return fmt.Errorf("Expected to find %v but found %v", subDir1_2.String(), dir.String())
		}
		if !lastCheck.Equal(oldestCheckTime) {
			return fmt.Errorf("Expected to find time of %v but found %v", oldestCheckTime, lastCheck)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestWorstHealthDirectory verifies that managedWorstHealthDirectory returns
// the correct directory
func TestWorstHealthDirectory(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a test directory with sub folders
	//
	// root/ 1
	// root/SubDir1/
	// root/SubDir1/SubDir2/
	// root/SubDir2/

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir2); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}

	// Confirm worst health directory is the top level directory since all
	// directories should be at the default health which is 0 or full health
	rt.renter.threadedBubbleMetadata(subDir1)
	build.Retry(100, 100*time.Millisecond, func() error {
		dir, health, err := rt.renter.managedWorstHealthDirectory()
		if err != nil {
			return err
		}
		if dir.Equals(modules.RootSiaPath()) {
			return fmt.Errorf("Expected to find top level directory but found %v", dir.String())
		}
		if health != 0 {
			return fmt.Errorf("Expected to find health of %v but found %v", 0, health)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set the Health of SubDir1/SubDir2 to be the worst
	worstHealth := float64(10)
	worstHealthUpdate := siadir.Metadata{
		Health:              worstHealth,
		StuckHealth:         0,
		LastHealthCheckTime: time.Now(),
	}
	if err := rt.renter.staticDirSet.UpdateMetadata(subDir1_2, worstHealthUpdate); err != nil {
		t.Fatal(err)
	}

	// Bubble the health of SubDir1 so that the worst health of
	// SubDir1/SubDir2 gets bubbled up
	rt.renter.threadedBubbleMetadata(subDir1)

	// Find the worst health directory, should be SubDir1/SubDir2
	build.Retry(100, 100*time.Millisecond, func() error {
		dir, health, err := rt.renter.managedWorstHealthDirectory()
		if err != nil {
			return err
		}
		if dir.Equals(subDir1_2) {
			return fmt.Errorf("Expected to find %v but found %v", subDir1_2.String(), dir.String())
		}
		if health != worstHealth {
			return fmt.Errorf("Expected to find health of %v but found %v", worstHealth, health)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add file to SubDir1/SubDir2
	rsc, _ := siafile.NewRSCode(1, 1)
	siaPath, err := subDir1_2.Join("test")
	if err != nil {
		t.Fatal(err)
	}
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     siaPath,
		ErasureCode: rsc,
	}
	f, err := rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Bubble health, confirm that the health worst directory is still
	// SubDir1/SubDir2
	rt.renter.threadedBubbleMetadata(subDir1_2)

	// Worst health with current erasure coding is 2 = (1 - (0-1)/1)
	worstHealth = float64(2)
	build.Retry(100, 100*time.Millisecond, func() error {
		dir, health, err := rt.renter.managedWorstHealthDirectory()
		if err != nil {
			return err
		}
		if dir.Equals(subDir1_2) {
			return fmt.Errorf("Expected to find %v but found %v", subDir1_2.String(), dir.String())
		}
		if health != worstHealth {
			return fmt.Errorf("Expected to find health of %v but found %v", worstHealth, health)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update file's recent repair time
	if err := f.UpdateRecentRepairTime(); err != nil {
		t.Fatal(err)
	}

	// Bubble Health and confirm that the worst directory is now the top level
	// directory
	rt.renter.threadedBubbleMetadata(siaPath)
	build.Retry(100, 100*time.Millisecond, func() error {
		dir, health, err := rt.renter.managedWorstHealthDirectory()
		if err != nil {
			return err
		}
		if dir != siaPath {
			return fmt.Errorf("Expected to find %v but found %v", siaPath, dir)
		}
		if health != float64(0) {
			return fmt.Errorf("Expected to find health of %v but found %v", float64(0), health)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNumFiles verifies that the number of files and aggregate number of files
// is accurately reported
func TestNumFiles(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a test directory with sub folders
	//
	// root/ file
	// root/SubDir1/
	// root/SubDir1/SubDir2/ file

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}
	// Add files
	rsc, _ := siafile.NewRSCode(1, 1)
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     newRandSiaPath(),
		ErasureCode: rsc,
	}
	_, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}
	up.SiaPath, err = subDir1_2.Join(hex.EncodeToString(fastrand.Bytes(8)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Call bubble on lowest lever and confirm top level reports accurate number
	// of files and aggregate number of files
	rt.renter.threadedBubbleMetadata(subDir1_2)
	build.Retry(100, 100*time.Millisecond, func() error {
		dirInfo, err := rt.renter.DirInfo(modules.RootSiaPath())
		if err != nil {
			return err
		}
		if dirInfo.NumFiles != 1 {
			return fmt.Errorf("NumFiles incorrect, got %v expected %v", dirInfo.NumFiles, 1)
		}
		if dirInfo.AggregateNumFiles != 2 {
			return fmt.Errorf("AggregateNumFiles incorrect, got %v expected %v", dirInfo.AggregateNumFiles, 2)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDirectorySize verifies that the Size of a directory is accurately
// reported
func TestDirectorySize(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a test directory with sub folders
	//
	// root/ file
	// root/SubDir1/
	// root/SubDir1/SubDir2/ file

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}
	// Add files
	rsc, _ := siafile.NewRSCode(1, 1)
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     newRandSiaPath(),
		ErasureCode: rsc,
	}
	fileSize := uint64(100)
	_, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), fileSize, 0777)
	if err != nil {
		t.Fatal(err)
	}
	up.SiaPath, err = subDir1_2.Join(hex.EncodeToString(fastrand.Bytes(8)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 2*fileSize, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Call bubble on lowest lever and confirm top level reports accurate size
	rt.renter.threadedBubbleMetadata(subDir1_2)
	build.Retry(100, 100*time.Millisecond, func() error {
		dirInfo, err := rt.renter.DirInfo(modules.RootSiaPath())
		if err != nil {
			return err
		}
		if dirInfo.AggregateSize != 3*fileSize {
			return fmt.Errorf("AggregateSize incorrect, got %v expected %v", dirInfo.AggregateSize, 3*fileSize)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDirectoryModTime verifies that the last update time of a directory is
// accurately reported
func TestDirectoryModTime(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create a test directory with sub folders
	//
	// root/ file
	// root/SubDir1/
	// root/SubDir1/SubDir2/ file

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create directory tree
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}
	// Add files
	rsc, _ := siafile.NewRSCode(1, 1)
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     newRandSiaPath(),
		ErasureCode: rsc,
	}
	fileSize := uint64(100)
	_, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), fileSize, 0777)
	if err != nil {
		t.Fatal(err)
	}
	up.SiaPath, err = subDir1_2.Join(hex.EncodeToString(fastrand.Bytes(8)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), fileSize, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Call bubble on lowest lever and confirm top level reports accurate last
	// update time
	rt.renter.threadedBubbleMetadata(subDir1_2)
	build.Retry(100, 100*time.Millisecond, func() error {
		dirInfo, err := rt.renter.DirInfo(modules.RootSiaPath())
		if err != nil {
			return err
		}
		if dirInfo.MostRecentModTime != f.ModTime() {
			return fmt.Errorf("ModTime is incorrect, got %v expected %v", dirInfo.MostRecentModTime, f.ModTime())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRandomStuckDirectory probes managedStuckDirectory to make sure it
// randomly picks a correct directory
func TestRandomStuckDirectory(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create test renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create a test directory with sub folders
	//
	// root/
	// root/SubDir1/
	// root/SubDir1/SubDir2/
	// root/SubDir2/
	subDir1, err := modules.NewSiaPath("SubDir1")
	if err != nil {
		t.Fatal(err)
	}
	subDir2, err := modules.NewSiaPath("SubDir2")
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1); err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir2); err != nil {
		t.Fatal(err)
	}
	subDir1_2, err := subDir1.Join(subDir2.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.renter.CreateDir(subDir1_2); err != nil {
		t.Fatal(err)
	}

	// Add a file to root and SubDir1/SubDir2 and mark the first chunk as stuck
	// in each file
	//
	// This will test the edge case of continuing to find stuck files when a
	// directory has no files only directories
	rsc, _ := siafile.NewRSCode(1, 1)
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     newRandSiaPath(),
		ErasureCode: rsc,
	}
	f, err := rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.SetStuck(uint64(0), true); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	up.SiaPath, err = subDir1_2.Join(hex.EncodeToString(fastrand.Bytes(8)))
	if err != nil {
		t.Fatal(err)
	}
	f, err = rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), 100, 0777)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.SetStuck(uint64(0), true); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	// Bubble directory information so NumStuckChunks is updated, there should
	// be at least 2 stuck chunks because of the two we manually marked as
	// stuck, but the repair loop could have marked the rest as stuck so we just
	// want to ensure that the root directory reflects at least the two we
	// marked as stuck
	rt.renter.threadedBubbleMetadata(subDir1_2)
	build.Retry(100, 100*time.Millisecond, func() error {
		// Get Root Directory Health
		metadata, err := rt.renter.managedDirectoryMetadata(modules.RootSiaPath())
		if err != nil {
			return err
		}
		// Check Health
		if metadata.NumStuckChunks < uint64(2) {
			return fmt.Errorf("Incorrect number of stuck chunks, should be at least 2")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create map of stuck directories that contain files. These are the only
	// directories that should be returned.
	stuckDirectories := make(map[modules.SiaPath]struct{})
	stuckDirectories[modules.RootSiaPath()] = struct{}{}
	stuckDirectories[subDir1_2] = struct{}{}

	// Find random directory several times, confirm that it finds a stuck
	// directory and there it finds unique directories
	var unique bool
	var previousDir modules.SiaPath
	for i := 0; i < 10; i++ {
		dir, err := rt.renter.managedStuckDirectory()
		if err != nil {
			t.Log("Error with Directory", dir)
			t.Fatal(err)
		}
		_, ok := stuckDirectories[dir]
		if !ok {
			t.Fatal("Found non stuck directory:", dir)
		}
		if !dir.Equals(previousDir) {
			unique = true
		}
		previousDir = dir
	}
	if !unique {
		t.Fatal("No unique directories found")
	}
}

// TestCalculateFileMetadata checks that the values returned from
// managedCalculateFileMetadata make sense
func TestCalculateFileMetadata(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()

	// Create renter
	rt, err := newRenterTester(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create a file
	rsc, _ := siafile.NewRSCode(1, 1)
	siaPath, err := modules.NewSiaPath("rootFile")
	if err != nil {
		t.Fatal(err)
	}
	up := modules.FileUploadParams{
		Source:      "",
		SiaPath:     siaPath,
		ErasureCode: rsc,
	}
	fileSize := uint64(100)
	sf, err := rt.renter.staticFileSet.NewSiaFile(up, crypto.GenerateSiaKey(crypto.RandomCipherType()), fileSize, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Grab initial metadata values
	offline, goodForRenew, _ := rt.renter.managedRenterContractsAndUtilities([]*siafile.SiaFileSetEntry{sf})
	health, stuckHealth, numStuckChunks := sf.Health(offline, goodForRenew)
	redundancy := sf.Redundancy(offline, goodForRenew)
	lastHealthCheckTime := sf.LastHealthCheckTime()
	modTime := sf.ModTime()
	recentRepairTime := sf.RecentRepairTime()

	// Check calculated metadata
	fileMetadata, err := rt.renter.managedCalculateFileMetadata(up.SiaPath)
	if err != nil {
		t.Fatal(err)
	}

	// Check siafile calculated metadata
	if fileMetadata.Health != health {
		t.Fatalf("health incorrect, expected %v got %v", health, fileMetadata.Health)
	}
	if fileMetadata.StuckHealth != stuckHealth {
		t.Fatalf("stuckHealth incorrect, expected %v got %v", stuckHealth, fileMetadata.StuckHealth)
	}
	if fileMetadata.Redundancy != redundancy {
		t.Fatalf("redundancy incorrect, expected %v got %v", redundancy, fileMetadata.Redundancy)
	}
	if fileMetadata.Size != fileSize {
		t.Fatalf("size incorrect, expected %v got %v", fileSize, fileMetadata.Size)
	}
	if fileMetadata.NumStuckChunks != numStuckChunks {
		t.Fatalf("numstuckchunks incorrect, expected %v got %v", numStuckChunks, fileMetadata.NumStuckChunks)
	}
	if !fileMetadata.RecentRepairTime.Equal(recentRepairTime) {
		t.Fatalf("Unexpected recentrepairtime, expected %v got %v", recentRepairTime, fileMetadata.RecentRepairTime)
	}
	if fileMetadata.LastHealthCheckTime.Equal(lastHealthCheckTime) || fileMetadata.LastHealthCheckTime.IsZero() {
		t.Log("Initial lasthealthchecktime", lastHealthCheckTime)
		t.Log("Calculated lasthealthchecktime", fileMetadata.LastHealthCheckTime)
		t.Fatal("Expected lasthealthchecktime to have updated and be non zero")
	}
	if !fileMetadata.ModTime.Equal(modTime) {
		t.Fatalf("Unexpected modtime, expected %v got %v", modTime, fileMetadata.ModTime)
	}
}
