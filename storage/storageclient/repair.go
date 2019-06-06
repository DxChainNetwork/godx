// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

// The following describes the work flow of how Dx repairs files
//
// There are 3 main functions that work together to make up Dx's file repair
// mechanism, threadedUpdateRenterHealth, threadedUploadLoop, and
// threadedStuckFileLoop. These 3 functions will be referred to as the health
// loop, the repair loop, and the stuck loop respectively.
//
// The health loop is responsible for ensuring that the health of the renter's
// file directory is updated periodically. The health information for a
// directory is stored in the .Dxdir metadata file and is the worst values for
// any of the files and sub directories. This is true for all directories which
// means the health of top level directory of the renter is the health of the
// worst file in the renter. For health and stuck health the worst value is the
// highest value, for timestamp values the oldest timestamp is the worst value,
// and for aggregate values (ie NumStuckSegments) it will be the sum of all the
// files and sub directories.  The health loop keeps the renter file directory
// updated by following the path of oldest LastHealthCheckTime and then calling
// threadedBubbleHealth, to be referred to as bubble, on that directory. When a
// directory is bubbled, the health information is recalculated and saved to
// disk and then bubble is called on the parent directory until the top level
// directory is reached. If during a bubble a file is found that meets the
// threshold health for repair, then a signal is sent to the repair loop. If a
// stuck Segment is found then a signal is sent to the stuck loop. Once the entire
// renter's directory has been updated within the healthCheckInterval the health
// loop sleeps until the time interval has passed.
//
// The repair loop is responsible for repairing the renter's files, this
// includes uploads. The repair loop follows the path of worst health and then
// adds the files from the directory with the worst health to the repair heap
// and begins repairing. If no directories are unhealthy enough to require
// repair the repair loop sleeps until a new upload triggers it to start or it
// is triggered by a bubble finding a file that requires repair. While there are
// files to repair, the repair loop will continue to work through the renter's
// directory finding the worst health directories and adding them to the repair
// heap. The rebuildSegmentHeapInterval is used to make sure the repair heap
// doesn't get stuck on repairing a set of Segments for too long. Once the
// rebuildSegmentheapInterval passes, the repair loop will continue in it's search
// for files that need repair. As Segments are repaired, they will call bubble on
// their directory to ensure that the renter directory gets updated.
//
// The stuck loop is responsible for targeting Segments that didn't get repaired
// properly. The stuck loop randomly finds a directory containing stuck Segments
// and adds those to the repair heap. The repair heap will randomly add one
// stuck Segment to the heap at a time. Stuck Segments are priority in the heap, so
// limiting it to 1 stuck Segment at a time prevents the heap from being saturated
// with stuck Segments that potentially cannot be repaired which would cause no
// other files to be repaired. If the repair of a stuck Segment is successful, a
// signal is sent to the stuck loop and another stuck Segment is added to the
// heap. If the repair wasn't successful, the stuck loop will wait for the
// repairStuckSegmentInterval to pass and then try another random stuck Segment. If
// the stuck loop doesn't find any stuck Segments, it will sleep until a bubble
// triggers it by finding a stuck Segment.

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"io/ioutil"
	"os"
	"time"
)

var (
	ErrNoStuckFiles = errors.New("No stuck files")
	ErrNoDirectory  = errors.New("No directories returned from dirList")
	ErrMarkStuck    = errors.New("unable to mark healthy segments as unstuck")
)

// addStuckSegmentsToHeap adds all the stuck Segments in a file to the repair heap
func (sc *StorageClient) addStuckSegmentsToHeap(dxPath storage.DxPath) error {
	// Open File
	sf, err := sc.staticFileSet.Open(dxPath)
	if err != nil {
		return fmt.Errorf("unable to open Dxfile %v, error: %v", dxPath, err)
	}
	defer sf.Close()
	// Add stuck Segments from file to repair heap
	files := []*dxfile.FileSetEntryWithID{sf}
	hosts := sc.refreshHostsAndWorkers()
	hostHealthInfoTable := sc.getClientHostHealthInfoTable(files)
	sc.createAndPushSegments(files, hosts, targetStuckSegments, hostHealthInfoTable)
	return nil
}


// managedDirectoryMetadata reads the directory metadata and returns the bubble metadata
func (sc *StorageClient) managedDirectoryMetadata(dxPath storage.DxPath) (dxdir.Metadata, error) {
	sysPath := dxPath.SysPath(storage.SysPath(sc.staticFilesDir))
	fi, err := os.Stat(string(sysPath))
	if err != nil {
		return dxdir.Metadata{}, err
	}
	if !fi.IsDir() {
		return dxdir.Metadata{}, fmt.Errorf("%v is not a directory", dxPath)
	}

	dxDir, err := sc.staticDirSet.Open(dxPath)
	if os.IsNotExist(err) {
		// Remember initial Error
		initError := err
		// Metadata file does not exists, check if directory is empty
		fileInfos, err := ioutil.ReadDir(string(sysPath))
		if err != nil {
			return dxdir.Metadata{}, err
		}
		// If the directory is empty and is not the root directory, assume it
		// was deleted so do not create a metadata file
		if len(fileInfos) == 0 && !dxPath.IsRoot() {
			return dxdir.Metadata{}, initError
		}
		// If we are at the root directory or the directory is not empty, create
		// a metadata file
		dxDir, err = sc.staticDirSet.NewDxDir(dxPath)
	}
	if err != nil {
		return dxdir.Metadata{}, err
	}
	defer dxDir.Close()

	return dxDir.Metadata(), nil
}

// threadedStuckFileLoop go through the renter directory and finds the stuck
// Segments and tries to repair them
func (sc *StorageClient) threadedStuckFileLoop() {
	err := sc.tm.Add()
	if err != nil {
		return
	}
	defer sc.tm.Done()

	// Loop until the renter has shutdown or until there are no stuck Segments
	for {
		// Wait until the renter is online to proceed.
		if !sc.blockUntilOnline() {
			// The renter shut down before the internet connection was restored.
			sc.log.Debug("renter shutdown before internet connection")
			return
		}

		// Randomly get directory with stuck files
		dirDxPath, err := sc.managedStuckDirectory()
		if err != nil && err != ErrNoStuckFiles {
			sc.log.Debug("WARN: error getting random stuck directory:", err)
			continue
		}
		if err == ErrNoStuckFiles {
			// Block until new work is required.
			select {
			case <-sc.tm.StopChan():
				// The renter has shut down.
				return
			case <-sc.uploadHeap.stuckSegmentFound:
				// Health Loop found stuck Segment
			case DxPath := <-sc.uploadHeap.stuckSegmentSuccess:
				// Stuck Segment was successfully repaired. Add the rest of the file
				// to the heap
				err := sc.addStuckSegmentsToHeap(DxPath)
				if err != nil {
					sc.log.Debug("WARN: unable to add stuck Segments from file", DxPath, "to heap:", err)
				}
			}
			continue
		}

		// Refresh the worker pool and get the set of hosts that are currently
		// useful for uploading.
		hosts := sc.refreshHostsAndWorkers()

		// push stuck Segment to upload heap
		sc.pushDirToSegmentHeap(dirDxPath, hosts, targetStuckSegments)

		sc.uploadLoop(hosts)

		// Call bubble once all Segments have been popped off heap
		sc.fileSystem.InitAndUpdateDirMetadata(dirDxPath)

		// Sleep until it is time to try and repair another stuck Segment
		rebuildStuckHeapSignal := time.After(RepairStuckSegmentInterval)
		select {
		case <-sc.tm.StopChan():
			// Return if the return has been shutdown
			return
		case <-rebuildStuckHeapSignal:
			// Time to find another random Segment
		case DxPath := <-sc.uploadHeap.stuckSegmentSuccess:
			// Stuck Segment was successfully repaired. Add the rest of the file
			// to the heap
			err := sc.addStuckSegmentsToHeap(DxPath)
			if err != nil {
				sc.log.Debug("WARN: unable to add stuck Segments from file", DxPath, "to heap:", err)
			}
		}
	}
}

// threadedUpdateRenterHealth reads all the Dxfiles in the renter, calculates
// the health of each file and updates the folder metadata
func (sc *StorageClient) threadedUpdateRenterHealth() {
	err := sc.tm.Add()
	if err != nil {
		return
	}
	defer sc.tm.Done()
	// Loop until the renter has shutdown or until the renter's top level files
	// directory has a LasHealthCheckTime within the healthCheckInterval
	for {
		select {
		// Check to make sure renter hasn't been shutdown
		case <-sc.tm.StopChan():
			return
		default:
		}
		// Follow path of oldest time, return directory and timestamp
		dxPath, lastHealthCheckTime, err := sc.managedOldestHealthCheckTime()
		if err != nil {
			sc.log.Debug("WARN: Could not find oldest health check time:", err)
			continue
		}

		// If lastHealthCheckTime is within the healthCheckInterval block
		// until it is time to check again
		var nextCheckTime time.Duration
		timeSinceLastCheck := time.Since(lastHealthCheckTime)
		if timeSinceLastCheck > HealthCheckInterval { // Check for underflow
			nextCheckTime = 0
		} else {
			nextCheckTime = HealthCheckInterval - timeSinceLastCheck
		}
		healthCheckSignal := time.After(nextCheckTime)
		select {
		case <-sc.tm.StopChan():
			return
		case <-healthCheckSignal:
			// Bubble directory
			sc.fileSystem.InitAndUpdateDirMetadata(dxPath)
		}
	}
}
