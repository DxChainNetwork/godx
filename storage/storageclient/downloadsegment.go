// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/storage"

	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// downloadSectorInfo contains all the information required to download and
// recover a sector of a segment from a host.
type downloadSectorInfo struct {
	index uint64
	root  common.Hash
}

// unfinishedDownloadSegment contains a segment for a download that is in progress.
type unfinishedDownloadSegment struct {
	// Fetch + Write instructions - read only or otherwise thread safe.
	destination downloadDestination // Where to write the recovered logical segment.
	erasureCode erasurecode.ErasureCoder
	masterKey   crypto.CipherKey

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticSegmentIndex uint64                        // Required for deriving the encryption keys for each sector.
	staticCacheID      string                        // Used to uniquely identify a segment in the segment cache.
	staticSegmentMap   map[string]downloadSectorInfo // Maps from host PubKey to the info for the sector associated with that host
	staticSegmentSize  uint64
	staticFetchLength  uint64 // Length within the logical segment to fetch.
	staticFetchOffset  uint64 // Offset within the logical segment that is being downloaded.
	staticSectorSize   uint64
	staticWriteOffset  int64 // Offset within the writer to write the completed data.

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticLatencyTarget time.Duration
	staticNeedsMemory   bool // Set to true if memory was not pre-allocated for this segment.
	staticOverdrive     uint32
	staticPriority      uint64

	// Download segment state - need mutex to access.
	completedSectors    []bool    // Which sectors were downloaded successfully.
	failed              bool      // Indicates if the segment has been marked as failed.
	physicalSegmentData [][]byte  // Used to recover the logical data.
	sectorUsage         []bool    // Which sectors are being actively fetched.
	sectorsCompleted    uint32    // Number of sectors that have successfully completed.
	sectorsRegistered   uint32    // Number of sectors that workers are actively fetching.
	recoveryComplete    bool      // Whether or not the recovery has completed and the segment memory released.
	workersRemaining    uint32    // Number of workers still able to fetch the segment.
	workersStandby      []*worker // Set of workers that are able to work on this download, but are not needed unless other workers fail.

	// Memory management variables.
	memoryAllocated uint64

	// The download object, mostly to update download progress.
	download *download
	mu       sync.Mutex

	// The SiaFile from which data is being downloaded.
	clientFile *dxfile.Snapshot
}

// removeWorker will remove a worker from the set of remaining workers in the uds
func (uds *unfinishedDownloadSegment) removeWorker() {
	uds.mu.Lock()
	uds.workersRemaining--
	uds.mu.Unlock()
	uds.cleanUp()
}

// cleanUp will check if the download has failed, and if not it will add
// any standby workers which need to be added.
//
// NOTE: Calling cleanUp too many times is not harmful,
// however missing a call to cleanUp can lead to dealocks.
func (uds *unfinishedDownloadSegment) cleanUp() {
	// Check if the segment is newly failed.
	uds.mu.Lock()
	if uds.workersRemaining+uds.sectorsCompleted < uds.erasureCode.MinSectors() && !uds.failed {
		uds.fail(errors.New("not enough workers to continue download"))
	}
	// Return any excess memory.
	uds.returnMemory()

	// Nothing to do if the segment has failed.
	if uds.failed {
		uds.mu.Unlock()
		return
	}

	// Check whether standby workers are required.
	segmentComplete := uds.sectorsCompleted >= uds.erasureCode.MinSectors()
	desiredsectorsRegistered := uds.erasureCode.MinSectors() + uds.staticOverdrive - uds.sectorsCompleted
	standbyWorkersRequired := !segmentComplete && uds.sectorsRegistered < desiredsectorsRegistered
	if !standbyWorkersRequired {
		uds.mu.Unlock()
		return
	}

	// Assemble a list of standby workers, release the uds lock, and then queue
	// the segment into the workers. The lock needs to be released early because
	// holding the uds lock and the worker lock at the same time is a deadlock
	// risk (they interact with eachother, call functions on eachother).
	var standbyWorkers []*worker
	for i := 0; i < len(uds.workersStandby); i++ {
		standbyWorkers = append(standbyWorkers, uds.workersStandby[i])
	}
	uds.workersStandby = uds.workersStandby[:0] // Workers have been taken off of standby.
	uds.mu.Unlock()
	for i := 0; i < len(standbyWorkers); i++ {
		standbyWorkers[i].queueDownloadSegment(uds)
	}
}

// fail will set the segment status to failed
func (uds *unfinishedDownloadSegment) fail(err error) {
	uds.failed = true
	uds.recoveryComplete = true
	for i := range uds.physicalSegmentData {
		uds.physicalSegmentData[i] = nil
	}
	uds.download.fail(fmt.Errorf("segment %v failed: %v", uds.staticSegmentIndex, err))
	uds.destination = nil
}

// returnMemory will check on the status of all the workers and sectors, and
// determine how much memory is safe to return to the client.
//
// NOTE: This should be called each time a worker returns, and also after the segment is recovered.
func (uds *unfinishedDownloadSegment) returnMemory() {

	// the maximum amount of memory is the sectors completed plus the number of workers remaining.
	maxMemory := uint64(uds.workersRemaining+uds.sectorsCompleted) * uds.staticSectorSize

	// If enough sectors have completed, max memory is the number of registered
	// sectors plus the number of completed sectors.
	if uds.sectorsCompleted >= uds.erasureCode.MinSectors() {
		maxMemory = uint64(uds.sectorsCompleted+uds.sectorsRegistered) * uds.staticSectorSize
	}

	// If the segment recovery has completed, the maximum number of sectors is the
	// number of registered.
	if uds.recoveryComplete {
		maxMemory = uint64(uds.sectorsRegistered) * uds.staticSectorSize
	}

	// Return any memory we don't need.
	if uint64(uds.memoryAllocated) > maxMemory {
		uds.download.memoryManager.Return(uds.memoryAllocated - maxMemory)
		uds.memoryAllocated = maxMemory
	}
}

// markSectorCompleted marks the sector with sectorIndex as completed.
func (uds *unfinishedDownloadSegment) markSectorCompleted(sectorIndex uint64) {
	uds.completedSectors[sectorIndex] = true
	uds.sectorsCompleted++
	completed := uint32(0)
	for _, b := range uds.completedSectors {
		if b {
			completed++
		}
	}
	if completed != uds.sectorsCompleted {
		log.Debug(fmt.Sprintf("sectors completed and completedSectors out of sync %v != %v",
			completed, uds.sectorsCompleted))
	}
}

// recoverLogicalData will take all of the sectors that have been
// downloaded and encode them into the logical data which is then written to the
// underlying writer for the download.
func (uds *unfinishedDownloadSegment) recoverLogicalData() error {

	// ensure cleanup occurs after the data is recovered, whether recovery succeeds or fails.
	defer uds.cleanUp()

	// calculate the number of bytes we need to recover
	btr := bytesToRecover(uds.staticFetchOffset, uds.staticFetchLength, uds.staticSegmentSize, uds.erasureCode)

	// recover the sectors into the logical segment data.
	recoverWriter := new(bytes.Buffer)
	err := uds.erasureCode.Recover(uds.physicalSegmentData, int(btr), recoverWriter)
	if err != nil {
		uds.mu.Lock()
		uds.fail(err)
		uds.mu.Unlock()
		return errors.New(fmt.Sprintf("unable to recover segment,error: %v", err))
	}

	// clear out the physical segments, we do not need them anymore.
	for i := range uds.physicalSegmentData {
		uds.physicalSegmentData[i] = nil
	}

	// get recovered data
	recoveredData := recoverWriter.Bytes()

	// write the bytes to the requested output.
	start := recoveredDataOffset(uds.staticFetchOffset, uds.erasureCode)
	end := start + uds.staticFetchLength
	_, err = uds.destination.WriteAt(recoveredData[start:end], uds.staticWriteOffset)
	if err != nil {
		uds.mu.Lock()
		uds.fail(err)
		uds.mu.Unlock()
		return errors.New(fmt.Sprintf("unable to write to download destination,error: %v", err))
	}
	recoverWriter = nil

	uds.mu.Lock()
	uds.recoveryComplete = true
	uds.mu.Unlock()

	// update the download and signal completion of this segment.
	uds.download.mu.Lock()
	defer uds.download.mu.Unlock()
	uds.download.segmentsRemaining--
	if uds.download.segmentsRemaining == 0 {
		uds.download.markComplete()
		return err
	}
	return nil
}

// returns the number of bytes we need to recover from the erasure coded segments.
func bytesToRecover(segmentFetchOffset, segmentFetchLength, segmentSize uint64, rs erasurecode.ErasureCoder) uint64 {

	// TODO: 确认下，是否不支持部分编码？？
	// If partialDecoding is not available we downloaded the whole sector and
	// recovered the whole segment.
	//if !rs.SupportsPartialEncoding() {
	//	return segmentSize
	//}

	// Else we need to calculate how much data we need to recover.
	recoveredSegmentSize := uint64(rs.MinSectors() * storage.SegmentSize)
	_, numSegments := segmentsForRecovery(segmentFetchOffset, segmentFetchLength, rs)
	return numSegments * recoveredSegmentSize

}

// recoveredDataOffset translates the fetch offset of the segment into the offset
// within the recovered data.
func recoveredDataOffset(segmentFetchOffset uint64, rs erasurecode.ErasureCoder) uint64 {

	// TODO: 确认下，是否不支持部分编码？？
	// If partialDecoding is not available we downloaded the whole sector and
	// recovered the whole chunk which means the offset and length are actually
	// equal to the segmentFetchOffset and segmentFetchLength.
	//if !rs.SupportsPartialEncoding() {
	//	return chunkFetchOffset
	//}

	// Else we need to adjust the offset a bit.
	recoveredSegmentSize := uint64(rs.MinSectors() * storage.SegmentSize)
	return segmentFetchOffset % recoveredSegmentSize
}
