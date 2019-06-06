// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"io"
	"os"
	"sync"
	"time"
)

// uploadSegmentID is a unique identifier for each Segment in the renter.
type uploadSegmentID struct {
	fid   dxfile.FileID // Unique to each file.
	index uint64        // Unique to each Segment within a file.
}

// unfinishedUploadSegment contains a Segment from the filesystem that has not
// finished uploading, including knowledge of the progress.
type unfinishedUploadSegment struct {
	// Information about the file. localPath may be the empty string if the file
	// is known not to exist locally.
	id        uploadSegmentID
	fileEntry *dxfile.FileSetEntryWithID
	threadUID int

	// Information about the Segment, namely where it exists within the file.
	//
	// TODO / NOTE: As we change the file mapper, we're probably going to have
	// to update these fields. Compatibility shouldn't be an issue because this
	// struct is not persisted anywhere, it's always built from other
	// structures.
	index          uint64
	length         uint64
	memoryNeeded   uint64 // memory needed in bytes
	memoryReleased uint64 // memory that has been returned of memoryNeeded
	minimumPieces  int    // number of pieces required to recover the file.
	offset         int64  // Offset of the Segment within the file.
	piecesNeeded   int    // number of pieces to achieve a 100% complete upload
	stuck          bool   // indicates if the Segment was marked as stuck during last repair
	stuckRepair    bool   // indicates if the Segment was identified for repair by the stuck loop

	// The logical data is the data that is presented to the user when the user
	// requests the Segment. The physical data is all of the pieces that get
	// stored across the network.
	logicalSegmentData  [][]byte
	physicalSegmentData [][]byte

	// Worker synchronization fields. The mutex only protects these fields.
	//
	// When a worker passes over a piece for upload to go on standby:
	//	+ the worker should add itself to the list of standby Segments
	//  + the worker should call for memory to be released
	//
	// When a worker passes over a piece because it's not useful:
	//	+ the worker should decrement the number of workers remaining
	//	+ the worker should call for memory to be released
	//
	// When a worker accepts a piece for upload:
	//	+ the worker should increment the number of pieces registered
	// 	+ the worker should mark the piece usage for the piece it is uploading
	//	+ the worker should decrement the number of workers remaining
	//
	// When a worker completes an upload (success or failure):
	//	+ the worker should decrement the number of pieces registered
	//  + the worker should call for memory to be released
	//
	// When a worker completes an upload (failure):
	//	+ the worker should unmark the piece usage for the piece it registered
	//	+ the worker should notify the standby workers of a new available piece
	//
	// When a worker completes an upload successfully:
	//	+ the worker should increment the number of pieces completed
	//	+ the worker should decrement the number of pieces registered
	//	+ the worker should release the memory for the completed piece
	mu               sync.Mutex
	pieceUsage       []bool              // 'true' if a piece is either uploaded, or a worker is attempting to upload that piece.
	piecesCompleted  int                 // number of pieces that have been fully uploaded.
	piecesRegistered int                 // number of pieces that are being uploaded, but aren't finished yet (may fail).
	released         bool                // whether this Segment has been released from the active Segments set.
	unusedHosts      map[string]struct{} // hosts that aren't yet storing any pieces or performing any work.
	workersRemaining int                 // number of inactive workers still able to upload a piece.
	workersStandby   []*worker           // workers that can be used if other workers fail.
}

// managedNotifyStandbyWorkers is called when a worker fails to upload a piece, meaning
// that the standby workers may now be needed to help the piece finish
// uploading.
func (uc *unfinishedUploadSegment) managedNotifyStandbyWorkers() {
	// Copy the standby workers into a new slice and reset it since we can't
	// hold the lock while calling the managed function.
	uc.mu.Lock()
	standbyWorkers := make([]*worker, len(uc.workersStandby))
	copy(standbyWorkers, uc.workersStandby)
	uc.workersStandby = uc.workersStandby[:0]
	uc.mu.Unlock()

	for i := 0; i < len(standbyWorkers); i++ {
		standbyWorkers[i].AppendUploadSegment(uc)
	}
}

// SegmentComplete checks some fields of the Segment to determine if the Segment is
// completed. This can either mean that it ran out of workers or that it was
// uploaded successfully.
func (uc *unfinishedUploadSegment) SegmentComplete() bool {
	// The whole Segment was uploaded successfully.
	if uc.piecesCompleted == uc.piecesNeeded && uc.piecesRegistered == 0 {
		return true
	}
	// We are no longer doing any uploads and we don't have any workers left.
	if uc.workersRemaining == 0 && uc.piecesRegistered == 0 {
		return true
	}
	return false
}

// distributeSegment distribute segment to the workers in the pool
func (sc *StorageClient) distributeSegment(uc *unfinishedUploadSegment) {
	// Add Segment to repairingSegments map
	sc.uploadHeap.mu.Lock()
	_, exists := sc.uploadHeap.repairingSegments[uc.id]
	if !exists {
		sc.uploadHeap.repairingSegments[uc.id] = struct{}{}
	}
	sc.uploadHeap.mu.Unlock()

	// Distribute the segment to each worker in the work pool, marking the number of workers that have received the segment
	sc.lock.Lock()
	uc.workersRemaining += len(sc.workerPool)
	workers := make([]*worker, 0, len(sc.workerPool))
	for _, worker := range sc.workerPool {
		workers = append(workers, worker)
	}
	sc.lock.Unlock()

	// TODO If we have 100 workers, but upload this file needs 30 workers ?
	// Is it necessary to distribute the segment to each worker ?
	for _, worker := range workers {
		worker.AppendUploadSegment(uc)
	}
}

// managedDownloadLogicalSegmentData will fetch the logical Segment data by sending a
// download to the renter's downloader, and then using the data that gets
// returned.
func (sc *StorageClient) managedDownloadLogicalSegmentData(segment *unfinishedUploadSegment) error {
	downloadLength := segment.length
	if segment.index == uint64(segment.fileEntry.NumSegments()-1) && segment.fileEntry.FileSize()%segment.length != 0 {
		downloadLength = segment.fileEntry.FileSize() % segment.length
	}

	// Create the download
	buf := NewDownloadBuffer(segment.length, segment.fileEntry.SectorSize())
	d, err := sc.newDownload(downloadParams{
		destination:     buf,
		destinationType: "buffer",
		file:            segment.fileEntry.DxFile.Snapshot(),

		latencyTarget: 200e3, // No need to rush latency on repair downloads.
		length:        downloadLength,
		needsMemory:   false, // We already requested memory, the download memory fits inside of that.
		offset:        uint64(segment.offset),
		overdrive:     0, // No need to rush the latency on repair downloads.
		priority:      0, // Repair downloads are completely de-prioritized.
	})
	if err != nil {
		return err
	}

	// Register some cleanup for when the download is done.
	d.onComplete(func(_ error) error {
		// Update the access time when the download is done
		return segment.fileEntry.DxFile.SetTimeAccess(time.Now())
	})

	// Set the in-memory buffer to nil just to be safe in case of a memory
	// leak.
	defer func() {
		d.destination = nil
	}()

	// Wait for the download to complete.
	select {
	case <-d.completeChan:
	case <-sc.tm.StopChan():
		return errors.New("repair download interrupted by stop call")
	}
	if d.Err() != nil {
		buf.buf = nil
		return d.Err()
	}
	segment.logicalSegmentData = [][]byte(buf.buf)
	return nil
}

// uploadSegment will fetch the logical data for a segment, create
// the physical pieces for the segment, and then distribute them.
func (sc *StorageClient) uploadSegment(segment *unfinishedUploadSegment) {
	err := sc.tm.Add()
	if err != nil {
		return
	}
	defer sc.tm.Done()

	// Calculate the amount of memory needed for erasure coding. This will need
	// to be released if there's an error before erasure coding is complete.
	erasureCodingMemory := segment.fileEntry.SectorSize() * uint64(segment.fileEntry.ErasureCode().MinSectors())

	// Calculate the amount of memory to release due to already completed
	// pieces. This memory gets released during encryption, but needs to be
	// released if there's a failure before encryption happens.
	var pieceCompletedMemory uint64
	for i := 0; i < len(segment.pieceUsage); i++ {
		if segment.pieceUsage[i] {
			pieceCompletedMemory += storage.SectorSize
		}
	}

	// Ensure that memory is released and that the Segment is cleaned up properly
	// after the Segment is distributed.
	//
	// Need to ensure the erasure coding memory is released as well as the
	// physical Segment memory. Physical Segment memory is released by setting
	// 'workersRemaining' to zero if the repair fails before being distributed
	// to workers. Erasure coding memory is released manually if the repair
	// fails before the erasure coding occurs.
	defer sc.cleanUpUploadSegment(segment)

	// Retrieve the logical data for the Segment.
	err = sc.retrieveLogicalSegmentData(segment)
	if err != nil {
		// retrieve logical data failed, interrupt upload and release memory
		segment.logicalSegmentData = nil
		segment.workersRemaining = 0
		sc.memoryManager.Return(erasureCodingMemory + pieceCompletedMemory)
		segment.memoryReleased += erasureCodingMemory + pieceCompletedMemory
		sc.log.Debug("retrieve logical data of a segment failed:", err)
		return
	}

	// Encode the physical sectors from content bytes of file
	segmentBytes := make([]byte, uint64(len(segment.logicalSegmentData))*storage.SectorSize)
	for _, b := range segment.logicalSegmentData {
		segmentBytes = append(segmentBytes, b...)
	}
	segment.physicalSegmentData, err = segment.fileEntry.ErasureCode().Encode(segmentBytes)
	segment.logicalSegmentData = nil
	sc.memoryManager.Return(erasureCodingMemory)
	segment.memoryReleased += erasureCodingMemory
	if err != nil {
		segment.workersRemaining = 0
		sc.memoryManager.Return(pieceCompletedMemory)
		segment.memoryReleased += pieceCompletedMemory
		for i := 0; i < len(segment.physicalSegmentData); i++ {
			segment.physicalSegmentData[i] = nil
		}
		sc.log.Debug("Fetching physical data of a Segment failed:", err)
		return
	}

	// Sanity check - we should have at least as many physical data pieces as we
	// do elements in our piece usage.
	if len(segment.physicalSegmentData) < len(segment.pieceUsage) {
		sc.log.Error("not enough physical pieces to match the upload settings of the file")
		return
	}
	// Loop through the pieces and encrypt any that are needed, while dropping
	// any pieces that are not needed.
	for i := 0; i < len(segment.pieceUsage); i++ {
		if segment.pieceUsage[i] {
			segment.physicalSegmentData[i] = nil
		} else {
			cipherData, err := segment.fileEntry.CipherKey().Encrypt(segment.physicalSegmentData[i])
			// TODO 加密失败之后，是传明文还是忽略该segment
			if err != nil {
				sc.log.Debug("encrypt segment failed", err)
			} else {
				segment.physicalSegmentData[i] = cipherData
			}

		}
	}

	if pieceCompletedMemory > 0 {
		sc.memoryManager.Return(pieceCompletedMemory)
		segment.memoryReleased += pieceCompletedMemory
	}

	sc.distributeSegment(segment)
}

// retrieveLogicalSegmentData will get the raw data from disk if possible otherwise queueing a download
func (sc *StorageClient) retrieveLogicalSegmentData(segment *unfinishedUploadSegment) error {
	numParityPieces := float64(segment.piecesNeeded - segment.minimumPieces)
	minMissingPiecesToDownload := int(numParityPieces * RemoteRepairDownloadThreshold)
	download := segment.piecesCompleted+minMissingPiecesToDownload < segment.piecesNeeded

	// Download the segment if it's not on disk.
	if segment.fileEntry.LocalPath() == "" && download {
		return sc.managedDownloadLogicalSegmentData(segment)
	} else if segment.fileEntry.LocalPath() == "" {
		return errors.New("file not available locally")
	}

	// Try to read the file content from disk. If failed, go through download
	osFile, err := os.Open(string(segment.fileEntry.LocalPath()))
	if err != nil && download {
		return sc.managedDownloadLogicalSegmentData(segment)
	} else if err != nil {
		return errors.New("failed to open file locally")
	}
	defer osFile.Close()
	// TODO: Once we have enabled support for small Segments, we should stop
	// needing to ignore the EOF errors, because the Segment size should always
	// match the tail end of the file. Until then, we ignore io.EOF.
	buf := NewDownloadBuffer(segment.length, segment.fileEntry.SectorSize())
	sr := io.NewSectionReader(osFile, segment.offset, int64(segment.length))
	_, err = buf.ReadFrom(sr)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF && download {
		sc.log.Debug("failed to read file, downloading instead:", err)
		return sc.managedDownloadLogicalSegmentData(segment)
	} else if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		sc.log.Debug("failed to read file locally:", err)
		return errors.New("failed to read file locally")
	}
	segment.logicalSegmentData = buf.buf

	return nil
}

// cleanUpUploadSegment will check the state of the segment and perform any
// cleanup required. This can include returning memory and releasing the segment
// from the map of active segments in the segment heap.
func (sc *StorageClient) cleanUpUploadSegment(uc *unfinishedUploadSegment) {
	uc.mu.Lock()
	piecesAvailable := 0
	var memoryReleased uint64
	// Release any unnecessary pieces, counting any pieces that are
	// currently available.
	for i := 0; i < len(uc.pieceUsage); i++ {
		// Skip the piece if it's not available.
		if uc.pieceUsage[i] {
			continue
		}

		// If we have all the available pieces we need, release this piece.
		// Otherwise, mark that there's another piece available. This algorithm
		// will prefer releasing later pieces, which improves computational
		// complexity for erasure coding.
		if piecesAvailable >= uc.workersRemaining {
			memoryReleased += storage.SectorSize
			if len(uc.physicalSegmentData) < len(uc.pieceUsage) {
				// TODO handle this. Might happen if erasure coding the Segment failed.
			}
			uc.physicalSegmentData[i] = nil
			// Mark this piece as taken so that we don't double release memory.
			uc.pieceUsage[i] = true
		} else {
			piecesAvailable++
		}
	}

	// Check if the Segment needs to be removed from the list of active
	// Segments. It needs to be removed if the Segment is complete, but hasn't
	// yet been released.
	segmentComplete := uc.SegmentComplete()
	released := uc.released
	if segmentComplete && !released {
		uc.released = true
	}
	uc.memoryReleased += uint64(memoryReleased)
	totalMemoryReleased := uc.memoryReleased
	uc.mu.Unlock()

	// If there are pieces available, add the standby workers to collect them.
	// Standby workers are only added to the Segment when piecesAvailable is equal
	// to zero, meaning this code will only trigger if the number of pieces
	// available increases from zero. That can only happen if a worker
	// experiences an error during upload.
	if piecesAvailable > 0 {
		uc.managedNotifyStandbyWorkers()
	}
	// If required, return the memory to the renter.
	if memoryReleased > 0 {
		sc.memoryManager.Return(memoryReleased)
	}
	// If required, remove the segment from the set of repairing segments.
	if segmentComplete && !released {
		sc.managedUpdateUploadSegmentStuckStatus(uc)
		err := uc.fileEntry.Close()
		if err != nil {
			sc.log.Debug("file not closed after segment upload complete: %v %v", uc.fileEntry.DxPath(), err)
		}
		sc.uploadHeap.mu.Lock()
		delete(sc.uploadHeap.repairingSegments, uc.id)
		sc.uploadHeap.mu.Unlock()
	}
	// Sanity check - all memory should be released if the Segment is complete.
	if segmentComplete && totalMemoryReleased != uc.memoryNeeded {
		sc.log.Debug("No workers remaining, but not all memory released:", uc.workersRemaining, uc.piecesRegistered, uc.memoryReleased, uc.memoryNeeded)
	}
}

// setStuckAndClose sets the unfinishedUploadSegment's stuck status
func (sc *StorageClient) setStuckAndClose(uc *unfinishedUploadSegment, stuck bool) error {
	err := uc.fileEntry.SetStuckByIndex(int(uc.index), stuck)
	if err != nil {
		return fmt.Errorf("unable to update Segment stuck status for file %v: %v", uc.fileEntry.DxPath(), err)
	}

	go sc.fileSystem.InitAndUpdateDirMetadata(uc.fileEntry.DxPath())

	err = uc.fileEntry.Close()
	if err != nil {
		return fmt.Errorf("unable to close dx file %v", uc.fileEntry.DxPath())
	}
	return nil
}

// managedUpdateUploadSegmentStuckStatus checks to see if the repair was
// successful and then updates the Segment's stuck status
func (sc *StorageClient) managedUpdateUploadSegmentStuckStatus(uc *unfinishedUploadSegment) {
	// Grab necessary information from upload Segment under lock
	uc.mu.Lock()
	index := uc.id.index
	stuck := uc.stuck
	piecesCompleted := uc.piecesCompleted
	piecesNeeded := uc.piecesNeeded
	stuckRepair := uc.stuckRepair
	uc.mu.Unlock()

	// Determine if repair was successful
	successfulRepair := (1-RemoteRepairDownloadThreshold)*float64(piecesNeeded) <= float64(piecesCompleted)

	// Check if renter is shutting down
	var renterError bool
	select {
	case <-sc.tm.StopChan():
		renterError = true
	default:
		// Check that the renter is still online
		if !sc.Online() {
			renterError = true
		}
	}

	// If the repair was unsuccessful and there was a renter error then return
	if !successfulRepair && renterError {
		sc.log.Debug("WARN: repair unsuccessful for Segment", uc.id, "due to an error with the renter")
		return
	}
	// Log if the repair was unsuccessful
	if !successfulRepair {
		sc.log.Debug("WARN: repair unsuccessful, marking Segment", uc.id, "as stuck", float64(piecesCompleted)/float64(piecesNeeded))
	} else {
		sc.log.Debug("SUCCESS: repair successsful, marking Segment as non-stuck:", uc.id)
	}
	// Update Segment stuck status
	if err := uc.fileEntry.SetStuckByIndex(int(index), !successfulRepair); err != nil {
		sc.log.Debug("WARN: could not set Segment %v stuck status for file %v: %v", uc.id, uc.fileEntry.DxPath(), err)
	}

	dxPath := uc.fileEntry.DxPath()

	sc.fileSystem.InitAndUpdateDirMetadata(dxPath)

	// Check to see if the Segment was stuck and now is successfully repaired by
	// the stuck loop
	if stuck && successfulRepair && stuckRepair {
		// Signal the stuck loop that the Segment was successfully repaired
		sc.log.Debug("Stuck Segment", uc.id, "successfully repaired")
		select {
		case <-sc.tm.StopChan():
			sc.log.Debug("WARN: renter shut down before the stuck loop was signalled that the stuck repair was successful")
			return
		case sc.uploadHeap.stuckSegmentSuccess <- dxPath:
		}
	}
}
