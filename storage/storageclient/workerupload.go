// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package storageclient

import (
	"github.com/DxChainNetwork/godx/storage"
	"github.com/pkg/errors"
	"time"
)

// dropSegment will remove a worker from the responsibility of tracking a segment
// This function is managed instead of static because it is against convention
// to be calling functions on other objects (in this case, the storage client) while
// holding a lock
func (w *worker) dropSegment(uc *unfinishedUploadSegment) {
	uc.mu.Lock()
	uc.workersRemain--
	uc.mu.Unlock()
	w.client.cleanupUploadSegment(uc)
}

// dropUploadSegments release all of the upload segments that the worker has received
// and then foreach unfinished segments to drop it
func (w *worker) dropUploadSegments() {
	var segmentsToDrop []*unfinishedUploadSegment

	w.mu.Lock()
	for k, _ := range w.sectorIndexMap {
		segmentsToDrop = append(segmentsToDrop, k)
	}
	w.sectorIndexMap = make(map[*unfinishedUploadSegment][]int)
	w.sectorTaskNum = 0
	w.mu.Unlock()

	for i := 0; i < len(segmentsToDrop); i++ {
		w.dropSegment(segmentsToDrop[i])
		w.client.log.Debug("dropping segment because the worker is dropping all segments", w.contract.HostID.String())
	}
}

// killUploading will disable all uploading for the worker
func (w *worker) killUploading() {
	// Mark the worker as disabled so that incoming segments are rejected
	w.mu.Lock()
	w.uploadTerminated = true
	w.mu.Unlock()

	contractID := storage.ContractID(w.contract.ContractID)
	session, ok := w.client.sessionSet[contractID]
	if session != nil && ok {
		delete(w.client.sessionSet, contractID)
		if err := w.client.ethBackend.Disconnect(session, w.contract.HostID.String()); err != nil {
			w.client.log.Debug("can't close connection after uploading, error: ", err)
		}
	}

	// After the worker is marked as disabled, clear out all of the segments
	w.dropUploadSegments()
}

// nextUploadSegment pull the next segment task from the worker's upload task list
func (w *worker) nextUploadSegment() (nextSegment *unfinishedUploadSegment, sectorIndex uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Retrieve a sector of segment from the unprocessed segments, Once found, return
	for k, v := range w.sectorIndexMap {
		nextSegment = k
		if len(v) == 1 {
			sectorIndex = uint64(v[0])
			delete(w.sectorIndexMap, k)
		} else {
			sectorIndex = uint64(v[0])
			v = v[1:]
			w.sectorIndexMap[k] = v
		}
		break
	}

	// When get a ready segment, pre-process check that the worker and segment whether can be uploaded
	if err := w.preProcessUploadSegment(nextSegment); err != nil {
		return nil, 0
	}

	return nextSegment, sectorIndex
}

// isReady indicates that a woker is ready for uploading a segment
// It must be goodForUpload, not on cool down and not terminated
func (w *worker) isReady(uc *unfinishedUploadSegment) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	//utility, exists := w.client.hostContractor.ContractUtility(w.contract.HostPublicKey)
	//goodForUpload := exists && utility.GoodForUpload
	goodForUpload := true
	onCoolDown := w.onUploadCoolDown()
	uploadTerminated := w.uploadTerminated

	if !goodForUpload || uploadTerminated || onCoolDown {
		// drop segment when work is not ready
		w.dropSegment(uc)
		w.client.log.Debug("Dropping segment before putting into queue", !goodForUpload, uploadTerminated, onCoolDown, w.contract.HostID.String())
		return false
	}
	return true
}

// Signal worker by sending uploadChan and then worker will retrieve sector index to upload sector
func (w *worker) signalUploadChan(uc *unfinishedUploadSegment) {
	select {
	case w.uploadChan <- struct{}{}:
	default:
	}
}

// upload will perform some upload work
func (w *worker) upload(uc *unfinishedUploadSegment, sectorIndex uint64) {
	contractID := storage.ContractID(w.contract.ContractID)

	// Renew is doing, refuse upload/download
	if _, ok := w.client.renewFlags[contractID]; ok {
		w.client.log.Debug("renew contract is doing, can't upload")
		w.uploadFailed(uc, sectorIndex)
		return
	}

	// Setup an active connection to the host and we will reuse previous connection
	session, ok := w.client.sessionSet[contractID]
	if !ok || session == nil || session.IsClosed() {
		s, err := w.client.ethBackend.SetupConnection(w.contract.HostID.String())
		if err != nil {
			w.client.log.Debug("Worker failed to setup an connection:", err)
			w.uploadFailed(uc, sectorIndex)
			return
		}

		w.client.sessionLock.Lock()
		w.client.sessionSet[contractID] = s
		w.client.sessionLock.Unlock()
		if hostInfo, ok := w.client.storageHostManager.RetrieveHostInfo(w.hostID); ok {
			s.SetHostInfo(&hostInfo)
		}
		session = s
	}

	// upload segment to host
	root, err := w.client.Append(session, uc.physicalSegmentData[sectorIndex])
	if err != nil {
		w.client.log.Debug("Worker failed to upload via the editor:", err)
		w.uploadFailed(uc, sectorIndex)
		return
	}
	w.mu.Lock()
	w.uploadConsecutiveFailures = 0
	w.mu.Unlock()

	// Add piece to storage clientFile
	err = uc.fileEntry.AddSector(w.contract.HostID, root, int(uc.index), int(sectorIndex))
	if err != nil {
		w.client.log.Debug("Worker failed to add new piece to SiaFile:", err)
		w.uploadFailed(uc, sectorIndex)
		return
	}

	// Upload is complete. Update the state of the Segment and the storage client's memory
	// available to reflect the completed upload.
	uc.mu.Lock()
	releaseSize := len(uc.physicalSegmentData[sectorIndex])
	uc.sectorsUploadingNum--
	uc.sectorsCompletedNum++
	uc.physicalSegmentData[sectorIndex] = nil
	uc.memoryReleased += uint64(releaseSize)
	uc.mu.Unlock()
	w.client.memoryManager.Return(uint64(releaseSize))
	w.client.cleanupUploadSegment(uc)
}

// onUploadCoolDown returns true if the worker is on coolDown from failed uploads
func (w *worker) onUploadCoolDown() bool {
	requiredCoolDown := UploadFailureCoolDown
	for i := 0; i < w.uploadConsecutiveFailures && i < MaxConsecutivePenalty; i++ {
		requiredCoolDown *= 2
	}
	return time.Now().Before(w.uploadRecentFailure.Add(requiredCoolDown))
}

// preProcessUploadSegment will pre-process a segment from the worker segment queue
func (w *worker) preProcessUploadSegment(uc *unfinishedUploadSegment) error {
	// Determine the usability value of this worker
	//utility, exists := w.client.contractManager.hostContractor.ContractUtility(w.contract.HostEnodeUrl)
	//goodForUpload := exists && utility.GoodForUpload
	goodForUpload := true
	w.mu.Lock()
	onCoolDown := w.onUploadCoolDown()
	w.mu.Unlock()

	// Determine what sort of help this segment needs
	// uc.mu condition race, low performance
	uc.mu.Lock()
	_, candidateHost := uc.unusedHosts[w.contract.HostID.String()]
	isComplete := uc.sectorsAllNeedNum <= uc.sectorsCompletedNum
	isNeedUpload := uc.sectorsAllNeedNum > uc.sectorsCompletedNum+uc.sectorsUploadingNum
	// If the Segment does not need help from this worker, release the segment
	if isComplete || !candidateHost || !goodForUpload || onCoolDown {
		// This worker no longer needs to track this segment
		uc.mu.Unlock()
		w.dropSegment(uc)
		w.client.log.Debug("Worker dropping a Segment while processing", isComplete, !candidateHost, !goodForUpload, onCoolDown, w.contract.HostID.String())
		return errors.New("worker no longer needs to track segment")
	}

	// If the worker does not need to upload, add the worker to be sent to backup worker queue
	if !isNeedUpload {
		uc.workerBackups = append(uc.workerBackups, w)
		uc.mu.Unlock()
		w.client.cleanupUploadSegment(uc)
		return errors.New("add worker to the sent of standby segments")
	}

	// If the segment needs help from this worker, find a sector to upload and
	// return the stats for that sector
	// Select a sector and mark that a sector has been selected.
	delete(uc.unusedHosts, w.contract.HostID.String())
	uc.sectorsUploadingNum++
	uc.workersRemain--
	uc.mu.Unlock()
	return nil
}

// uploadFailed is called if a worker failed to upload part of an unfinished segment
func (w *worker) uploadFailed(uc *unfinishedUploadSegment, pieceIndex uint64) {
	// Mark the failure in the worker if the gateway says we are online. It's
	// not the worker's fault if we are offline.
	if w.client.Online() {
		w.mu.Lock()
		w.uploadRecentFailure = time.Now()
		w.uploadConsecutiveFailures++
		w.mu.Unlock()
	}

	// Unregister the piece from the segment and hunt for a replacement.
	uc.mu.Lock()
	uc.sectorsUploadingNum--
	uc.sectorSlotsStatus[pieceIndex] = false
	uc.mu.Unlock()

	// Notify the standby workers of the Segment
	uc.notifyBackupWorkers()
	w.client.cleanupUploadSegment(uc)

	// Because the worker is now on cooldown, drop all remaining Segments.
	w.dropUploadSegments()
}
