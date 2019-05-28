// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"crypto/ecdsa"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
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
				contract:     contract,
				hostPubKey:   contract.HostPublicKey,
				downloadChan: make(chan struct{}, 1),
				killChan:     make(chan struct{}),

				// TODO: 初始化upload变量
				//uploadChan:   make(chan struct{}, 1),

				client: c,
			}
			c.workerPool[id] = worker
			if err := c.tm.Add(); err != nil {
				log.Error("storage client thread manager failed to add in activateWorkerPool", "error", err)
				break
			}
			go func() {
				defer c.tm.Done()
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
		//segment, sectorIndex := w.managedNextUploadSegment()
		//if segment != nil {
		//	w.managedUpload(segment, sectorIndex)
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
		case <-w.client.tm.StopChan():
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

// actually perform a download task
func (w *worker) download(uds *unfinishedDownloadSegment) {

	// If 'nil' is returned, it is either because the worker has been removed
	// from the segment entirely, or because the worker has been put on standby.
	uds = w.processDownloadSegment(uds)
	if uds == nil {
		return
	}

	// whether download success or fail, we should remove the worker at last
	defer uds.removeWorker()

	fetchOffset, fetchLength := sectorOffsetAndLength(uds.staticFetchOffset, uds.staticFetchLength, uds.erasureCode)

	hostEnodeID := PubkeyToEnodeID(w.contract.HostPublicKey)
	root := uds.staticSegmentMap[hostEnodeID.String()].root

	// TODO: 获取host netAddress
	// Setup connection
	hostNetAddress := ""
	session, err := w.client.ethBackend.SetupConnection(hostNetAddress)
	defer w.client.ethBackend.Disconnect(hostNetAddress)

	// call rpc request the data from host, if get error, unregister the worker.
	sectorData, err := w.client.Download(session, root, uint32(fetchOffset), uint32(fetchLength))
	if err != nil {
		w.client.log.Debug("worker failed to download sector", "error", err)
		uds.unregisterWorker(w)
		return
	}

	// record the amount of all data transferred between download connection
	atomic.AddUint64(&uds.download.atomicTotalDataTransferred, uds.staticSectorSize)

	// calculate a seed for twofishgcm
	sectorIndex := uds.staticSegmentMap[hostEnodeID.String()].index
	segmentIndexBytes := make([]byte, 8)
	binary.PutUvarint(segmentIndexBytes, uds.staticSegmentIndex)
	sectorIndexBytes := make([]byte, 8)
	binary.PutUvarint(sectorIndexBytes, sectorIndex)
	seed := crypto.Keccak256(segmentIndexBytes, sectorIndexBytes)

	// create a twofishgcm key
	key, err := crypto.NewCipherKey(crypto.GCMCipherCode, seed)
	if err != nil {
		w.client.log.Error("failed to new cipher key", "error", err)
		return
	}

	if uint64(fetchOffset/storage.SegmentSize) == 0 {
		w.client.log.Error("twofish doesn't support a blockIndex != 0")
		return
	}

	// decrypt the sector
	decryptedSector, err := key.DecryptInPlace(sectorData)
	if err != nil {
		w.client.log.Debug("worker failed to decrypt sector", "error", err)
		uds.unregisterWorker(w)
		return
	}

	// mark the sector as completed. perform segment recovery if we newly have enough sectors to do so.
	uds.mu.Lock()
	uds.markSectorCompleted(sectorIndex)
	uds.sectorsRegistered--

	// as soon as the num of sectors completed reached the minimal num of sectors that erasureCode need, we can recover the original data
	if uds.sectorsCompleted <= uds.erasureCode.MinSectors() {

		// this a accumulation processing, every time we receive a sector
		atomic.AddUint64(&uds.download.atomicDataReceived, uds.staticFetchLength/uint64(uds.erasureCode.MinSectors()))
		uds.physicalSegmentData[sectorIndex] = decryptedSector
	}
	if uds.sectorsCompleted == uds.erasureCode.MinSectors() {

		// client maybe receive a not integral sector
		addedReceivedData := uint64(uds.erasureCode.MinSectors()) * (uds.staticFetchLength / uint64(uds.erasureCode.MinSectors()))
		atomic.AddUint64(&uds.download.atomicDataReceived, uds.staticFetchLength-addedReceivedData)

		// recover the logical data
		go uds.recoverLogicalData()
	}
	uds.mu.Unlock()
}

// processDownloadSegment will take a potential download segment, figure out if
// there is work to do, and then perform any registration or processing with the
// segment before returning the segment to the caller.
func (w *worker) processDownloadSegment(uds *unfinishedDownloadSegment) *unfinishedDownloadSegment {
	uds.mu.Lock()
	segmentComplete := uds.sectorsCompleted >= uds.erasureCode.MinSectors() || uds.download.staticComplete()
	segmentFailed := uds.sectorsCompleted+uds.workersRemaining < uds.erasureCode.MinSectors()

	hostEnodeID := PubkeyToEnodeID(w.contract.HostPublicKey)
	sectorData, workerHasSector := uds.staticSegmentMap[hostEnodeID.String()]

	sectorCompleted := uds.completedSectors[sectorData.index]

	// if the given segment downloading complete/fail, or no sector associated with host for downloading, or the sector has completed,
	// the worker should be removed.
	if segmentComplete || segmentFailed || w.onDownloadCooldown() || !workerHasSector || sectorCompleted {
		uds.mu.Unlock()
		uds.removeWorker()
		return nil
	}
	defer uds.mu.Unlock()

	// if the workers in progress are not enough for the segment downloading,
	// we need more extra worker matched the criteria
	meetsExtraCriteria := true

	sectorTaken := uds.sectorUsage[sectorData.index]
	sectorsInProgress := uds.sectorsRegistered + uds.sectorsCompleted
	desiredSectorsInProgress := uds.erasureCode.MinSectors() + uds.staticOverdrive
	workersDesired := sectorsInProgress < desiredSectorsInProgress && !sectorTaken

	// if need more sector, and the sector has not been fetched yet,
	// should register the worker and return the segment for downloading.
	if workersDesired && meetsExtraCriteria {
		uds.sectorsRegistered++
		uds.sectorUsage[sectorData.index] = true
		return uds
	}

	// worker is not needed unless another worker fails, so put this worker on standby for this segment.
	// the worker is still available to help with the download, so the worker is not removed from the segment.
	uds.workersStandby = append(uds.workersStandby, w)
	return nil
}

// returns true if the worker is on cooldown for download failure.
func (w *worker) onDownloadCooldown() bool {
	requiredCooldown := DownloadFailureCooldown
	for i := 0; i < w.ownedDownloadConsecutiveFailures && i < MaxConsecutivePenalty; i++ {
		requiredCooldown *= 2
	}
	return time.Now().Before(w.ownedDownloadRecentFailure.Add(requiredCooldown))
}

// sectorOffsetAndLength translates the fetch offset and length of the segment
// into the offset and length of the sector we need to download for a
// successful recovery of the requested data.
func sectorOffsetAndLength(segmentFetchOffset, segmentFetchLength uint64, rs erasurecode.ErasureCoder) (uint64, uint64) {
	segmentIndex, numSegments := segmentsForRecovery(segmentFetchOffset, segmentFetchLength, rs)
	return uint64(segmentIndex * storage.SegmentSize), uint64(numSegments * storage.SegmentSize)
}

// segmentsForRecovery calculates the first segment and how many segments we
// need in total to recover the requested data.
func segmentsForRecovery(segmentFetchOffset, segmentFetchLength uint64, rs erasurecode.ErasureCoder) (uint64, uint64) {

	// TODO: 确认下擦除🐴是否支持部分编码
	// If partialDecoding is not available we need to download the whole
	// sector.
	//if !rs.SupportsPartialEncoding() {
	//	return 0, uint64(storage.SectorSize) / storage.SegmentSize
	//}

	// Else we need to figure out what segments of the piece we need to
	// download for the recovered data to contain the data we want.
	recoveredSegmentSize := uint64(rs.MinSectors() * storage.SegmentSize)

	// Calculate the offset of the download.
	startSegment := segmentFetchOffset / recoveredSegmentSize

	// Calculate the length of the download.
	endSegment := (segmentFetchOffset + segmentFetchLength) / recoveredSegmentSize
	if (segmentFetchOffset+segmentFetchLength)%recoveredSegmentSize != 0 {
		endSegment++
	}
	return startSegment, endSegment - startSegment
}

// unregisterWorker will remove the worker from an unfinished download
// segment, and then un-register the sectors that it grabbed. This function should
// only be called when a worker download fails.
func (uds *unfinishedDownloadSegment) unregisterWorker(w *worker) {
	uds.mu.Lock()
	uds.sectorsRegistered--
	hostEnodeID := PubkeyToEnodeID(w.contract.HostPublicKey)
	sectorIndex := uds.staticSegmentMap[hostEnodeID.String()].index
	uds.sectorUsage[sectorIndex] = false
	uds.mu.Unlock()
}
