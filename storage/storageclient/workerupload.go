// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package storageclient

import (
	"time"
)

// dropSegment will remove a worker from the responsibility of tracking a segment
// This function is managed instead of static because it is against convention
// to be calling functions on other objects (in this case, the renter) while
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

// AppendUploadSegment - Append a segment to the heap to the unprocessedSegments of work
// and the signal the uploadChan
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

// managedUpload will perform some upload work.
func (w *worker) upload(uc *unfinishedUploadSegment, sectorIndex uint64) {
	// Open an editing connection to the host
	// TODO 考虑要不要复用client与host之间的session connection
	//e, err := w.renter.hostContractor.Editor(w.contract.HostPublicKey, w.renter.tg.StopChan())
	session, err := w.client.ethBackend.SetupConnection(w.contract.HostID.String())
	if err != nil {
		w.client.log.Debug("Worker failed to acquire an editor:", err)
		w.uploadFailed(uc, sectorIndex)
		return
	}
	defer w.client.ethBackend.Disconnect(session, w.contract.HostID.String())

	root, err := w.client.Append(session, uc.physicalSegmentData[sectorIndex])
	if err != nil {
		w.client.log.Debug("Worker failed to upload via the editor:", err)
		w.uploadFailed(uc, sectorIndex)
		return
	}
	w.mu.Lock()
	w.uploadConsecutiveFailures = 0
	w.mu.Unlock()

	// Add piece to renterFile
	err = uc.fileEntry.AddSector(w.contract.HostID, root, int(uc.index), int(sectorIndex))
	if err != nil {
		w.client.log.Debug("Worker failed to add new piece to SiaFile:", err)
		w.uploadFailed(uc, sectorIndex)
		return
	}

	// Upload is complete. Update the state of the Segment and the renter's memory
	// available to reflect the completed upload.
	uc.mu.Lock()
	releaseSize := len(uc.physicalSegmentData[sectorIndex])
	uc.piecesRegistered--
	uc.piecesCompleted++
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
func (w *worker) preProcessUploadSegment(uc *unfinishedUploadSegment) (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
	// Determine the usability value of this worker
	//utility, exists := w.client.contractManager.hostContractor.ContractUtility(w.contract.HostEnodeUrl)
	//goodForUpload := exists && utility.GoodForUpload
	goodForUpload := true
	w.mu.Lock()
	onCoolDown := w.onUploadCoolDown()
	w.mu.Unlock()

	// Determine what sort of help this segment needs
	uc.mu.Lock()
	_, candidateHost := uc.unusedHosts[w.contract.HostID.String()]
	segmentComplete := uc.piecesNeeded <= uc.piecesCompleted
	needsHelp := uc.piecesNeeded > uc.piecesCompleted+uc.piecesRegistered
	// If the Segment does not need help from this worker, release the segment
	if segmentComplete || !candidateHost || !goodForUpload || onCoolDown {
		// This worker no longer needs to track this segment
		uc.mu.Unlock()
		w.dropSegment(uc)
		w.client.log.Debug("Worker dropping a Segment while processing", segmentComplete, !candidateHost, !goodForUpload, onCoolDown, w.contract.HostID.String())
		return nil, 0
	}

	// If the worker does not need help, add the worker to the sent of standby segments
	if !needsHelp {
		uc.workersStandby = append(uc.workersStandby, w)
		uc.mu.Unlock()
		w.client.cleanUpUploadSegment(uc)
		return nil, 0
	}

	// If the Segment needs help from this worker, find a piece to upload and
	// return the stats for that piece
	// Select a piece and mark that a piece has been selected.
	index := -1
	for i := 0; i < len(uc.pieceUsage); i++ {
		if !uc.pieceUsage[i] {
			index = i
			uc.pieceUsage[i] = true
			break
		}
	}
	if index == -1 {
		uc.mu.Unlock()
		w.dropSegment(uc)
		return nil, 0
	}
	delete(uc.unusedHosts, w.contract.HostID.String())
	uc.piecesRegistered++
	uc.workersRemaining--
	uc.mu.Unlock()
	return uc, uint64(index)
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
	uc.piecesRegistered--
	uc.pieceUsage[pieceIndex] = false
	uc.mu.Unlock()

	// Notify the standby workers of the Segment
	uc.managedNotifyStandbyWorkers()
	w.client.cleanUpUploadSegment(uc)

	// Because the worker is now on cooldown, drop all remaining Segments.
	w.dropUploadSegments()
}
