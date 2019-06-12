// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"container/heap"
	"errors"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type uploadTarget int

// targetStuckSegments   indicates the repair loop to retrieve stuck segments for repair loop and
// targetUnstuckSegments indicates the upload loop to target unstuck segments for upload
const (
	targetError uploadTarget = iota
	targetStuckSegments
	targetUnstuckSegments
)

// uploadSegmentHeap is a min-heap of priority-sorted segments that need to be either uploaded or repaired
// The rules of priority:
//   1) stuck first
//   2) the lower completion percentage, the more forward when they have the same stuck status
type uploadSegmentHeap []*unfinishedUploadSegment

func (uch uploadSegmentHeap) Len() int { return len(uch) }
func (uch uploadSegmentHeap) Less(i, j int) bool {
	if uch[i].stuck == uch[j].stuck {
		return float64(uch[i].sectorsCompletedNum)/float64(uch[i].sectorsAllNeedNum) < float64(uch[j].sectorsCompletedNum)/float64(uch[j].sectorsAllNeedNum)
	}

	if uch[i].stuck {
		return true
	}

	return false
}
func (uch uploadSegmentHeap) Swap(i, j int)       { uch[i], uch[j] = uch[j], uch[i] }
func (uch *uploadSegmentHeap) Push(x interface{}) { *uch = append(*uch, x.(*unfinishedUploadSegment)) }
func (uch *uploadSegmentHeap) Pop() interface{} {
	old := *uch
	n := len(old)
	x := old[n-1]
	*uch = old[0 : n-1]
	return x
}

// uploadHeap is a wrapper heap of uploadSegmentHeap that contains all control chan to the storage client
type uploadHeap struct {
	heap uploadSegmentHeap

	// pendingSegments is a map containing all the segments are that currently
	// assigned to workers and are being repaired or uploaded
	pendingSegments map[uploadSegmentID]struct{}

	// Control channels
	newUploads          chan struct{}
	repairNeeded        chan struct{}
	stuckSegmentFound   chan struct{}
	stuckSegmentSuccess chan storage.DxPath

	mu sync.Mutex
}

func (uh *uploadHeap) len() int {
	uh.mu.Lock()
	uhLen := uh.heap.Len()
	uh.mu.Unlock()
	return uhLen
}

func (uh *uploadHeap) push(uuc *unfinishedUploadSegment) bool {
	var added bool
	uh.mu.Lock()
	_, exists := uh.pendingSegments[uuc.id]
	if !exists {
		uh.pendingSegments[uuc.id] = struct{}{}
		heap.Push(&uh.heap, uuc)
		added = true
	}
	uh.mu.Unlock()
	return added
}

func (uh *uploadHeap) pop() (uc *unfinishedUploadSegment) {
	uh.mu.Lock()
	if len(uh.heap) > 0 {
		uc = heap.Pop(&uh.heap).(*unfinishedUploadSegment)
		delete(uh.pendingSegments, uc.id)
	}
	uh.mu.Unlock()
	return uc
}

func (sc *StorageClient) createUnfinishedSegments(entry *dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) []*unfinishedUploadSegment {
	if len(sc.workerPool) < int(entry.ErasureCode().MinSectors()) {
		sc.log.Debug("cannot create any segment from file because there are not enough workers, so marked all unhealthy segments as stuck")
		if err := entry.MarkAllUnhealthySegmentsAsStuck(hostHealthInfoTable); err != nil {
			sc.log.Debug("unable to mark all segments as stuck:", err)
		}
		return nil
	}

	// Assemble segment indexes, stuck loop should only be adding stuck segments and
	// the repair loop should only be adding unstuck segments
	var segmentIndexes []int
	for i := 0; i < entry.NumSegments(); i++ {
		if (target == targetStuckSegments) == entry.GetStuckByIndex(i) {
			segmentIndexes = append(segmentIndexes, i)
		}
	}

	// Sanity check that we have segment indices to go through
	if len(segmentIndexes) == 0 {
		sc.log.Debug("no segment indices gathered, can't add segments to heap")
		return nil
	}

	// Assemble the set of segments
	newUnfinishedSegments := make([]*unfinishedUploadSegment, len(segmentIndexes))
	for i, index := range segmentIndexes {
		// Sanity check: fileUID should not be the empty value.
		fid := entry.UID()
		if string(fid[:]) == "" {
			return nil
		}

		// Create unfinishedUploadSegment
		newUnfinishedSegments[i] = &unfinishedUploadSegment{
			fileEntry: entry.CopyEntry(),

			id: uploadSegmentID{
				fid:   fid,
				index: uint64(index),
			},

			index:  uint64(index),
			length: entry.SegmentSize(),
			offset: int64(uint64(index) * entry.SegmentSize()),

			memoryNeeded:      entry.SectorSize()*uint64(entry.ErasureCode().NumSectors()+entry.ErasureCode().MinSectors()) + uint64(entry.ErasureCode().NumSectors())*uint64(entry.CipherKey().Overhead()),
			sectorsMinNeedNum: int(entry.ErasureCode().MinSectors()),
			sectorsAllNeedNum: int(entry.ErasureCode().NumSectors()),
			stuck:             entry.GetStuckByIndex(index),

			physicalSegmentData: make([][]byte, entry.ErasureCode().NumSectors()),

			sectorSlotsStatus: make([]bool, entry.ErasureCode().NumSectors()),
			unusedHosts:       make(map[string]struct{}),
		}

		// Every Segment can have a different set of unused hosts.
		for host := range hosts {
			newUnfinishedSegments[i].unusedHosts[host] = struct{}{}
		}
	}

	// Iterate through the sectors of all segments of the file and mark which
	// hosts are already in use for a particular Segment. As you delete hosts
	// from the 'unusedHosts' map, also increment the 'sectorsCompletedNum' value.
	for i, index := range segmentIndexes {
		sectors, err := entry.Sectors(index)
		if err != nil {
			sc.log.Debug("failed to get sectors for building incomplete Segments")
			return nil
		}
		for sectorIndex, sectorSet := range sectors {
			for _, sector := range sectorSet {
				// Get the contract for the sector
				// TODO get contractUtility from hostmanager
				// contractUtility, exists := sc.storageHostManager.ContractUtility(sector.HostID)
				//if !exists {
				//	// File contract does not seem to be part of the host anymore.
				//	continue
				//}

				contractUtility := storage.ContractUtility{}
				if !contractUtility.GoodForRenew {
					continue
				}

				// Mark the Segment set based on the pieces in this contract
				_, exists := newUnfinishedSegments[i].unusedHosts[sector.HostID.String()]
				redundantSector := newUnfinishedSegments[i].sectorSlotsStatus[sectorIndex]
				if exists && !redundantSector {
					newUnfinishedSegments[i].sectorSlotsStatus[sectorIndex] = true
					newUnfinishedSegments[i].sectorsCompletedNum++
					delete(newUnfinishedSegments[i].unusedHosts, sector.HostID.String())
				} else if exists {
					delete(newUnfinishedSegments[i].unusedHosts, sector.HostID.String())
				}
			}
		}
	}

	// Iterate through the set of newUnfinishedSegments and remove any that are
	// completed or are not downloadable.
	incompleteSegments := newUnfinishedSegments[:0]
	for _, segment := range newUnfinishedSegments {
		// Check if segment is complete
		incomplete := segment.sectorsCompletedNum < segment.sectorsAllNeedNum

		// Check if segment is downloadable
		segmentHealth := segment.fileEntry.SegmentHealth(int(segment.index), hostHealthInfoTable)
		_, err := os.Stat(string(segment.fileEntry.LocalPath()))
		downloadable := segmentHealth >= dxfile.UnstuckHealthThreshold || err == nil

		// Check if segment seems stuck
		stuck := !incomplete && segmentHealth != dxfile.CompleteHealthThreshold

		// Add segment to list of incompleteSegments if it is incomplete and
		// downloadable or if we are targeting stuck segments
		if incomplete && (downloadable || target == targetStuckSegments) {
			incompleteSegments = append(incompleteSegments, segment)
			continue
		}

		// If a segment is not downloadable mark it as stuck
		// When the file upload does not reach the recoverable level,
		// the source file is deleted again and will be marked as stuck = true forever
		if !downloadable {
			sc.log.Info("Marking segment", segment.id, "as stuck due to not being downloadable")
			err = segment.fileEntry.SetStuckByIndex(int(segment.index), true)
			if err != nil {
				sc.log.Debug("unable to mark segment as stuck:", err)
			}
			continue
		} else if stuck {
			sc.log.Info("Marking segment", segment.id, "as stuck due to being complete but having a health of", segmentHealth)
			err = segment.fileEntry.SetStuckByIndex(int(segment.index), true)
			if err != nil {
				sc.log.Debug("unable to mark segment as stuck:", err)
			}
			continue
		}

		// Close entry of completed Segment
		err = sc.setStuckAndClose(segment, false)
		if err != nil {
			sc.log.Debug("unable to mark Segment as stuck and close:", err)
		}
	}
	return incompleteSegments
}

// Select a dxfile randomly and then grab one segment randomly in this file
func (sc *StorageClient) createAndPushRandomSegment(files []*dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) {
	// Sanity check that there are files
	if len(files) == 0 {
		return
	}

	// Grab a random file
	randFileIndex := rand.Intn(len(files))
	file := files[randFileIndex]
	sc.lock.Lock()

	// Build the unfinished stuck segments from the file
	unfinishedUploadSegments := sc.createUnfinishedSegments(file, hosts, target, hostHealthInfoTable)
	sc.lock.Unlock()

	// Sanity check that there are stuck segments
	if len(unfinishedUploadSegments) == 0 {
		sc.log.Debug("no stuck unfinishedUploadSegments returned from buildUnfinishedSegments, so no stuck Segments will be added to the heap")
		return
	}

	// Add a random stuck segment to the upload heap and set its stuckRepair field to true
	randSegmentIndex := rand.Intn(len(unfinishedUploadSegments))
	randSegment := unfinishedUploadSegments[randSegmentIndex]
	randSegment.stuckRepair = true
	if !sc.uploadHeap.push(randSegment) {
		// Segment wasn't added to the heap. Close the file
		err := randSegment.fileEntry.Close()
		if err != nil {
			sc.log.Debug("unable to close file:", err)
		}
	}
	// Close the unused unfinishedUploadSegments
	unfinishedUploadSegments = append(unfinishedUploadSegments[:randSegmentIndex], unfinishedUploadSegments[randSegmentIndex+1:]...)
	for _, segment := range unfinishedUploadSegments {
		err := segment.fileEntry.Close()
		if err != nil {
			sc.log.Debug("unable to close file:", err)
		}
	}
	return
}

// createAndPushSegments creates the unfinished segments and push them to the upload heap
func (sc *StorageClient) createAndPushSegments(files []*dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) {
	for _, file := range files {
		sc.lock.Lock()
		unfinishedUploadSegments := sc.createUnfinishedSegments(file, hosts, target, hostHealthInfoTable)
		sc.lock.Unlock()
		if len(unfinishedUploadSegments) == 0 {
			sc.log.Debug("No unfinishedUploadSegments returned from buildUnfinishedSegments, so no Segments will be added to the heap")
			continue
		}
		for i := 0; i < len(unfinishedUploadSegments); i++ {
			if !sc.uploadHeap.push(unfinishedUploadSegments[i]) {
				err := unfinishedUploadSegments[i].fileEntry.Close()
				if err != nil {
					sc.log.Debug("unable to close file:", err)
				}
			}
		}
	}
}

// pushDirToSegmentHeap is charge of creating segment heap that worker tasks locate in
func (sc *StorageClient) pushDirOrFileToSegmentHeap(dxPath storage.DxPath, dir bool, hosts map[string]struct{}, target uploadTarget) {
	// Get files of directory and sub directories
	var files []*dxfile.FileSetEntryWithID

	if !dir {
		if file, _ := sc.openDxFile(dxPath, target); file != nil {
			files = append(files, file)
		}
	} else {
		fileInfos, err := ioutil.ReadDir(string(dxPath.SysPath(sc.fileSystem.FileRootDir())))
		if err != nil {
			return
		}
		for _, fi := range fileInfos {
			// skip sub directories and non dxFiles
			ext := filepath.Ext(fi.Name())
			if fi.IsDir() || ext != storage.DxFileExt {
				continue
			}

			// Open DxFile
			dxPath, err := dxPath.Join(strings.TrimSuffix(fi.Name(), ext))
			if err != nil {
				return
			}

			if file, _ := sc.openDxFile(dxPath, target); file != nil {
				files = append(files, file)
			}
		}
	}

	// Check if any files were selected from directory
	if len(files) == 0 {
		sc.log.Debug("No files pulled from `", dxPath, "` to build the repair heap")
		return
	}

	// Build the unfinished upload Segments and add them to the upload heap
	// TODO - offline, goodForRenew, _ := sc.managedContractUtilityMaps()
	nilHostHealthInfoTable := make(storage.HostHealthInfoTable)

	switch target {
	case targetStuckSegments:
		sc.log.Debug("Adding stuck segment to heap")
		sc.createAndPushRandomSegment(files, hosts, target, nilHostHealthInfoTable)
	case targetUnstuckSegments:
		sc.log.Debug("Adding unstuck segments to heap")
		sc.createAndPushSegments(files, hosts, target, nilHostHealthInfoTable)
	default:
		sc.log.Debug("repair target not recognized", target)
	}

	// Close all files
	for _, file := range files {
		err := file.Close()
		if err != nil {
			sc.log.Debug("Could not close file:", err)
		}
	}
}

func (sc *StorageClient) openDxFile(path storage.DxPath, target uploadTarget) (*dxfile.FileSetEntryWithID, error) {
	file, err := sc.fileSystem.OpenFile(path)
	if err != nil {
		sc.log.Debug("could not open dx file:", err)
		return nil, err
	}

	// For stuck segment repairs, check to see if file has stuck segments
	if target == targetStuckSegments && file.NumStuckSegments() == 0 {
		err := file.Close()
		if err != nil {
			sc.log.Debug("Could not close file:", err)
		}
		return nil, err
	}

	// For normal repairs, ignore files that don't have any unstuck segments
	if target == targetUnstuckSegments && file.NumSegments() == file.NumStuckSegments() {
		err := file.Close()
		if err != nil {
			sc.log.Debug("Could not close file:", err)
		}
		return nil, err
	}

	return file, nil
}

// doProcessNextSegment takes the next segment from the segment heap and prepares it for upload
func (sc *StorageClient) doProcessNextSegment(uuc *unfinishedUploadSegment) error {
	// Block until there is enough memory, and then upload segment asynchronously
	if !sc.memoryManager.Request(uuc.memoryNeeded, false) {
		return errors.New("can't obtain enough memory")
	}

	// Don't block the outer loop
	go sc.retrieveDataAndDispatchSegment(uuc)
	return nil
}

// refreshHostsAndWorkers will reset the set of hosts and the set of
// workers for the storage client
func (sc *StorageClient) refreshHostsAndWorkers() map[string]struct{} {
	currentContracts := sc.storageHostManager.GetStorageContractSet().Contracts()
	hosts := make(map[string]struct{})
	for _, contract := range currentContracts {
		hosts[contract.Header().EnodeID.String()] = struct{}{}
	}

	// Refresh the worker pool
	sc.activateWorkerPool()
	return hosts
}

// repairLoop works through the upload heap repairing segments. The repair
// loop will continue until the storage client stops, there are no more Segments, or
// enough time has passed indicated by the rebuildHeapSignal
func (sc *StorageClient) uploadAndRepair(hosts map[string]struct{}) {
	var consecutiveSegmentUploads int
	rebuildHeapSignal := time.After(RebuildSegmentHeapInterval)
	for {
		select {
		case <-sc.tm.StopChan():
			return
		case <-rebuildHeapSignal:
			return
		default:
		}

		// Return if not online.
		if !sc.Online() {
			return
		}

		// Pop the next segment and check whether is empty
		nextSegment := sc.uploadHeap.pop()
		if nextSegment == nil {
			return
		}

		sc.log.Debug("Sending next segment to the workers", nextSegment.id)
		// If the num of workers in worker pool is not enough to cover the tasks, we will
		// mark the segment as stuck
		sc.lock.Lock()
		availableWorkers := len(sc.workerPool)
		sc.lock.Unlock()
		if availableWorkers < nextSegment.sectorsMinNeedNum {
			sc.log.Debug("Setting segment as stuck because there are not enough good workers", nextSegment.id)
			err := sc.setStuckAndClose(nextSegment, true)
			if err != nil {
				sc.log.Debug("Unable to mark segment as stuck and close:", err)
			}
			continue
		}

		// doPrepareNextSegment block until enough memory of segment and then distribute it to the workers
		err := sc.doProcessNextSegment(nextSegment)
		if err != nil {
			sc.log.Debug("Unable to prepare next segment without issues", err, nextSegment.id)
			err = sc.setStuckAndClose(nextSegment, true)
			if err != nil {
				sc.log.Debug("Unable to mark segment as stuck and close:", err)
			}
			continue
		}
		consecutiveSegmentUploads++

		// Check if enough segments are currently being repaired
		if consecutiveSegmentUploads >= MaxConsecutiveSegmentUploads {
			var stuckSegments []*unfinishedUploadSegment
			for sc.uploadHeap.len() > 0 {
				if c := sc.uploadHeap.pop(); c.stuck {
					stuckSegments = append(stuckSegments, c)
				}
			}
			for _, ss := range stuckSegments {
				if !sc.uploadHeap.push(ss) {
					err := ss.fileEntry.Close()
					if err != nil {
						sc.log.Debug("Unable to close file:", err)
					}
				}
			}
			return
		}
	}
}

// doUploadAndRepair will find new uploads and existing files in need of
// repair and execute the uploads and repairs. This function effectively runs a
// single iteration of threadedUploadAndRepair.
func (sc *StorageClient) doUpload() error {
	// Find the lowest health file to queue for repairs.
	dxFile, err := sc.fileSystem.SelectDxFileToFix()
	if err != nil {
		sc.log.Debug("error getting worst health dxfile:", err)
		return err
	}

	// Refresh the worker pool and get the set of hosts that are currently
	// useful for uploading
	hosts := sc.refreshHostsAndWorkers()

	// Push a min-heap of segments organized by upload progress
	sc.pushDirOrFileToSegmentHeap(dxFile.DxPath(), false, hosts, targetUnstuckSegments)
	sc.uploadHeap.mu.Lock()
	heapLen := sc.uploadHeap.heap.Len()
	sc.uploadHeap.mu.Unlock()
	if heapLen == 0 {
		sc.log.Debug("No segments added to the heap for repair from `%v` even through health was %v", dxFile.DxPath(), dxFile.GetHealth())
		return sc.fileSystem.InitAndUpdateDirMetadata(dxFile.DxPath())
	}
	sc.log.Info("Repairing", heapLen, "Segments from", dxFile.DxPath())

	// Work through the heap and repair files
	sc.uploadAndRepair(hosts)

	// When we have worked through the heap, invoke update metadata to update
	return sc.fileSystem.InitAndUpdateDirMetadata(dxFile.DxPath())
}

func (sc *StorageClient) uploadLoop() {
	err := sc.tm.Add()
	if err != nil {
		return
	}
	defer sc.tm.Done()

	for {
		select {
		case <-sc.tm.StopChan():
			return
		default:
		}

		// Wait for client online
		if !sc.blockUntilOnline() {
			return
		}

		// Check whether a repair is needed of root dir. If the root dir health is more than
		// RepairHealthThreshold, it is not necessary to upload any sectors
		rootMetadata, err := sc.managedDirectoryMetadata(storage.RootDxPath())
		if err != nil {
			// If there is an error fetching the root directory metadata, sleep
			// for a bit and hope that on the next iteration, things will be better
			sc.log.Debug("error fetching filesystem root metadata:", err)
			select {
			case <-time.After(UploadAndRepairErrorSleepDuration):
			case <-sc.tm.StopChan():
				return
			}
			continue
		}

		// It is not necessary to upload or repair immediately because of enough health score
		if rootMetadata.Health >= dxfile.RepairHealthThreshold {
			// Block until a signal is received that there is more work to do.
			// newUploads - Upload console api
			// repairNeeded - stuck loop
			select {
			case <-sc.uploadHeap.newUploads:
			case <-sc.uploadHeap.repairNeeded:
			case <-sc.tm.StopChan():
				return
			}
			continue
		}

		// Last we call doUpload to complete upload task
		err = sc.doUpload()
		if err != nil {
			sc.log.Debug("error performing upload and repair iteration:", err)
			select {
			case <-time.After(UploadAndRepairErrorSleepDuration):
			case <-sc.tm.StopChan():
				return
			}
		}

		// TODO: This sleep is a hack to keep the CPU from spinning at 100% for
		// a brief time when all of the Segments in the directory have been added
		// to the repair loop, but the directory isn't full yet so it keeps
		// trying to add more Segments ???
		time.Sleep(20 * time.Millisecond)
	}
}
