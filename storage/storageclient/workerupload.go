// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package storageclient

import (
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

// dropSegment will remove a worker from the responsibility of tracking a segment
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
	for i := 0; i < len(w.pendingSegments); i++ {
		segmentsToDrop = append(segmentsToDrop, w.pendingSegments[i])
	}
	w.pendingSegments = w.pendingSegments[:0]
	w.mu.Unlock()

	for i := 0; i < len(segmentsToDrop); i++ {
		w.dropSegment(segmentsToDrop[i])
		w.client.log.Info("dropping segment because the worker is dropping all segments", "contractID", w.contract.ID.String())
	}
}

// killUploading will disable all uploading for the worker
func (w *worker) killUploading() {
	// Mark the worker as disabled so that incoming segments are rejected
	w.mu.Lock()
	w.uploadTerminated = true
	w.mu.Unlock()

	contractID := storage.ContractID(w.contract.ID)
	session, ok := w.client.sessionSet[contractID]
	if session != nil && ok {
		delete(w.client.sessionSet, contractID)
		if err := w.client.disconnect(session, w.contract.EnodeID); err != nil {
			w.client.log.Error("can't close connection after uploading", "error", err)
		}
	}

	// After the worker is marked as disabled, clear out all of the segments
	w.dropUploadSegments()
}

// nextUploadSegment pull the next segment task from the worker's upload task list
func (w *worker) nextUploadSegment() (nextSegment *unfinishedUploadSegment, sectorIndex uint64) {
	// Loop through the unprocessed segments and find some work to do
	for {
		// Pull a segment off of the unprocessed segments stack
		w.mu.Lock()
		if len(w.pendingSegments) <= 0 {
			w.mu.Unlock()
			break
		}

		segment := w.pendingSegments[0]
		w.pendingSegments = w.pendingSegments[1:]
		w.mu.Unlock()

		// Process the segment and return it if valid
		nextSegment, sectorIndex := w.preProcessUploadSegment(segment)
		if nextSegment != nil {
			return nextSegment, sectorIndex
		}
	}
	return nil, 0
}

// isReady indicates that a worker is ready for uploading a segment
// It must be UploadAbility, not on cool down and not terminated
func (w *worker) isReady(uc *unfinishedUploadSegment) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	uploadAbility := false
	if storage.ENV == storage.Env_Test {
		uploadAbility = true
	}
	if meta, ok := w.client.contractManager.RetrieveActiveContract(w.contract.ID); ok {
		uploadAbility = meta.Status.UploadAbility
	}

	onCoolDown := w.onUploadCoolDown()
	uploadTerminated := w.uploadTerminated

	if !uploadAbility || uploadTerminated || onCoolDown {
		// drop segment when work is not ready
		w.dropSegment(uc)
		w.client.log.Info("Append worker unfinished segments failed due to it is not ready", "uploadAbility", !uploadAbility, "uploadTerminated", uploadTerminated, "onCoolDown", onCoolDown, "contractID", w.contract.ID.String())
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
func (w *worker) upload(uc *unfinishedUploadSegment, sectorIndex uint64) error {
	session, err := w.checkSession()
	defer func() {
		if session != nil {
			session.ResetBusy()

			select {
			case session.RevisionDone() <- struct{}{}:
			default:
			}
		}
	}()
	if err != nil {
		w.client.log.Error("check session failed", "err", err)
		w.uploadFailed(uc, sectorIndex)
		return err
	}

	// upload segment to host
	root, err := w.client.Append(session, uc.physicalSegmentData[sectorIndex])
	if err != nil {
		w.client.log.Error("Worker failed to upload", "err", err)
		w.uploadFailed(uc, sectorIndex)
		return err
	}
	w.mu.Lock()
	w.uploadConsecutiveFailures = 0
	w.mu.Unlock()
	// Add sector to storage clientFile
	err = uc.fileEntry.AddSector(w.contract.EnodeID, root, int(uc.index), int(sectorIndex))
	if err != nil {
		w.client.log.Error("Worker failed to add new sector in dxfile", "err", err)
		w.uploadFailed(uc, sectorIndex)
		return err
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

	return nil
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
func (w *worker) preProcessUploadSegment(uc *unfinishedUploadSegment) (*unfinishedUploadSegment, uint64) {
	// Determine the usability value of this worker
	uploadAbility := false
	if meta, ok := w.client.contractManager.RetrieveActiveContract(w.contract.ID); ok {
		uploadAbility = meta.Status.UploadAbility
	}

	w.mu.Lock()
	onCoolDown := w.onUploadCoolDown()
	w.mu.Unlock()

	// Determine what sort of help this segment needs
	// uc.mu condition race, low performance
	uc.mu.Lock()
	_, candidateHost := uc.unusedHosts[w.contract.EnodeID.String()]
	isComplete := uc.sectorsAllNeedNum <= uc.sectorsCompletedNum
	isNeedUpload := uc.sectorsAllNeedNum > uc.sectorsCompletedNum+uc.sectorsUploadingNum

	// If the segment does not need help from this worker, release the segment
	if isComplete || !candidateHost || !uploadAbility || onCoolDown {
		// This worker no longer needs to track this segment
		uc.mu.Unlock()
		w.dropSegment(uc)
		w.client.log.Info("Worker will drop a segment due to it's status: complete/notCandidate/uploadInAbility/onCoolDown")
		return nil, 0
	}

	// If the worker does not need to upload, add the worker to be sent to backup worker queue
	if !isNeedUpload {
		uc.workerBackups = append(uc.workerBackups, w)
		uc.mu.Unlock()
		w.client.cleanupUploadSegment(uc)
		return nil, 0
	}

	// If the segment needs upload by this worker, find a sector to upload and return the index for that sector
	// and then mark the sector as true
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

	delete(uc.unusedHosts, w.contract.EnodeID.String())
	uc.sectorsUploadingNum++
	uc.workersRemain--
	uc.mu.Unlock()
	return uc, uint64(index)
}

// uploadFailed is called if a worker failed to upload part of an unfinished segment
func (w *worker) uploadFailed(uc *unfinishedUploadSegment, sectorIndex uint64) {
	// Mark the failure in the worker if the gateway says we are online. It's
	// not the worker's fault if we are offline
	if w.client.Online() {
		w.mu.Lock()
		w.uploadRecentFailure = time.Now()
		w.uploadConsecutiveFailures++
		w.mu.Unlock()
	}

	// Unregister the sector from the segment and hunt for a replacement
	uc.mu.Lock()
	uc.workersRemain--
	uc.sectorsUploadingNum--
	uc.sectorSlotsStatus[sectorIndex] = false
	uc.mu.Unlock()

	// Clean up this segment, we may notify backup workers of segment to help upload
	w.client.cleanupUploadSegment(uc)

	// Because the worker is now on cool down, drop all other remaining segments
	w.dropUploadSegments()
}
