// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"container/heap"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
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
	segmentComing       chan struct{}
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

func (client *StorageClient) createUnfinishedSegments(entry *dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) ([]*unfinishedUploadSegment, error) {
	ec, err := entry.ErasureCode()
	if err != nil {
		return nil, err
	}
	if len(client.workerPool) < int(ec.MinSectors()) {
		client.log.Info("cannot create any segment from file because there are not enough workers, so marked all unhealthy segments as stuck")

		var err error
		if err = entry.MarkAllUnhealthySegmentsAsStuck(hostHealthInfoTable); err != nil {
			client.log.Error("unable to mark all segments as stuck", "err", err)
		} else {
			err = errors.New("not enough storage contracts meets the minimum sectors")
		}
		return nil, err
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
		client.log.Info("no segment indices gathered, can't add segments to heap")
		return nil, nil
	}

	// Assemble the set of segments
	newUnfinishedSegments := make([]*unfinishedUploadSegment, len(segmentIndexes))
	for i, index := range segmentIndexes {
		// Sanity check: fileUID should not be the empty value.
		fid := entry.UID()
		if string(fid[:]) == "" {
			return nil, errors.New("entry fid is empty")
		}

		// Create unfinishedUploadSegment
		key, err := entry.CipherKey()
		if err != nil {
			return nil, fmt.Errorf("cannot create cipher: %v", err)
		}
		ec, err := entry.ErasureCode()
		if err != nil {
			return nil, fmt.Errorf("cannot create erasure code: %v", err)
		}
		newUnfinishedSegments[i] = &unfinishedUploadSegment{
			fileEntry: entry.CopyEntry(),

			id: uploadSegmentID{
				fid:   fid,
				index: uint64(index),
			},

			index:  uint64(index),
			length: entry.SegmentSize(),
			offset: int64(uint64(index) * entry.SegmentSize()),

			memoryNeeded:      entry.SectorSize()*uint64(ec.NumSectors()+ec.MinSectors()) + uint64(ec.NumSectors())*uint64(key.Overhead()),
			sectorsMinNeedNum: int(ec.MinSectors()),
			sectorsAllNeedNum: int(ec.NumSectors()),
			stuck:             entry.GetStuckByIndex(index),

			physicalSegmentData: make([][]byte, ec.NumSectors()),

			sectorSlotsStatus: make([]bool, ec.NumSectors()),
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
			client.log.Error("failed to get sectors for building incomplete segments", "err", err)
			return nil, err
		}
		for sectorIndex, sectorSet := range sectors {
			for _, sector := range sectorSet {
				contractID := client.contractManager.GetStorageContractSet().GetContractIDByHostID(sector.HostID)
				if meta, ok := client.contractManager.GetStorageContractSet().RetrieveContractMetaData(contractID); !ok || !meta.Status.RenewAbility {
					continue
				}

				// Mark the segment set based on the sectors in this contract
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
		isIncomplete := segment.sectorsCompletedNum < segment.sectorsAllNeedNum

		// Check if segment is downloadable
		segmentHealth := segment.fileEntry.SegmentHealth(int(segment.index), hostHealthInfoTable)
		_, err := os.Stat(string(segment.fileEntry.LocalPath()))
		downloadable := segmentHealth >= dxfile.StuckThreshold || err == nil

		// Check if segment seems stuck
		stuck := !isIncomplete && segmentHealth != dxfile.CompleteHealthThreshold

		// Add segment to list of incompleteSegments if it is isIncomplete and
		// downloadable or if we are targeting stuck segments
		if isIncomplete && (downloadable || target == targetStuckSegments) {
			incompleteSegments = append(incompleteSegments, segment)
			continue
		}

		// If a segment is not downloadable mark it as stuck
		// When the file upload does not reach the recoverable level,
		// the source file is deleted again and will be marked as stuck = true forever
		if !downloadable {
			client.log.Info("Marking segment", "ID", segment.id, "as stuck due to not being downloadable")
			err = segment.fileEntry.SetStuckByIndex(int(segment.index), true)
			if err != nil {
				client.log.Error("unable to mark segment as stuck", "err", err)
			}
			continue
		} else if stuck {
			client.log.Info("Marking segment", "ID", segment.id, "as stuck due to being complete but having a health of", segmentHealth)
			err = segment.fileEntry.SetStuckByIndex(int(segment.index), true)
			if err != nil {
				client.log.Error("unable to mark segment as stuck", "err", err)
			}
			continue
		}

		// Close entry of completed Segment
		err = client.setStuckAndClose(segment, false)
		if err != nil {
			client.log.Error("unable to mark segment as unstuck and close", "err", err)
		}
	}
	return incompleteSegments, nil
}

// Select a dxfile randomly and then grab one segment randomly in this file
func (client *StorageClient) createAndPushRandomSegment(files []*dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) {
	// Sanity check that there are files
	if len(files) == 0 {
		return
	}

	// Grab a random file
	rand.Seed(time.Now().UnixNano())
	randFileIndex := rand.Intn(len(files))
	file := files[randFileIndex]

	client.lock.Lock()
	// Build the unfinished stuck segments from the file
	unfinishedUploadSegments, _ := client.createUnfinishedSegments(file, hosts, target, hostHealthInfoTable)
	client.lock.Unlock()

	// Sanity check that there are stuck segments
	if len(unfinishedUploadSegments) == 0 {
		client.log.Info("no stuck unfinished upload segments returned")
		return
	}

	// Add a random stuck segment to the upload heap and set its stuckRepair field to true
	randSegmentIndex := rand.Intn(len(unfinishedUploadSegments))
	randSegment := unfinishedUploadSegments[randSegmentIndex]
	randSegment.stuckRepair = true

	// add segment to upload heap
	client.uploadHeap.push(randSegment)

	//unfinishedUploadSegments = append(unfinishedUploadSegments[:randSegmentIndex], unfinishedUploadSegments[randSegmentIndex+1:]...)
	//for _, segment := range unfinishedUploadSegments {
	//	err := segment.fileEntry.Close()
	//	if err != nil {
	//		client.log.Error("unable to close file", "err", err)
	//	}
	//}
	return
}

// createAndPushSegments creates the unfinished segments and push them to the upload heap
func (client *StorageClient) createAndPushSegments(files []*dxfile.FileSetEntryWithID, hosts map[string]struct{}, target uploadTarget, hostHealthInfoTable storage.HostHealthInfoTable) error {
	for _, file := range files {
		client.lock.Lock()
		unfinishedUploadSegments, err := client.createUnfinishedSegments(file, hosts, target, hostHealthInfoTable)
		if err != nil {
			client.lock.Unlock()
			return err
		}
		client.lock.Unlock()

		if len(unfinishedUploadSegments) == 0 {
			client.log.Info("no unfinished upload segments returned")
			continue
		}

		for i := 0; i < len(unfinishedUploadSegments); i++ {
			client.uploadHeap.push(unfinishedUploadSegments[i])
		}
	}
	return nil
}

// pushDirToSegmentHeap is charge of creating segment heap that worker tasks locate in
func (client *StorageClient) pushDirOrFileToSegmentHeap(dxPath storage.DxPath, dir bool, hosts map[string]struct{}, target uploadTarget) {
	// Get files of directory and sub directories
	var files []*dxfile.FileSetEntryWithID

	if !dir {
		if file, _ := client.openDxFile(dxPath, target); file != nil {
			files = append(files, file)
		}
	} else {
		fileInfos, err := ioutil.ReadDir(string(dxPath.SysPath(client.fileSystem.RootDir())))

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

			if file, _ := client.openDxFile(dxPath, target); file != nil {
				files = append(files, file)
			}
		}
	}

	// Check if any files were selected from directory
	if len(files) == 0 {
		client.log.Info("No files pulled to build the upload heap", "dxpath", dxPath)
		return
	}

	hostHealthInfoTable := client.contractManager.HostHealthMap()

	switch target {
	case targetStuckSegments:
		client.log.Info("Adding stuck segment to heap")
		client.createAndPushRandomSegment(files, hosts, target, hostHealthInfoTable)
	case targetUnstuckSegments:
		client.log.Info("Adding unstuck segments to heap")
		client.createAndPushSegments(files, hosts, target, hostHealthInfoTable)
	default:
		client.log.Info("target not recognized", "target", target)
	}
}

func (client *StorageClient) openDxFile(path storage.DxPath, target uploadTarget) (*dxfile.FileSetEntryWithID, error) {
	file, err := client.fileSystem.OpenDxFile(path)

	if err != nil {
		client.log.Error("Could not open dx file", "err", err)
		return nil, err
	}

	// For stuck segment repairs, check to see if file has stuck segments
	if target == targetStuckSegments && file.NumStuckSegments() == 0 {
		err := file.Close()
		if err != nil {
			client.log.Error("Could not close file", "err", err)
		}
		return nil, err
	}

	// For normal repairs, ignore files that don't have any unstuck segments
	if target == targetUnstuckSegments && file.NumSegments() == file.NumStuckSegments() {
		err := file.Close()
		if err != nil {
			client.log.Error("Could not close file", "err", err)
		}
		return nil, err
	}

	return file, nil
}

// doProcessNextSegment takes the next segment from the segment heap and prepares it for upload
func (client *StorageClient) doProcessNextSegment(uuc *unfinishedUploadSegment) error {
	// Block until there is enough memory, and then upload segment asynchronously
	if !client.memoryManager.Request(uuc.memoryNeeded, false) {
		return errors.New("can't obtain enough memory")
	}

	// Don't block the outer loop
	go client.retrieveDataAndDispatchSegment(uuc)
	return nil
}

// refreshHostsAndWorkers will reset the set of hosts and the set of
// workers for the storage client
func (client *StorageClient) refreshHostsAndWorkers() map[string]struct{} {
	currentContracts := client.contractManager.GetStorageContractSet().Contracts()

	hosts := make(map[string]struct{})
	for _, contract := range currentContracts {
		hosts[contract.Header().EnodeID.String()] = struct{}{}
	}

	// Refresh the worker pool
	client.activateWorkerPool()
	return hosts
}

// repairLoop works through the upload heap repairing segments. The repair
// loop will continue until the storage client stops, there are no more Segments, or
// enough time has passed indicated by the rebuildHeapSignal
func (client *StorageClient) uploadOrRepair() {
	var consecutiveSegmentUploads int
	for {
		select {
		case <-client.tm.StopChan():
			return
		case <-client.uploadHeap.segmentComing:
		}

	LOOP:
		if !(storage.ENV == storage.Env_Test) {
			// Return if not online.
			if !client.blockUntilOnline() {
				return
			}
		}

		// Pop the next segment and check whether is empty
		nextSegment := client.uploadHeap.pop()
		if nextSegment == nil {
			continue
		}

		// If the num of workers in worker pool is not enough to cover the tasks, we will
		// mark the segment as stuck
		client.lock.Lock()
		availableWorkers := len(client.workerPool)
		client.lock.Unlock()
		if availableWorkers < nextSegment.sectorsMinNeedNum {
			client.log.Info("Setting segment as stuck because there are not enough good workers", "segmentID", nextSegment.id)
			err := client.setStuckAndClose(nextSegment, true)
			if err != nil {
				client.log.Error("Unable to mark segment as stuck and close", "err", err)
			}
			goto LOOP
		}

		// doPrepareNextSegment block until enough memory of segment and then distribute it to the workers
		err := client.doProcessNextSegment(nextSegment)
		if err != nil {
			client.log.Error("Unable to prepare next segment without issues", "segmentID", nextSegment.id, "err", err)
			err = client.setStuckAndClose(nextSegment, true)
			if err != nil {
				client.log.Error("Unable to mark segment as stuck and close", "err", err)
			}
			goto LOOP
		}
		consecutiveSegmentUploads++

		// Check if enough segments are currently being repaired
		if consecutiveSegmentUploads >= MaxConsecutiveSegmentUploads {
			var stuckSegments []*unfinishedUploadSegment
			for client.uploadHeap.len() > 0 {
				if c := client.uploadHeap.pop(); c.stuck {
					stuckSegments = append(stuckSegments, c)
				}
			}
			for _, ss := range stuckSegments {
				client.uploadHeap.push(ss)
				//err := ss.fileEntry.Close()
				//if err != nil {
				//	client.log.Error("Unable to close file", "err", err)
				//}
			}
		}

		client.uploadHeap.mu.Lock()
		heapLen := client.uploadHeap.heap.Len()
		client.uploadHeap.mu.Unlock()
		if heapLen != 0 {
			goto LOOP
		}

	}
}

// doUploadAndRepair will find new uploads and existing files in need of
// repair and execute the uploads and repairs. This function effectively runs a
// single iteration of threadedUploadAndRepair.
func (client *StorageClient) doUpload() error {
	// Find the lowest health file to queue for repairs.
	dxFile, err := client.fileSystem.SelectDxFileToFix()
	if err != nil && err != filesystem.ErrNoRepairNeeded {
		return err
	}

	if err == filesystem.ErrNoRepairNeeded {
		return nil
	}

	// Refresh the worker pool and get the set of hosts that are currently
	// useful for uploading
	hosts := client.refreshHostsAndWorkers()

	// Push a min-heap of segments organized by upload progress
	// we don't worry about the dxfile nil problem. we have done it above
	client.pushDirOrFileToSegmentHeap(dxFile.DxPath(), false, hosts, targetUnstuckSegments)
	client.uploadHeap.mu.Lock()
	heapLen := client.uploadHeap.heap.Len()
	client.uploadHeap.mu.Unlock()
	if heapLen == 0 {
		return client.fileSystem.InitAndUpdateDirMetadata(dxFile.DxPath())
	}

	select {
	case client.uploadHeap.segmentComing <- struct{}{}:
	default:
	}

	// When we have worked through the heap, invoke update metadata to update
	return client.fileSystem.InitAndUpdateDirMetadata(dxFile.DxPath())
}

func (client *StorageClient) uploadLoop() {
	err := client.tm.Add()
	if err != nil {
		return
	}
	defer client.tm.Done()

	for {
		// Wait for client online
		if !client.blockUntilOnline() {
			return
		}

		// Check whether a repair is needed of root dir. If the root dir health is more than
		// RepairHealthThreshold, it is not necessary to upload any sectors
		rootMetadata, err := client.dirMetadata(storage.RootDxPath())
		if err != nil {
			// If there is an error fetching the root directory metadata, sleep
			// for a bit and hope that on the next iteration, things will be better
			select {
			case <-time.After(UploadAndRepairErrorSleepDuration):
			case <-client.tm.StopChan():
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
			case <-client.fileSystem.RepairNeededChan():
			case <-client.tm.StopChan():
				return
			}
			continue
		}

		// Last we call doUpload to complete upload task
		err = client.doUpload()
		if err != nil {
			select {
			case <-time.After(UploadAndRepairErrorSleepDuration):
			case <-client.tm.StopChan():
				return
			}
		}
		<-time.After(100 * time.Millisecond)
	}
}
