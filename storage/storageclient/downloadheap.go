// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"container/heap"
	"time"

	"github.com/DxChainNetwork/godx/log"
)

// download tasks are added to the downloadSegmentHeap.
// As resources become available to execute downloads,
// segments are pulled off of the heap and distributed to workers.
type downloadSegmentHeap []*unfinishedDownloadSegment

func (dch downloadSegmentHeap) Len() int {
	return len(dch)
}

func (dch downloadSegmentHeap) Less(i, j int) bool {

	// sort by priority.
	if dch[i].priority != dch[j].priority {
		return dch[i].priority > dch[j].priority
	}

	// if equal above then sort by start time.
	if dch[i].download.startTime != dch[j].download.startTime {
		return dch[i].download.startTime.Before(dch[j].download.startTime)
	}

	// if equal above then sort by segmentIndex.
	return dch[i].segmentIndex < dch[j].segmentIndex
}

func (dch downloadSegmentHeap) Swap(i, j int) {
	dch[i], dch[j] = dch[j], dch[i]
}

func (dch *downloadSegmentHeap) Push(x interface{}) {
	*dch = append(*dch, x.(*unfinishedDownloadSegment))
}

func (dch *downloadSegmentHeap) Pop() interface{} {
	old := *dch
	n := len(old)
	x := old[n-1]
	*dch = old[0 : n-1]
	return x
}

// downloadLoop utilizes the worker pool to make progress on any queued downloads.
func (client *StorageClient) downloadLoop() {
	err := client.tm.Add()
	if err != nil {
		log.Error("storage client thread manager failed to add in downloadLoop", "error", err)
		return
	}
	defer client.tm.Done()

	// infinite loop to process downloads, return if get stopping signal.
LOOP:
	for {
		// wait until the client is online.
		if !client.blockUntilOnline() {
			return
		}

		client.activateWorkerPool()
		workerActivateTime := time.Now()

		// pull downloads out of the heap
		for {

			// if client is offline, or timeout for activating worker pool, will reset to loop
			if !client.Online() || time.Now().After(workerActivateTime.Add(WorkerActivateTimeout)) {
				continue LOOP
			}

			// get the next segment.
			nextSegment := client.nextDownloadSegment()
			if nextSegment == nil {
				break
			}

			// get the required memory to download this segment.
			if !client.acquireMemoryForDownloadSegment(nextSegment) {
				return
			}

			// distribute the segment to workers.
			client.distributeDownloadSegmentToWorkers(nextSegment)
		}

		// wait for more work.
		select {
		case <-client.tm.StopChan():
			return
		case <-client.newDownloads:
		}
	}
}

func (client *StorageClient) blockUntilOnline() bool {
	for !client.Online() {
		select {
		case <-client.tm.StopChan():
			return false
		case <-time.After(OnlineCheckFrequency):
		}
	}
	return true
}

// fetch the next segment from the download heap
func (client *StorageClient) nextDownloadSegment() *unfinishedDownloadSegment {
	client.downloadHeapMu.Lock()
	defer client.downloadHeapMu.Unlock()

	for {
		if client.downloadHeap.Len() <= 0 {
			return nil
		}
		nextSegment := heap.Pop(client.downloadHeap).(*unfinishedDownloadSegment)
		if !nextSegment.download.isComplete() {
			return nextSegment
		}
	}
}

// Request memory to download segment, will block until memory is available
func (client *StorageClient) acquireMemoryForDownloadSegment(uds *unfinishedDownloadSegment) bool {

	// the amount of memory required is equal minimum number of sectors plus the overdrive amount.
	memoryRequired := uint64(uds.overdrive+uds.erasureCode.MinSectors()) * uds.sectorSize
	uds.memoryAllocated = memoryRequired
	return client.memoryManager.Request(memoryRequired, true)
}

// Pass a segment out to all of the workers.
func (client *StorageClient) distributeDownloadSegmentToWorkers(uds *unfinishedDownloadSegment) {

	// distribute the segment to workers, marking the number of workers that have received the work.
	client.lock.Lock()
	uds.mu.Lock()
	uds.workersRemaining = uint32(len(client.workerPool))
	uds.mu.Unlock()
	for _, worker := range client.workerPool {
		worker.queueDownloadSegment(uds)
	}
	client.lock.Unlock()

	// if there are no workers, there will be no workers to attempt to clean up
	// the segment, so we must make sure that cleanUp is called at least once on the segment.
	uds.cleanUp()
}

// Add a segment to the download heap
func (client *StorageClient) addSegmentToDownloadHeap(uds *unfinishedDownloadSegment) {

	// the sole purpose of the heap is to block workers from receiving a segment until memory has been allocated
	if !uds.needsMemory {
		client.distributeDownloadSegmentToWorkers(uds)
		return
	}

	// put the segment into the segment heap.
	client.downloadHeapMu.Lock()
	client.downloadHeap.Push(uds)
	client.downloadHeapMu.Unlock()
}
