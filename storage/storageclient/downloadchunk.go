// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// downloadPieceInfo contains all the information required to download and
// recover a piece of a segment from a host. It is a value in a map where the key
// is the file contract id.
type downloadPieceInfo struct {
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
	staticSegmentIndex uint64                       // Required for deriving the encryption keys for each piece.
	staticCacheID      string                       // Used to uniquely identify a segment in the segment cache.
	staticSegmentMap   map[string]downloadPieceInfo // Maps from host PubKey to the info for the piece associated with that host
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
	completedsectors    []bool    // Which sectors were downloaded successfully.
	failed              bool      // Indicates if the segment has been marked as failed.
	physicalSegmentData [][]byte  // Used to recover the logical data.
	pieceUsage          []bool    // Which sectors are being actively fetched.
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
	renterFile *dxfile.Snapshot

	// Caching related fields
	staticStreamCache *streamCache
}

// removeWorker will decrement a worker from the set of remaining workers
// in the uds. After a worker has been removed, the uds needs to be cleaned up.
func (uds *unfinishedDownloadSegment) removeWorker() {
	uds.mu.Lock()
	uds.workersRemaining--
	uds.mu.Unlock()
	uds.cleanUp()
}

// cleanUp will check if the download has failed, and if not it will add
// any standby workers which need to be added. Calling cleanUp too many
// times is not harmful, however missing a call to cleanUp can lead to
// dealocks.
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

// fail will set the segment status to failed. The physical segment memory will be
// wiped and any memory allocation will be returned to the client. The download
// as a whole will be failed as well.
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
// determine how much memory is safe to return to the client. This should be
// called each time a worker returns, and also after the segment is recovered.
func (uds *unfinishedDownloadSegment) returnMemory() {
	// The maximum amount of memory is the sectors completed plus the number of
	// workers remaining.
	maxMemory := uint64(uds.workersRemaining+uds.sectorsCompleted) * uds.staticSectorSize
	// If enough sectors have completed, max memory is the number of registered
	// sectors plus the number of completed sectors.
	if uds.sectorsCompleted >= uds.erasureCode.MinSectors() {
		// uds.sectorsRegistered is guaranteed to be at most equal to the number
		// of overdrive sectors, meaning it will be equal to or less than
		// initialMemory.
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
