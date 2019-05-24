// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"crypto/ecdsa"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// A worker listens for work on a certain host.
type worker struct {

	// The contract and host used by this worker.
	contract   storage.ClientContract
	hostPubKey *ecdsa.PublicKey
	client     *StorageClient

	// How many failures in a row?
	ownedDownloadConsecutiveFailures int

	// the time that last failure
	ownedDownloadRecentFailure time.Time

	// Notifications of new work. Takes priority over uploads.
	downloadChan chan struct{}

	// Yet unprocessed work items.
	downloadSegments []*unfinishedDownloadSegment
	downloadMu       sync.Mutex

	// Has downloading been terminated for this worker?
	downloadTerminated bool

	// TODO: Upload variables.
	//unprocessedSegments         []*unfinishedUploadSegment // Yet unprocessed work items.
	//uploadChan                chan struct{}            // Notifications of new work.
	//uploadConsecutiveFailures int                      // How many times in a row uploading has failed.
	//uploadRecentFailure       time.Time                // How recent was the last failure?
	//uploadTerminated          bool                     // Have we stopped uploading?

	// Worker will shut down if a signal is sent down this channel.
	killChan chan struct{}
	mu       sync.Mutex
}

// activateWorkerPool will grab the set of contracts from the contractManager and
// update the worker pool to match.
func (c *StorageClient) activateWorkerPool() {

	// TODO: 获取renter签订的所有合约，fileContractID ==》fileContract
	contractMap := make(map[common.Hash]storage.ClientContract)
	//contractSlice := c.contractManager.Contracts()
	//for i := 0; i < len(contractSlice); i++ {
	//	contractMap[contractSlice[i].ID] = contractSlice[i]
	//}

	// Add a worker for any contract that does not already have a worker.
	for id, contract := range contractMap {
		c.lock.Lock()
		_, exists := c.workerPool[id]
		if !exists {
			worker := &worker{
				contract:   contract,
				hostPubKey: contract.HostPublicKey,

				downloadChan: make(chan struct{}, 1),
				killChan:     make(chan struct{}),

				// TODO: 初始化upload变量
				//uploadChan:   make(chan struct{}, 1),

				client: c,
			}
			c.workerPool[id] = worker
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				worker.workLoop()
			}()
		}
		c.lock.Unlock()
	}

	// Remove a worker for any worker that is not in the set of new contracts.
	c.lock.Lock()
	for id, worker := range c.workerPool {
		_, exists := contractMap[id]
		if !exists {
			delete(c.workerPool, id)
			close(worker.killChan)
		}
	}
	c.lock.Unlock()
}

// workLoop repeatedly issues task to a worker, stopping when the worker
// is killed or when the thread group is closed.
func (w *worker) workLoop() {
	//defer w.managedKillUploading()
	defer w.killDownloading()

	for {

		// Perform one step of processing download task.
		downloadSegment := w.nextDownloadSegment()
		if downloadSegment != nil {
			w.download(downloadSegment)
			continue
		}

		// TODO: 执行上传任务
		// Perform one step of processing upload task.
		//segment, pieceIndex := w.managedNextUploadSegment()
		//if segment != nil {
		//	w.managedUpload(segment, pieceIndex)
		//	continue
		//}

		// keep listening for a new upload/download task, or a stop signal
		select {
		case <-w.downloadChan:
			continue

		// TODO: 监听上传任务
		//case <-w.uploadChan:
		//	continue
		case <-w.killChan:
			return
		case <-w.client.quitCh:
			return
		}
	}
}

// killDownloading will drop all of the download task given to the worker,
// and set a signal to prevent the worker from accepting more download task.
func (w *worker) killDownloading() {
	w.downloadMu.Lock()
	var removedSegments []*unfinishedDownloadSegment
	for i := 0; i < len(w.downloadSegments); i++ {
		removedSegments = append(removedSegments, w.downloadSegments[i])
	}
	w.downloadSegments = w.downloadSegments[:0]
	w.downloadTerminated = true
	w.downloadMu.Unlock()
	for i := 0; i < len(removedSegments); i++ {
		removedSegments[i].removeWorker()
	}
}

// queueDownloadSegment adds a segment to the worker's queue.
func (w *worker) queueDownloadSegment(uds *unfinishedDownloadSegment) {
	// Accept the segment unless the worker has been terminated. Accepting the
	// segment needs to happen under the same lock as fetching the termination
	// status.
	w.downloadMu.Lock()
	terminated := w.downloadTerminated
	if !terminated {
		// Accept the segment and issue a notification to the master thread that
		// there is a new download.
		w.downloadSegments = append(w.downloadSegments, uds)
		select {
		case w.downloadChan <- struct{}{}:
		default:
		}
	}
	w.downloadMu.Unlock()

	// If the worker has terminated, remove it from the uds. This call needs to
	// happen without holding the worker lock.
	if terminated {
		uds.removeWorker()
	}
}

// nextDownloadSegment will pull the next potential segment out of the work
// queue for downloading.
func (w *worker) nextDownloadSegment() *unfinishedDownloadSegment {
	w.downloadMu.Lock()
	defer w.downloadMu.Unlock()

	if len(w.downloadSegments) == 0 {
		return nil
	}
	nextSegment := w.downloadSegments[0]
	w.downloadSegments = w.downloadSegments[1:]
	return nextSegment
}

// TODO: worker执行下载任务
func (w *worker) download(uds *unfinishedDownloadSegment) {

}
