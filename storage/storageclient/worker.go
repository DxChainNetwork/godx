// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"errors"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"

)

var (
	// ErrNoContractsWithHost will be used when the client has no contract with host
	// the worker will be terminated
	ErrNoContractsWithHost = errors.New("no contract with host which is need to terminate")

	// ErrUnableRetrieveHostInfo is used when host information cannot be retrieved
	// worker will be terminated
	ErrUnableRetrieveHostInfo = errors.New("can't retrieve host info")

	// ErrContractRenewing is used when client and host is renewing contract
	// the worker will return directly
	ErrContractRenewing = errors.New("client and host is renewing contract")
)

// Listen for a work on a certain host.
type worker struct {

	// The contract and host used by this worker.
	contract storage.ContractMetaData
	hostID   enode.ID
	client   *StorageClient

	// How many failures in a row?
	ownedDownloadConsecutiveFailures int

	// the time that last failure
	ownedDownloadRecentFailure time.Time

	// Notifications of new download work. Takes priority over uploads.
	downloadChan chan struct{}

	downloadSegments []*unfinishedDownloadSegment
	downloadMu       sync.Mutex

	// Has downloading been terminated for this worker?
	downloadTerminated bool

	// pending upload segment in heap
	pendingSegments []*unfinishedUploadSegment

	uploadChan                chan struct{} // Notifications of new segment
	uploadConsecutiveFailures int           // How many times in a row uploading has failed.
	uploadRecentFailure       time.Time     // How recent was the last failure?
	uploadTerminated          bool          // Have we stopped uploading?

	// Worker will shut down if a signal is sent down this channel.
	killChan chan struct{}
	mu       sync.Mutex
}

// ActivateWorkerPool will grab the set of contracts from the contract manager and
// update the worker pool to match.
func (client *StorageClient) activateWorkerPool() {
	// get all contracts in client
	contractMap := client.contractManager.GetStorageContractSet().Contracts()

	// new a worker for a contract that haven't a worker
	for id, contract := range contractMap {
		client.lock.Lock()
		_, exists := client.workerPool[id]
		if !exists {
			worker := &worker{
				contract:     contract.Metadata(),
				hostID:       contract.Header().EnodeID,
				downloadChan: make(chan struct{}, 1),
				uploadChan:   make(chan struct{}, 1),
				killChan:     make(chan struct{}),
				client:       client,
			}
			client.workerPool[id] = worker

			// start worker goroutine
			if err := client.tm.Add(); err != nil {
				log.Error("storage client failed to add in worker progress", "error", err)
				client.lock.Unlock()
				break
			}
			go func() {
				defer client.tm.Done()
				worker.workLoop()
			}()

		}
		client.lock.Unlock()
	}

	// Remove a worker for any worker that is not in the set of new contracts.
	client.lock.Lock()
	for id, worker := range client.workerPool {
		_, exists := contractMap[storage.ContractID(id)]
		if !exists {
			delete(client.workerPool, id)
			close(worker.killChan)
		}
	}
	client.lock.Unlock()
}

// WorkLoop repeatedly issues task to a worker, will stop when receive stop or kill signal
func (w *worker) workLoop() {
	defer w.killUploading()
	defer w.killDownloading()

	for {
		downloadSegment := w.nextDownloadSegment()
		if downloadSegment != nil {
			err := w.download(downloadSegment)
			if err == ErrNoContractsWithHost || err == ErrUnableRetrieveHostInfo {
				break
			}

			if err == ErrContractRenewing {
				<-time.After(50 * time.Millisecond)
			}
			if err != nil {
				return
			}
			continue
		}

		segment, sectorIndex := w.nextUploadSegment()
		if segment != nil {
			err := w.upload(segment, sectorIndex)
			if err == ErrNoContractsWithHost || err == ErrUnableRetrieveHostInfo {
				break
			}

			// the client is renewing, we wait for some millisecond
			if err == ErrContractRenewing {
				<-time.After(50 * time.Millisecond)
			}
			if err != nil {
				return
			}
			continue
		}

		// keep listening for a new upload/download task, or a stop signal
		select {
		case <-w.downloadChan:
			continue
		case <-w.uploadChan:
			continue
		case <-w.killChan:
			return
		case <-w.client.tm.StopChan():
			return
		}
	}
}

// Drop all of the download task given to the worker.
func (w *worker) killDownloading() {
	w.downloadMu.Lock()
	var removedSegments []*unfinishedDownloadSegment
	for i := 0; i < len(w.downloadSegments); i++ {
		removedSegments = append(removedSegments, w.downloadSegments[i])
	}
	w.downloadSegments = w.downloadSegments[:0]
	w.downloadTerminated = true
	w.downloadMu.Unlock()

	// close connection after downloading
	for i := 0; i < len(removedSegments); i++ {
		removedSegments[i].removeWorker()
	}
}

// Add a segment to the worker's queue.
func (w *worker) queueDownloadSegment(uds *unfinishedDownloadSegment) {
	w.downloadMu.Lock()
	terminated := w.downloadTerminated
	if !terminated {

		// accept the segment and notify client that there is a new download.
		w.downloadSegments = append(w.downloadSegments, uds)
		select {
		case w.downloadChan <- struct{}{}:
		default:
		}
	}
	w.downloadMu.Unlock()

	// if the worker has terminated, remove it from the uds
	if terminated {
		uds.removeWorker()
	}
}

// Pull the next potential segment out of the work queue for downloading.
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

func (w *worker) checkConnection() (storage.Peer, *storage.HostInfo, error) {
	// check this contract whether is renewing
	contractID := w.contract.ID

	// get the storage host information
	hostInfo, err := w.updateWorkerContractID(contractID)
	if err != nil {
		return nil, nil, err
	}

	// set up the connection
	sp, err := w.client.SetupConnection(hostInfo.EnodeURL)

	// start contract revision, if failed, meaning the
	// renewing is started
	if ok := sp.TryToRenewOrRevise(); !ok {
		return nil, nil, errors.New("the contract is currently renewing or revising")
	}

	return sp, hostInfo, err
}

// Actually perform a download task
func (w *worker) download(uds *unfinishedDownloadSegment) error {
	sp, hostInfo, err := w.checkConnection()
	defer sp.RevisionOrRenewingDone()

	if err != nil {
		w.client.log.Error("failed to check the connection", "err", err)
		return err
	}

	// check the uds whether can be the worker performed
	uds = w.processDownloadSegment(uds)
	if uds == nil {
		return err
	}

	// whether download success or fail, we should remove the worker at last
	defer uds.removeWorker()

	// for not supporting partial encoding, we need to download the whole sector every time.
	fetchOffset, fetchLength := 0, storage.SectorSize
	root := uds.segmentMap[w.hostID.String()].root

	// call rpc request the data from host, if get error, unregister the worker.
	sectorData, err := w.client.Download(sp, root, uint32(fetchOffset), uint32(fetchLength), hostInfo)
	if err != nil {
		w.client.log.Error("worker failed to download sector", "error", err)
		uds.unregisterWorker(w)
		return err
	}

	// decrypt the sector
	key := uds.clientFile.CipherKey()
	decryptedSector, err := key.DecryptInPlace(sectorData)
	if err != nil {
		w.client.log.Error("worker failed to decrypt sector", "error", err)
		uds.unregisterWorker(w)
		return err
	}

	// mark the sector as completed
	sectorIndex := uds.segmentMap[w.hostID.String()].index
	uds.mu.Lock()
	uds.markSectorCompleted(sectorIndex)
	uds.sectorsRegistered--

	// if the num of sectorsCompleted has not reached the required min sector num,
	// go on keeping the decrypted sector.
	if uds.sectorsCompleted <= uds.erasureCode.MinSectors() {
		uds.physicalSegmentData[sectorIndex] = decryptedSector
		w.client.log.Debug("received a sector,but not enough to recover", "sectors_completed", uds.sectorsCompleted)
	}

	// recover the logical data
	if uds.sectorsCompleted == uds.erasureCode.MinSectors() {
		go uds.recoverLogicalData()
		w.client.log.Debug("received enough sectors to recover", "sectors_completed", uds.sectorsCompleted)
	}

	uds.mu.Unlock()

	return nil
}

// Check the given download segment whether there is work to do, and update its info
func (w *worker) processDownloadSegment(uds *unfinishedDownloadSegment) *unfinishedDownloadSegment {
	uds.mu.Lock()
	segmentComplete := uds.sectorsCompleted >= uds.erasureCode.MinSectors() || uds.download.isComplete()
	segmentFailed := uds.sectorsCompleted+uds.workersRemaining < uds.erasureCode.MinSectors()
	sectorData, workerHasSector := uds.segmentMap[w.hostID.String()]

	sectorCompleted := uds.completedSectors[sectorData.index]

	// if the given segment downloading complete/fail, or no sector associated with host for downloading,
	// or the sector has completed, the worker should be removed.
	if segmentComplete || segmentFailed || w.onDownloadCooldown() || !workerHasSector || sectorCompleted {
		uds.mu.Unlock()
		uds.removeWorker()
		return nil
	}
	defer uds.mu.Unlock()

	// if need more sector, and the sector has not been fetched yet,
	// should register the worker and return the segment for downloading.
	sectorTaken := uds.sectorUsage[sectorData.index]
	sectorsInProgress := uds.sectorsRegistered + uds.sectorsCompleted
	desiredSectorsInProgress := uds.erasureCode.MinSectors() + uds.overdrive
	workersDesired := sectorsInProgress < desiredSectorsInProgress && !sectorTaken
	if workersDesired {
		uds.sectorsRegistered++
		uds.sectorUsage[sectorData.index] = true
		return uds
	}

	// put this worker on standby for this segment, we can use it to download later.
	uds.workersStandby = append(uds.workersStandby, w)
	return nil
}

// Return true if the worker is on cooldown for download failure.
func (w *worker) onDownloadCooldown() bool {
	requiredCooldown := DownloadFailureCooldown
	for i := 0; i < w.ownedDownloadConsecutiveFailures && i < MaxConsecutivePenalty; i++ {
		requiredCooldown *= 2
	}
	return time.Now().Before(w.ownedDownloadRecentFailure.Add(requiredCooldown))
}

// Remove the worker from an unfinished download segment,
// and then un-register the sectors that it grabbed.
//
// NOTE: This function should only be called when a worker download fails.
func (uds *unfinishedDownloadSegment) unregisterWorker(w *worker) {
	uds.mu.Lock()
	uds.sectorsRegistered--
	sectorIndex := uds.segmentMap[w.hostID.String()].index
	uds.sectorUsage[sectorIndex] = false
	uds.mu.Unlock()
}

func (w *worker) updateWorkerContractID(contractID storage.ContractID) (*storage.HostInfo, error) {
	hostInfo, ok := w.client.storageHostManager.RetrieveHostInfo(w.hostID)
	if !ok {
		return nil, ErrUnableRetrieveHostInfo
	}

	cm := w.client.contractManager
	if _, exist := cm.RetrieveActiveContract(contractID); exist {
		return &hostInfo, nil
	}

	scs := cm.GetStorageContractSet()
	renewContractID := scs.GetContractIDByHostID(w.hostID)
	if contract, exist := cm.RetrieveActiveContract(renewContractID); exist {
		w.contract = contract
		w.hostID = contract.EnodeID
		return &hostInfo, nil
	}

	return nil, ErrNoContractsWithHost
}
