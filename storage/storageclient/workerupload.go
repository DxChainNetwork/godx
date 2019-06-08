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
	uc.workersRemaining--
	uc.mu.Unlock()
	w.client.cleanUpUploadSegment(uc)
}

// dropUploadSegments will release all of the upload segments that the worker has received.
func (w *worker) dropUploadSegments() {
	// Make a copy of the slice under lock, clear the slice, then drop the
	// Segments without a lock (managed function).
	var SegmentsToDrop []*unfinishedUploadSegment
	w.mu.Lock()
	for i := 0; i < len(w.unprocessedSegments); i++ {
		SegmentsToDrop = append(SegmentsToDrop, w.unprocessedSegments[i])
	}
	w.unprocessedSegments = w.unprocessedSegments[:0]
	w.mu.Unlock()

	for i := 0; i < len(SegmentsToDrop); i++ {
		w.dropSegment(SegmentsToDrop[i])
		w.client.log.Debug("dropping Segment because the worker is dropping all Segments", w.contract.HostID.String())
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
		w.client.ethBackend.Disconnect(session, w.contract.HostID.String())
	}

	// After the worker is marked as disabled, clear out all of the segments
	w.dropUploadSegments()
}

// nextUploadSegment will pull the next segment from the worker's upload list
func (w *worker) nextUploadSegment() (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
	// Loop through the unprocessed segments and find some work to do
	for {
		// Pull a segment off of the unprocessed segments stack
		w.mu.Lock()
		if len(w.unprocessedSegments) <= 0 {
			w.mu.Unlock()
			break
		}
		segment := w.unprocessedSegments[0]
		w.unprocessedSegments = w.unprocessedSegments[1:]
		w.mu.Unlock()

		// Process the segment and return it if valid
		nextSegment, pieceIndex := w.preProcessUploadSegment(segment)
		if nextSegment != nil {
			return nextSegment, pieceIndex
		}
	}
	return nil, 0
}

func (w *worker) nextUploadSegment2() (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
	// Loop through the unprocessed segments and find some work to do
	for {
		// Pull a segment off of the unprocessed segments stack
		w.mu.Lock()
		if len(w.sectorIndexMap) <= 0 {
			w.mu.Unlock()
			break
		}
		for k, v := range w.sectorIndexMap {
			nextSegment = k
			if len(v) == 1 {
				pieceIndex = uint64(v[0])
				delete(w.sectorIndexMap, k)
			} else {
				pieceIndex = uint64(v[0])
				v = v[1:]
				w.sectorIndexMap[k] = v
			}
			break
		}
		w.mu.Unlock()
		if err := w.preProcessUploadSegment2(nextSegment); err != nil {
			return nil, 0
		}
		return nextSegment, pieceIndex
	}
	return nil, 0
}

// AppendUploadSegment - Append a segment to the heap to the unprocessedSegments of work
// and the signal the uploadChan
// Deprecated: this original method won't be used because low performance
func (w *worker) AppendUploadSegment(uc *unfinishedUploadSegment) {
	//utility, exists := w.client.hostContractor.ContractUtility(w.contract.HostPublicKey)
	//goodForUpload := exists && utility.GoodForUpload
	goodForUpload := true
	w.mu.Lock()
	onCoolDown := w.onUploadCoolDown()
	uploadTerminated := w.uploadTerminated
	if !goodForUpload || uploadTerminated || onCoolDown {
		w.mu.Unlock()
		w.dropSegment(uc)
		w.client.log.Debug("Dropping segment before putting into queue", !goodForUpload, uploadTerminated, onCoolDown, w.contract.HostID.String())
		return
	}
	w.unprocessedSegments = append(w.unprocessedSegments, uc)
	w.mu.Unlock()

	// signal worker
	select {
	case w.uploadChan <- struct{}{}:
	default:
	}
}

func (w *worker) isReady(uc *unfinishedUploadSegment) bool {
	//utility, exists := w.client.hostContractor.ContractUtility(w.contract.HostPublicKey)
	//goodForUpload := exists && utility.GoodForUpload
	goodForUpload := true
	w.mu.Lock()
	defer w.mu.Unlock()
	onCoolDown := w.onUploadCoolDown()
	uploadTerminated := w.uploadTerminated
	if !goodForUpload || uploadTerminated || onCoolDown {
		// why drop segment when work is not ready ???
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

// upload will perform some upload work.
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
	w.client.cleanUpUploadSegment(uc)
}

// onUploadCoolDown returns true if the worker is on coolDown from failed
// uploads.
func (w *worker) onUploadCoolDown() bool {
	requiredCoolDown := UploadFailureCoolDown
	for i := 0; i < w.uploadConsecutiveFailures && i < MaxConsecutivePenalty; i++ {
		requiredCoolDown *= 2
	}
	return time.Now().Before(w.uploadRecentFailure.Add(requiredCoolDown))
}

// preProcessUploadSegment will pre-process a segment from the worker segment queue
// Deprecated:
func (w *worker) preProcessUploadSegment(uc *unfinishedUploadSegment) (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
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
	isComplete := uc.sectorsNeedNum <= uc.sectorsCompletedNum
	needsHelp := uc.sectorsNeedNum > uc.sectorsCompletedNum+uc.sectorsUploadingNum
	// If the Segment does not need help from this worker, release the segment
	if isComplete || !candidateHost || !goodForUpload || onCoolDown {
		// This worker no longer needs to track this segment
		uc.mu.Unlock()
		w.dropSegment(uc)
		w.client.log.Debug("Worker dropping a Segment while processing", isComplete, !candidateHost, !goodForUpload, onCoolDown, w.contract.HostID.String())
		return nil, 0
	}

	// If the worker does not need help, add the worker to the sent of standby segments
	if !needsHelp {
		uc.workerBackups = append(uc.workerBackups, w)
		uc.mu.Unlock()
		w.client.cleanUpUploadSegment(uc)
		return nil, 0
	}

	// If the Segment needs help from this worker, find a piece to upload and
	// return the stats for that piece
	// Select a piece and mark that a piece has been selected.
	index := -1
	for i := 0; i < len(uc.sectorSlotsStatus); i++ {
		if !uc.sectorSlotsStatus[i] {
			index = i
			uc.sectorSlotsStatus[i] = true
			break
		}
	}
	if index == -1 {
		uc.mu.Unlock()
		w.dropSegment(uc)
		return nil, 0
	}
	delete(uc.unusedHosts, w.contract.HostID.String())
	uc.sectorsUploadingNum++
	uc.workersRemaining--
	uc.mu.Unlock()
	return uc, uint64(index)
}

// preProcessUploadSegment will pre-process a segment from the worker segment queue
func (w *worker) preProcessUploadSegment2(uc *unfinishedUploadSegment) error {
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
	isComplete := uc.sectorsNeedNum <= uc.sectorsCompletedNum
	isNeedUpload := uc.sectorsNeedNum > uc.sectorsCompletedNum+uc.sectorsUploadingNum
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
		w.client.cleanUpUploadSegment(uc)
		return errors.New("add worker to the sent of standby segments")
	}

	// If the segment needs help from this worker, find a sector to upload and
	// return the stats for that sector
	// Select a sector and mark that a sector has been selected.
	delete(uc.unusedHosts, w.contract.HostID.String())
	uc.sectorsUploadingNum++
	uc.workersRemaining--
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

	// Unregister the piece from the Segment and hunt for a replacement.
	uc.mu.Lock()
	uc.sectorsUploadingNum--
	uc.sectorSlotsStatus[pieceIndex] = false
	uc.mu.Unlock()

	// Notify the standby workers of the Segment
	uc.managedNotifyStandbyWorkers()
	w.client.cleanUpUploadSegment(uc)

	// Because the worker is now on cooldown, drop all remaining Segments.
	w.dropUploadSegments()
}
