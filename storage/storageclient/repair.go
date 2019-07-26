// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"io/ioutil"
	"os"
	"time"
)

// addStuckSegmentsToHeap adds all the stuck segments in a file to the repair heap
func (client *StorageClient) addStuckSegmentsToHeap(dxPath storage.DxPath) error {
	// Open File
	sf, err := client.fileSystem.OpenDxFile(dxPath)
	if err != nil {
		return fmt.Errorf("unable to open Dxfile %v, error: %v", dxPath, err)
	}
	defer sf.Close()

	// Add stuck segments from file to repair heap
	files := []*dxfile.FileSetEntryWithID{sf}
	hosts := client.refreshHostsAndWorkers()
	hostHealthInfoTable := client.contractManager.HostHealthMap()
	client.createAndPushSegments(files, hosts, targetStuckSegments, hostHealthInfoTable)
	return nil
}

// dirMetadata retrieve the directory metadata and returns the dir metadata after bubble
func (client *StorageClient) dirMetadata(dxPath storage.DxPath) (dxdir.Metadata, error) {
	sysPath := dxPath.SysPath(storage.SysPath(client.fileSystem.RootDir()))
	fi, err := os.Stat(string(sysPath))
	if err != nil {
		return dxdir.Metadata{}, err
	}
	if !fi.IsDir() {
		return dxdir.Metadata{}, fmt.Errorf("%v is not a directory", dxPath)
	}

	dxDir, err := client.fileSystem.OpenDxDir(dxPath)
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
		dxDir, err = client.fileSystem.NewDxDir(dxPath)
	}
	if err != nil {
		return dxdir.Metadata{}, err
	}
	defer dxDir.Close()

	return dxDir.Metadata(), nil
}

// stuckLoop go through the storage client directory and finds the stuck
// Segments and tries to repair them
func (client *StorageClient) stuckLoop() {
	err := client.tm.Add()
	if err != nil {
		return
	}
	defer client.tm.Done()

	// Loop until the storage client has shutdown or until there are no stuck segments
	for {
		<-time.After(100 * time.Millisecond)
		// Wait until the storage client is online to proceed.
		if !client.blockUntilOnline() {
			// The storage client shut down before the internet connection was restored.
			client.log.Info("storage client shutdown before internet connection")
			return
		}

		// Randomly get directory with stuck files
		dir, err := client.fileSystem.RandomStuckDirectory()
		if err != nil && err != filesystem.ErrNoRepairNeeded {
			// sleep 5 seconds. wait for the filesystem
			<-time.After(5 * time.Second)
			continue
		}
		if err == filesystem.ErrNoRepairNeeded {
			// Block until new work is required
			select {
			case <-client.tm.StopChan():
				// The storage client has shut down
				return
			case <-client.fileSystem.StuckFoundChan():
				// Health Loop found stuck segment
			case dxPath := <-client.uploadHeap.stuckSegmentSuccess:
				// Stuck segment was successfully repaired. Add the rest of the file to the heap
				err := client.addStuckSegmentsToHeap(dxPath)
				if err != nil {
					client.log.Error("unable to add stuck segments from file", dxPath, "to heap:", err)
				}
			}
			continue
		}

		// Refresh the worker pool and get the set of hosts that are currently
		// useful for uploading.
		hosts := client.refreshHostsAndWorkers()

		// push stuck segment to upload heap
		client.pushDirOrFileToSegmentHeap(dir.DxPath(), true, hosts, targetStuckSegments)

		client.uploadHeap.mu.Lock()
		heapLen := client.uploadHeap.heap.Len()
		client.uploadHeap.mu.Unlock()
		if heapLen == 0 {
			continue
		}

		select {
		case client.uploadHeap.segmentComing <- struct{}{}:
		default:
		}

		// Call bubble once all segments have been popped off heap
		if err := client.fileSystem.InitAndUpdateDirMetadata(dir.DxPath()); err != nil {
			client.log.Error("[stuck loop]update dir meta data failed", "error", err)
		}

		// Sleep until it is time to try and repair another stuck Segment
		rebuildStuckHeapSignal := time.After(RepairStuckSegmentInterval)
		select {
		case <-client.tm.StopChan():
			// Return if the return has been shutdown
			return
		case <-rebuildStuckHeapSignal:
			// Time to find another random segment
		case dxPath := <-client.uploadHeap.stuckSegmentSuccess:
			// Stuck segment was successfully repaired. Add the rest of the file
			// to the heap
			err := client.addStuckSegmentsToHeap(dxPath)
			if err != nil {
				client.log.Error("unable to add stuck segments from file", "dxpath", dxPath, "error", err)
			}
		}
	}
}

// healthCheckLoop reads all the dxfiles in the storage client, calculates
// the health of each file and updates the directory metadata
func (client *StorageClient) healthCheckLoop() {
	err := client.tm.Add()
	if err != nil {
		return
	}
	defer client.tm.Done()
	// Loop until the storage client has shutdown or until the storage client's top level files
	// directory has a LasHealthCheckTime within the healthCheckInterval
	for {
		select {
		case <-client.tm.StopChan():
			return
		default:
		}
		// get path of oldest time, return directory and timestamp
		dxPath, lastHealthCheckTime, err := client.fileSystem.OldestLastTimeHealthCheck()
		if err != nil {
			client.log.Error("could not find oldest health check time", "error", err)
			// sleep 3 seconds. Avoid consuming cpu
			<-time.After(3 * time.Second)
			continue
		}

		var nextCheckTime time.Duration
		timeSinceLastCheck := time.Since(lastHealthCheckTime)
		if timeSinceLastCheck > HealthCheckInterval {
			nextCheckTime = 0
		} else {
			nextCheckTime = HealthCheckInterval - timeSinceLastCheck
		}
		healthCheckSignal := time.After(nextCheckTime)
		select {
		case <-client.tm.StopChan():
			return
		case <-healthCheckSignal:
			if err := client.fileSystem.InitAndUpdateDirMetadata(dxPath); err != nil {
				client.log.Error("[health check loop]update dir meta data failed", "error", err)
			}
		}
	}
}
