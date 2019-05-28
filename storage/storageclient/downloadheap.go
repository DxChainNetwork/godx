// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"container/heap"
	"time"

	"github.com/DxChainNetwork/godx/log"
)

// downloadSegmentHeap is a heap that is sorted first by file priority, then by
// the start time of the download, and finally by the index of the segment.  As
// downloads are queued, they are added to the downloadSegmentHeap. As resources
// become available to execute downloads, segments are pulled off of the heap and
// distributed to workers.
type downloadSegmentHeap []*unfinishedDownloadSegment

func (dch downloadSegmentHeap) Len() int {
	return len(dch)
}

func (dch downloadSegmentHeap) Less(i, j int) bool {

	// First sort by priority.
	if dch[i].staticPriority != dch[j].staticPriority {
		return dch[i].staticPriority > dch[j].staticPriority
	}

	// For equal priority, sort by start time.
	if dch[i].download.staticStartTime != dch[j].download.staticStartTime {
		return dch[i].download.staticStartTime.Before(dch[j].download.staticStartTime)
	}

	// For equal start time, sort by segmentIndex.
	return dch[i].staticSegmentIndex < dch[j].staticSegmentIndex
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

// downloadLoop utilizes the worker pool to make progress on any queued
// downloads.
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

// nextDownloadSegment will fetch the next segment from the download heap
func (c *StorageClient) nextDownloadSegment() *unfinishedDownloadSegment {
	c.downloadHeapMu.Lock()
	defer c.downloadHeapMu.Unlock()

	for {
		if c.downloadHeap.Len() <= 0 {
			return nil
		}
		nextSegment := heap.Pop(c.downloadHeap).(*unfinishedDownloadSegment)
		if !nextSegment.download.staticComplete() {
			return nextSegment
		}
	}
}

// acquireMemoryForDownloadSegment will block until memory is available for the
// segment to be downloaded.
func (c *StorageClient) acquireMemoryForDownloadSegment(uds *unfinishedDownloadSegment) bool {

	// TODO: 确认下erasure code算法是否已经优化。
	//  按照Sia描述，这里实际上要求的内存空间是：擦除🐎恢复文件需要的最小文件量 + 设定的过载空间量
	// the amount of memory required is equal minimum number of sectors plus the overdrive amount.
	memoryRequired := uint64(uds.staticOverdrive+uds.erasureCode.MinSectors()) * uds.staticSectorSize
	uds.memoryAllocated = memoryRequired
	return c.memoryManager.Request(memoryRequired, true)
}

// distributeDownloadSegmentToWorkers will take a segment and pass it out to all of the workers.
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

// addSegmentToDownloadHeap will add a segment to the download heap
func (c *StorageClient) addSegmentToDownloadHeap(uds *unfinishedDownloadSegment) {

	// the sole purpose of the heap is to block workers from receiving a segment until memory has been allocated
	if !uds.staticNeedsMemory {
		c.distributeDownloadSegmentToWorkers(uds)
		return
	}

	// put the segment into the segment heap.
	c.downloadHeapMu.Lock()
	c.downloadHeap.Push(uds)
	c.downloadHeapMu.Unlock()
}
