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
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxdir"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNoStuckFiles = errors.New("No stuck files")
	ErrNoDirectory  = errors.New("No directories returned from dirList")
	ErrMarkStuck    = errors.New("unable to mark healthy segments as unstuck")
)

// bubbleStatus indicates the status of a bubble being executed on a
// directory
type bubbleStatus int

// bubbleError, bubbleInit, bubbleActive, and bubblePending are the constants
// used to determine the status of a bubble being executed on a directory
const (
	bubbleError bubbleStatus = iota
	bubbleInit
	bubbleActive
	bubblePending

	DefaultDirHealth = uint32(0)
)

// managedAddStuckSegmentsToHeap adds all the stuck Segments in a file to the repair heap
func (sc *StorageClient) managedAddStuckSegmentsToHeap(dxPath dxdir.DxPath) error {
	// Open File
	sf, err := sc.staticFileSet.Open(dxPath.String())
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

// managedBubbleNeeded checks if a bubble is needed for a directory, updates the
// renter's bubbleUpdates map and returns a bool
func (sc *StorageClient) managedBubbleNeeded(dxPath dxdir.DxPath) (bool, error) {
	sc.bubbleUpdatesMu.Lock()
	defer sc.bubbleUpdatesMu.Unlock()

	// Check for bubble in bubbleUpdate map
	DxPathStr := dxPath.String()
	status, ok := sc.bubbleUpdates[DxPathStr]
	if !ok {
		status = bubbleInit
		sc.bubbleUpdates[DxPathStr] = status
	}

	// Update the bubble status
	var err error
	switch status {
	case bubblePending:
	case bubbleActive:
		sc.bubbleUpdates[DxPathStr] = bubblePending
	case bubbleInit:
		sc.bubbleUpdates[DxPathStr] = bubbleActive
		return true, nil
	default:
		err = errors.New("WARN: invalid bubble status")
	}
	return false, err
}

// managedCalculateDirectoryMetadata calculates the new values for the
// directory's metadata and tracks the value, either worst or best, for each to
// be bubbled up
func (sc *StorageClient) managedCalculateDirectoryMetadata(dxPath dxdir.DxPath) (dxdir.Metadata, error) {
	// Set default metadata values to start
	metadata := dxdir.Metadata{
		NumFiles:   			uint64(0),
		Health:     			DefaultDirHealth,
		TimeLastHealthCheck: 	time.Now(),
		TimeModify:          	time.Now(),
		MinRedundancy:       	math.MaxFloat64,
		NumStuckSegments:      	uint64(0),
		TotalSize:       		uint64(0),
		StuckHealth:         	DefaultDirHealth,
	}
	// Read directory
	fileInfos, err := ioutil.ReadDir(dxPath.DxDirSysPath(sc.staticFilesDir))
	if err != nil {
		sc.log.Debug("Error in reading files in directory %v : %v\n", dxPath.DxDirSysPath(sc.staticFilesDir), err)
		return dxdir.Metadata{}, err
	}

	// Iterate over directory
	for _, fi := range fileInfos {
		// Check to make sure renter hasn't been shutdown
		select {
		case <-sc.tm.StopChan():
			return dxdir.Metadata{}, err
		default:
		}

		var health, stuckHealth, redundancy uint32
		var numStuckSegments uint64
		var lastHealthCheckTime, modTime time.Time
		ext := filepath.Ext(fi.Name())
		// Check for DxFiles and Directories
		if ext == dxdir.DxFileExtension {
			// DxFile found, calculate the needed metadata information of the Dxfile
			fName := strings.TrimSuffix(fi.Name(), dxdir.DxFileExtension)
			fileDxPath, err := dxPath.Join(fName)
			if err != nil {
				return dxdir.Metadata{}, err
			}
			fileMetadata, err := sc.managedCalculateFileMetadata(fileDxPath)
			if err != nil {
				sc.log.Debug("failed to calculate file metadata %v: %v", fi.Name(), err)
				continue
			}
			if time.Since(fileMetadata.RecentRepairTime) >= FileRepairInterval {
				// If the file has not recently been repaired then consider the
				// health of the file
				health = fileMetadata.Health
			}
			lastHealthCheckTime = fileMetadata.LastHealthCheck
			modTime = fileMetadata.TimeModify
			numStuckSegments = fileMetadata.NumStuckSegments
			redundancy = fileMetadata.Redundancy
			stuckHealth = fileMetadata.StuckHealth
			metadata.NumFiles++
			metadata.TotalSize += fileMetadata.Size
		} else if fi.IsDir() {
			// Directory is found, read the directory metadata file
			dirDxPath, err := dxPath.Join(fi.Name())
			if err != nil {
				return dxdir.Metadata{}, err
			}
			dirMetadata, err := sc.managedDirectoryMetadata(dirDxPath)
			if err != nil {
				return dxdir.Metadata{}, err
			}
			health = dirMetadata.Health
			lastHealthCheckTime = dirMetadata.TimeLastHealthCheck
			modTime = dirMetadata.TimeModify
			numStuckSegments = dirMetadata.NumStuckSegments
			redundancy = dirMetadata.MinRedundancy
			stuckHealth = dirMetadata.StuckHealth
			// Update AggregateNumFiles
			metadata.NumFiles += dirMetadata.NumFiles
			// Update Size
			metadata.TotalSize += dirMetadata.TotalSize
		} else {
			// Ignore everthing that is not a DxFile or a directory
			continue
		}
		// Update Health and Stuck Health
		if health > metadata.Health {
			metadata.Health = health
		}
		if stuckHealth > metadata.StuckHealth {
			metadata.StuckHealth = stuckHealth
		}
		// Update ModTime
		if modTime.After(metadata.TimeModify) {
			metadata.TimeModify = modTime
		}
		// Increment NumStuckSegments
		metadata.NumStuckSegments += numStuckSegments
		// Update MinRedundancy
		if redundancy < metadata.MinRedundancy {
			metadata.MinRedundancy = redundancy
		}
		// Update LastHealthCheckTime
		if lastHealthCheckTime.Before(metadata.TimeLastHealthCheck) {
			metadata.TimeLastHealthCheck = lastHealthCheckTime
		}
		metadata.NumStuckSegments += numStuckSegments
	}
	// Sanity check on ModTime. If mod time is still zero it means there were no
	// files or subdirectories. Set ModTime to now since we just updated this
	// directory
	if metadata.TimeModify.IsZero() {
		metadata.TimeModify = time.Now()
	}

	// Sanity check on Redundancy. If MinRedundancy is still math.MaxFloat64
	// then set it to 0
	if metadata.MinRedundancy == math.MaxUint32 {
		metadata.MinRedundancy = 0
	}

	return metadata, nil
}

// managedCalculateFileMetadata calculates and returns the necessary metadata
// information of a Dxfile that needs to be bubbled
func (sc *StorageClient) managedCalculateFileMetadata(dxPath dxdir.DxPath) (dxfile.UpdateMetaData, error) {
	// Load the Dxfile.
	sf, err := sc.staticFileSet.Open(dxPath.String())
	if err != nil {
		return dxfile.UpdateMetaData{}, err
	}
	defer sf.Close()

	// Mark sure that healthy Segments are not marked as stuck
	hostHealthInfoTable := sc.getClientHostHealthInfoTable([]*dxfile.FileSetEntryWithID{sf})
	err = sf.MarkAllHealthySegmentsAsUnstuck(hostHealthInfoTable)
	if err != nil {
		return dxfile.UpdateMetaData{}, ErrMarkStuck
	}
	// Calculate file health
	health, stuckHealth, numStuckSegments := sf.Health(hostHealthInfoTable)
	// Update the LastHealthCheckTime
	if err := sf.SetTimeLastHealthCheck(time.Now()); err != nil {
		return dxfile.UpdateMetaData{}, err
	}
	// Calculate file Redundancy and check if local file is missing and
	// redundancy is less than one
	redundancy := sf.Redundancy(hostHealthInfoTable)
	if _, err := os.Stat(sf.LocalPath()); os.IsNotExist(err) && redundancy < 1 {
		sc.log.Debug("File not found on disk and possibly unrecoverable:", sf.LocalPath())
	}
	metadata := dxfile.CachedHealthMetadata{
		Health:      health,
		Redundancy:  redundancy,
		StuckHealth: stuckHealth,
	}
	return dxfile.UpdateMetaData{
		Health:              	health,
		LastHealthCheck: 		sf.TimeLastHealthCheck(),
		TimeModify:             sf.TimeModify(),
		NumStuckSegments:      	uint64(numStuckSegments),
		Redundancy:          	redundancy,
		Size:                	sf.FileSize(),
		StuckHealth:         	stuckHealth,
	}, sf.ApplyCachedHealthMetadata(metadata)
}

// managedCompleteBubbleUpdate completes the bubble update and updates and/or
// removes it from the renter's bubbleUpdates.
func (sc *StorageClient) managedCompleteBubbleUpdate(dxPath dxdir.DxPath) error {
	sc.bubbleUpdatesMu.Lock()
	defer sc.bubbleUpdatesMu.Unlock()

	// Check current status
	DxPathStr := dxPath.String()
	status, ok := sc.bubbleUpdates[DxPathStr]
	if !ok {
		// Bubble not found in map, nothing to do.
		return nil
	}

	// Update status and call new bubble or remove from bubbleUpdates and save
	switch status {
	case bubblePending:
		sc.bubbleUpdates[DxPathStr] = bubbleInit
		defer func() {
			go sc.threadedBubbleMetadata(dxPath)
		}()
	case bubbleActive:
		delete(sc.bubbleUpdates, DxPathStr)
	default:
		return errors.New("WARN: invalid bubble status")
	}

	return nil
	// TODO bubble update
	//return sc.saveBubbleUpdates()
}

// managedDirectoryMetadata reads the directory metadata and returns the bubble
// metadata
func (sc *StorageClient) managedDirectoryMetadata(dxPath dxdir.DxPath) (dxdir.Metadata, error) {
	sysPath := dxPath.DxDirSysPath(sc.staticFilesDir)
	fi, err := os.Stat(sysPath)
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
		fileInfos, err := ioutil.ReadDir(sysPath)
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

// managedOldestHealthCheckTime finds the lowest level directory that has a
// LastHealthCheckTime that is outside the healthCheckInterval
func (sc *StorageClient) managedOldestHealthCheckTime() (dxdir.DxPath, time.Time, error) {
	// Check the dxdir metadata for the root files directory
	dxPath := dxdir.RootPath
	health, err := sc.managedDirectoryMetadata(dxPath)
	if err != nil {
		return dxdir.EmptyPath, time.Time{}, err
	}

	// Find the lowest level directory that has a LastHealthCheckTime outside
	// the healthCheckInterval
	for time.Since(health.TimeLastHealthCheck) > HealthCheckInterval {
		// Check to make sure renter hasn't been shutdown
		select {
		case <-sc.tm.StopChan():
			return dxdir.EmptyPath, time.Time{}, err
		default:
		}

		// Check for sub directories
		subDirDxPaths, err := sc.managedSubDirectories(dxPath)
		if err != nil {
			return dxdir.EmptyPath, time.Time{}, err
		}
		// If there are no sub directories, return
		if len(subDirDxPaths) == 0 {
			return dxPath, health.TimeLastHealthCheck, nil
		}

		// Find the oldest LastHealthCheckTime of the sub directories
		updated := false
		for _, subDirPath := range subDirDxPaths {
			// Check lastHealthCheckTime of sub directory
			subHealth, err := sc.managedDirectoryMetadata(subDirPath)
			if err != nil {
				return dxdir.EmptyPath, time.Time{}, err
			}

			// If lastCheck is after current lastHealthCheckTime continue since
			// we are already in a directory with an older timestamp
			if subHealth.TimeLastHealthCheck.After(health.TimeLastHealthCheck) {
				continue
			}

			// Update lastHealthCheckTime and follow older path
			updated = true
			health.TimeLastHealthCheck = subHealth.TimeLastHealthCheck
			dxPath = subDirPath
		}

		// If the values were never updated with any of the sub directory values
		// then return as we are in the directory we are looking for
		if !updated {
			return dxPath, health.TimeLastHealthCheck, nil
		}
	}

	return dxPath, health.TimeLastHealthCheck, nil
}

// managedStuckDirectory randomly finds a directory that contains stuck Segments
func (sc *StorageClient) managedStuckDirectory() (dxdir.DxPath, error) {
	// Iterating of the renter direcotry until randomly ending up in a
	// directory, break and return that directory
	dxPath := dxdir.RootPath
	for {
		select {
		// Check to make sure renter hasn't been shutdown
		case <-sc.tm.StopChan():
			return dxdir.EmptyPath, nil
		default:
		}

		directories, files, err := sc.DirList(dxPath)
		if err != nil {
			return dxdir.EmptyPath, err
		}
		// Sanity check that there is at least the current directory
		if len(directories) == 0 {
			return dxdir.EmptyPath, ErrNoDirectory
		}
		// Check if we are in an empty Directory. This could happen if the only
		// file in a directory was stuck and was very recently deleted so the
		// health of the directory has not yet been updated.
		emptyDir := len(directories) == 1 && len(files) == 0
		if emptyDir {
			sc.log.Debug("WARN: empty directory found with stuck Segments:", dxPath)
			return dxPath, ErrNoStuckFiles
		}
		// Check if there are stuck Segments in this directory
		if directories[0].NumStuckSegments == 0 {
			// Log error if we are not at the root directory
			if !dxPath.IsRoot() {
				sc.log.Debug("WARN: ended up in directory with no stuck Segments that is not root directory:", dxPath)
			}
			return dxPath, ErrNoStuckFiles
		}
		// Check if we have reached a directory with only files
		if len(directories) == 1 {
			return dxPath, nil
		}

		// Get random int
		rand := rand.Intn(int(directories[0].NumStuckSegments))

		// Use rand to decide which directory to go into. Work backwards over
		// the slice of directories. Since the first element is the current
		// directory that means that it is the sum of all the files and
		// directories.  We can chose a directory by subtracting the number of
		// stuck Segments a directory has from rand and if rand gets to 0 or less
		// we choose that direcotry
		for i := len(directories) - 1; i >= 0; i-- {
			// If we make it to the last iteration double check that the current
			// directory has files
			if i == 0 && len(files) == 0 {
				break
			}

			// If we are on the last iteration and the directory does have files
			// then return the current directory
			if i == 0 {
				dxPath.LoadString(directories[0].DxPath.String())
				return dxPath, nil
			}

			// Skip directories with no stuck Segments
			if directories[i].NumStuckSegments == uint64(0) {
				continue
			}

			rand = rand - int(directories[i].NumStuckSegments)
			dxPath.LoadString(directories[i].DxPath.String())
			// If rand is less than 0 break out of the loop and continue into
			// that directory
			if rand <= 0 {
				break
			}
		}
	}
}

// managedSubDirectories reads a directory and returns a slice of all the sub
// directory DxPaths
func (sc *StorageClient) managedSubDirectories(dxPath dxdir.DxPath) ([]dxdir.DxPath, error) {
	// Read directory
	fileinfos, err := ioutil.ReadDir(dxPath.DxDirSysPath(sc.staticFilesDir))
	if err != nil {
		return nil, err
	}
	// Find all sub directory DxPaths
	folders := make([]dxdir.DxPath, 0, len(fileinfos))
	for _, fi := range fileinfos {
		if fi.IsDir() {
			subDir, err := dxPath.Join(fi.Name())
			if err != nil {
				return nil, err
			}
			folders = append(folders, subDir)
		}
	}
	return folders, nil
}

// managedWorstHealthDirectory follows the path of worst health to the lowest
// level possible
func (sc *StorageClient) managedWorstHealthDirectory() (dxdir.DxPath, float64, error) {
	// Check the health of the root files directory
	DxPath := dxdir.RootPath
	health, err := sc.managedDirectoryMetadata(DxPath)
	if err != nil {
		return dxdir.EmptyPath, 0, err
	}

	// Follow the path of worst health to the lowest level. We only want to find
	// directories with a health worse than the repairHealthThreshold to save
	// resources
	for float64(health.Health) >= RemoteRepairDownloadThreshold {
		// Check to make sure renter hasn't been shutdown
		select {
		case <-sc.tm.StopChan():
			return dxdir.EmptyPath, 0, errors.New("could not find worst health directory due to shutdown")
		default:
		}
		// Check for subdirectories
		subDirDxPaths, err := sc.managedSubDirectories(DxPath)
		if err != nil {
			return dxdir.EmptyPath, 0, err
		}
		// If there are no sub directories, return
		if len(subDirDxPaths) == 0 {
			return DxPath, float64(health.Health), nil
		}

		// Check sub directory healths to find the worst health
		updated := false
		for _, subDirPath := range subDirDxPaths {
			// Check health of sub directory
			subHealth, err := sc.managedDirectoryMetadata(subDirPath)
			if err != nil {
				return dxdir.EmptyPath, 0, err
			}

			// If the health of the sub directory is better than the current
			// worst health continue
			if subHealth.Health < health.Health {
				continue
			}

			// Update Health and worst health path
			updated = true
			health.Health = subHealth.Health
			DxPath = subDirPath
		}

		// If the values were never updated with any of the sub directory values
		// then return as we are in the directory we are looking for
		if !updated {
			return DxPath, float64(health.Health), nil
		}
	}

	return DxPath, float64(health.Health), nil
}

// threadedBubbleMetadata calculates the updated values of a directory's
// metadata and updates the dxdir metadata on disk then calls
// threadedBubbleMetadata on the parent directory
func (sc *StorageClient) threadedBubbleMetadata(dxPath dxdir.DxPath) {
	if err := sc.tm.Add(); err != nil {
		return
	}
	defer sc.tm.Done()

	// Check if bubble is needed
	needed, err := sc.managedBubbleNeeded(dxPath)
	if err != nil {
		sc.log.Debug("WARN: error in checking if bubble is needed:", err)
		return
	}
	if !needed {
		return
	}

	// Make sure we call threadedBubbleMetadata on the parent once we are done.
	defer func() {
		// Complete bubble
		err = sc.managedCompleteBubbleUpdate(dxPath)
		if err != nil {
			sc.log.Debug("WARN: error in completing bubble:", err)
			return
		}
		// Continue with parent dir if we aren't in the root dir already.
		if dxPath.IsRoot() {
			return
		}
		parentDir, err := dxPath.Dir()
		if err != nil {
			sc.log.Debug("WARN: Failed to defer threadedBubbleMetadata: %v", err)
			return
		}
		go sc.threadedBubbleMetadata(parentDir)
	}()

	// Calculate the new metadata values of the directory
	metadata, err := sc.managedCalculateDirectoryMetadata(dxPath)
	if err != nil {
		sc.log.Debug("WARN: Could not calculate the metadata of directory %v: %v\n", dxPath.DxDirSysPath(sc.staticFilesDir), err)
		return
	}

	// Update directory metadata with the health information. Don't return here
	// to avoid skipping the repairNeeded and stuckSegmentFound signals.
	dxDir, err := sc.staticDirSet.Open(dxPath)
	if err != nil {
		sc.log.Debug("WARN: Could not open directory %v: %v\n", dxPath.DxDirSysPath(sc.staticFilesDir), err)
	} else {
		defer dxDir.Close()
		err = dxDir.UpdateMetadata(metadata)
		if err != nil {
			sc.log.Debug("WARN: Could not update the metadata of the directory %v: %v\n", dxPath.DxDirSysPath(sc.staticFilesDir), err)
		}
	}

	// If DxPath is equal to "" then return as we are in the root files
	// directory of the renter
	if dxPath.IsRoot() {
		// If we are at the root directory then check if any files were found in
		// need of repair or and stuck Segments and trigger the appropriate repair
		// loop. This is only done at the root directory as the repair and stuck
		// loops start at the root directory so there is no point triggering
		// them until the root directory is updated
		if float64(metadata.Health) >= RemoteRepairDownloadThreshold {
			select {
			case sc.uploadHeap.repairNeeded <- struct{}{}:
			default:
			}
		}
		if metadata.NumStuckSegments > 0 {
			select {
			case sc.uploadHeap.stuckSegmentFound <- struct{}{}:
			default:
			}
		}
		return
	}
	return
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
				err := sc.managedAddStuckSegmentsToHeap(DxPath)
				if err != nil {
					sc.log.Debug("WARN: unable to add stuck Segments from file", DxPath, "to heap:", err)
				}
			}
			continue
		}

		// Refresh the worker pool and get the set of hosts that are currently
		// useful for uploading.
		hosts := sc.refreshHostsAndWorkers()

		// Add stuck Segment to upload heap
		sc.createSegmentHeap(dirDxPath, hosts, targetStuckSegments)

		sc.uploadLoop(hosts)

		// Call bubble once all Segments have been popped off heap
		sc.threadedBubbleMetadata(dirDxPath)

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
			err := sc.managedAddStuckSegmentsToHeap(DxPath)
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
		DxPath, lastHealthCheckTime, err := sc.managedOldestHealthCheckTime()
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
			sc.threadedBubbleMetadata(DxPath)
		}
	}
}
