// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package storageclient

import (
	"github.com/DxChainNetwork/godx/p2p/enode"
	"time"
)

// managedDropSegment will remove a worker from the responsibility of tracking a Segment.
//
// This function is managed instead of static because it is against convention
// to be calling functions on other objects (in this case, the renter) while
// holding a lock.
func (w *worker) managedDropSegment(uc *unfinishedUploadSegment) {
	uc.mu.Lock()
	uc.workersRemaining--
	uc.mu.Unlock()
	w.client.managedCleanUpUploadSegment(uc)
}

// managedDropUploadSegments will release all of the upload Segments that the worker
// has received.
func (w *worker) managedDropUploadSegments() {
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
		w.managedDropSegment(SegmentsToDrop[i])
		w.client.log.Debug("dropping Segment because the worker is dropping all Segments", w.contract.HostEnodeUrl)
	}
}

// managedKillUploading will disable all uploading for the worker.
func (w *worker) managedKillUploading() {
	// Mark the worker as disabled so that incoming Segments are rejected.
	w.mu.Lock()
	w.uploadTerminated = true
	w.mu.Unlock()

	// After the worker is marked as disabled, clear out all of the Segments.
	w.managedDropUploadSegments()
}

// managedNextUploadSegment will pull the next potential Segment out of the worker's
// work queue for uploading.
func (w *worker) managedNextUploadSegment() (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
	// Loop through the unprocessed Segments and find some work to do.
	for {
		// Pull a Segment off of the unprocessed Segments stack.
		w.mu.Lock()
		if len(w.unprocessedSegments) <= 0 {
			w.mu.Unlock()
			break
		}
		segment := w.unprocessedSegments[0]
		w.unprocessedSegments = w.unprocessedSegments[1:]
		w.mu.Unlock()

		// Process the Segment and return it if valid.
		nextSegment, pieceIndex := w.managedProcessUploadSegment(segment)
		if nextSegment != nil {
			return nextSegment, pieceIndex
		}
	}
	return nil, 0 // no work found
}

// managedQueueUploadSegment will take a Segment and add it to the worker's repair
// stack.
func (w *worker) managedQueueUploadSegment(uc *unfinishedUploadSegment) {

	utility, exists := w.client.hostContractor.ContractUtility(w.contract.HostPublicKey)
	goodForUpload := exists && utility.GoodForUpload
	w.mu.Lock()
	onCoolDown := w.onUploadCooldown()
	uploadTerminated := w.uploadTerminated
	if !goodForUpload || uploadTerminated || onCoolDown {
		w.mu.Unlock()
		w.managedDropSegment(uc)
		w.client.log.Debug("Dropping Segment before putting into queue", !goodForUpload, uploadTerminated, onCoolDown, w.contract.HostEnodeUrl)
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
func (w *worker) managedUpload(uc *unfinishedUploadSegment, pieceIndex uint64) {
	// Open an editing connection to the host
	// TODO 考虑要不要复用client与host之间的session connection
	//e, err := w.renter.hostContractor.Editor(w.contract.HostPublicKey, w.renter.tg.StopChan())
	session, err := w.client.ethBackend.SetupConnection(w.contract.HostEnodeUrl)
	if err != nil {
		w.client.log.Debug("Worker failed to acquire an editor:", err)
		w.managedUploadFailed(uc, pieceIndex)
		return
	}
	defer w.client.ethBackend.Disconnect(w.contract.HostEnodeUrl)

	root, err := w.client.Append(session, uc.physicalSegmentData[pieceIndex])
	if err != nil {
		w.client.log.Debug("Worker failed to upload via the editor:", err)
		w.managedUploadFailed(uc, pieceIndex)
		return
	}
	w.mu.Lock()
	w.uploadConsecutiveFailures = 0
	w.mu.Unlock()

	// Add piece to renterFile
	err = uc.fileEntry.AddSector(enode.HexID(w.contract.HostEnodeUrl), root, int(uc.index), int(pieceIndex))
	if err != nil {
		w.client.log.Debug("Worker failed to add new piece to SiaFile:", err)
		w.managedUploadFailed(uc, pieceIndex)
		return
	}

	// Upload is complete. Update the state of the Segment and the renter's memory
	// available to reflect the completed upload.
	uc.mu.Lock()
	releaseSize := len(uc.physicalSegmentData[pieceIndex])
	uc.piecesRegistered--
	uc.piecesCompleted++
	uc.physicalSegmentData[pieceIndex] = nil
	uc.memoryReleased += uint64(releaseSize)
	uc.mu.Unlock()
	w.client.memoryManager.Return(uint64(releaseSize))
	w.client.managedCleanUpUploadSegment(uc)
}

// onUploadCooldown returns true if the worker is on cooldown from failed
// uploads.
func (w *worker) onUploadCooldown() bool {
	requiredCooldown := UploadFailureCoolDown
	for i := 0; i < w.uploadConsecutiveFailures && i < MaxConsecutivePenalty; i++ {
		requiredCooldown *= 2
	}
	return time.Now().Before(w.uploadRecentFailure.Add(requiredCooldown))
}

// managedProcessUploadSegment will process a Segment from the worker Segment queue.
func (w *worker) managedProcessUploadSegment(uc *unfinishedUploadSegment) (nextSegment *unfinishedUploadSegment, pieceIndex uint64) {
	// Determine the usability value of this worker.
	utility, exists := w.client.hostContractor.ContractUtility(w.contract.HostEnodeUrl)
	goodForUpload := exists && utility.GoodForUpload
	w.mu.Lock()
	onCooldown := w.onUploadCooldown()
	w.mu.Unlock()

	// Determine what sort of help this Segment needs.
	uc.mu.Lock()
	_, candidateHost := uc.unusedHosts[w.contract.HostEnodeUrl]
	SegmentComplete := uc.piecesNeeded <= uc.piecesCompleted
	needsHelp := uc.piecesNeeded > uc.piecesCompleted+uc.piecesRegistered
	// If the Segment does not need help from this worker, release the Segment.
	if SegmentComplete || !candidateHost || !goodForUpload || onCooldown {
		// This worker no longer needs to track this Segment.
		uc.mu.Unlock()
		w.managedDropSegment(uc)
		w.client.log.Debug("Worker dropping a Segment while processing", SegmentComplete, !candidateHost, !goodForUpload, onCooldown, w.contract.HostEnodeUrl)
		return nil, 0
	}

	// If the worker does not need help, add the worker to the sent of standby
	// Segments.
	if !needsHelp {
		uc.workersStandby = append(uc.workersStandby, w)
		uc.mu.Unlock()
		w.client.managedCleanUpUploadSegment(uc)
		return nil, 0
	}

	// If the Segment needs help from this worker, find a piece to upload and
	// return the stats for that piece.
	//
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
		w.managedDropSegment(uc)
		return nil, 0
	}
	delete(uc.unusedHosts, w.contract.HostEnodeUrl)
	uc.piecesRegistered++
	uc.workersRemaining--
	uc.mu.Unlock()
	return uc, uint64(index)
}

// managedUploadFailed is called if a worker failed to upload part of an unfinished
// Segment.
func (w *worker) managedUploadFailed(uc *unfinishedUploadSegment, pieceIndex uint64) {
	// Mark the failure in the worker if the gateway says we are online. It's
	// not the worker's fault if we are offline.
	if w.client.Online(){
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
	w.client.managedCleanUpUploadSegment(uc)

	// Because the worker is now on cooldown, drop all remaining Segments.
	w.managedDropUploadSegments()
}
