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

	// First sort by priority.
	if dch[i].priority != dch[j].priority {
		return dch[i].priority > dch[j].priority
	}

	// For equal priority, sort by start time.
	if dch[i].download.startTime != dch[j].download.startTime {
		return dch[i].download.startTime.Before(dch[j].download.startTime)
	}

	// For equal start time, sort by segmentIndex.
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
func (c *StorageClient) downloadLoop() {
	err := c.tm.Add()
	if err != nil {
		log.Error("storage client thread manager failed to add in downloadLoop", "error", err)
		return
	}
	defer c.tm.Done()

	// infinite loop to process downloads. Will return if get stopping signal.
LOOP:
	for {
		// wait until the client is online.
		if !c.blockUntilOnline() {
			return
		}

		c.activateWorkerPool()
		workerActivateTime := time.Now()

		// pull downloads out of the heap
		for {

			// if client is offline, or timeout for activating worker pool, will reset to loop
			if !c.Online() || time.Now().After(workerActivateTime.Add(WorkerActivateTimeout)) {
				continue LOOP
			}

			// get the next segment.
			nextSegment := c.nextDownloadSegment()
			if nextSegment == nil {
				break
			}

			// get the required memory to download this segment.
			if !c.acquireMemoryForDownloadSegment(nextSegment) {
				return
			}

			// distribute the segment to workers.
			c.distributeDownloadSegmentToWorkers(nextSegment)
		}

		// wait for more work.
		select {
		case <-c.tm.StopChan():
			return
		case <-c.newDownloads:
		}
	}
}

func (c *StorageClient) blockUntilOnline() bool {
	for !c.Online() {
		select {
		case <-c.tm.StopChan():
			return false
		case <-time.After(OnlineCheckFrequency):
		}
	}
	return true
}

// fetch the next segment from the download heap
func (c *StorageClient) nextDownloadSegment() *unfinishedDownloadSegment {
	c.downloadHeapMu.Lock()
	defer c.downloadHeapMu.Unlock()

	for {
		if c.downloadHeap.Len() <= 0 {
			return nil
		}
		nextSegment := heap.Pop(c.downloadHeap).(*unfinishedDownloadSegment)
		if !nextSegment.download.isComplete() {
			return nextSegment
		}
	}
}

// request memory to download segment, will block until memory is available
func (c *StorageClient) acquireMemoryForDownloadSegment(uds *unfinishedDownloadSegment) bool {

	// the amount of memory required is equal minimum number of sectors plus the overdrive amount.
	memoryRequired := uint64(uds.overdrive+uds.erasureCode.MinSectors()) * uds.sectorSize
	uds.memoryAllocated = memoryRequired
	return c.memoryManager.Request(memoryRequired, true)
}

// pass a segment out to all of the workers.
func (c *StorageClient) distributeDownloadSegmentToWorkers(uds *unfinishedDownloadSegment) {

	// distribute the segment to workers, marking the number of workers that have received the work.
	c.lock.Lock()
	uds.mu.Lock()
	uds.workersRemaining = uint32(len(c.workerPool))
	uds.mu.Unlock()
	for _, worker := range c.workerPool {
		worker.queueDownloadSegment(uds)
	}
	c.lock.Unlock()

	// if there are no workers, there will be no workers to attempt to clean up
	// the segment, so we must make sure that cleanUp is called at least once on the segment.
	uds.cleanUp()
}

// add a segment to the download heap
func (c *StorageClient) addSegmentToDownloadHeap(uds *unfinishedDownloadSegment) {

	// the sole purpose of the heap is to block workers from receiving a segment until memory has been allocated
	if !uds.needsMemory {
		c.distributeDownloadSegmentToWorkers(uds)
		return
	}

	// put the segment into the segment heap.
	c.downloadHeapMu.Lock()
	c.downloadHeap.Push(uds)
	c.downloadHeapMu.Unlock()
}
