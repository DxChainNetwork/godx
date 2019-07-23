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

	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// the information of a sector of a segment to download
type downloadSectorInfo struct {
	index uint64
	root  common.Hash
}

// represent a unfinished download task
type unfinishedDownloadSegment struct {

	// where to write the recovered logical data
	destination writeDestination
	erasureCode erasurecode.ErasureCoder

	// used to generate twofishgcm key seed
	segmentIndex uint64

	// maps from host id to the downloadSectorInfo
	segmentMap map[string]downloadSectorInfo

	// the length of the original data decoded
	segmentSize uint64

	// the length of segment to fetch
	fetchLength uint64

	// where is the logical segment being downloaded at
	fetchOffset uint64

	// the number of bytes every sector of remote file
	sectorSize uint64

	// where to write the completed data for the writer
	writeOffset int64

	latencyTarget time.Duration
	needsMemory   bool
	overdrive     uint32
	priority      uint64

	// which sectors in the segment were successfully downloaded
	completedSectors []bool

	// whether or not the segment successfully downloaded
	failed bool

	// the data used to recover the logical data
	physicalSegmentData [][]byte

	// which sectors in the segment are fetching
	sectorUsage []bool

	// how many sectors are successfully completed
	sectorsCompleted uint32

	// how many sectors are fetching
	sectorsRegistered uint32

	// whether or not the recovery has completed for this download task
	recoveryComplete bool

	// the number of workers still able to fetch the segment
	workersRemaining uint32

	// backup workers that can be used to download when other workers fail
	workersStandby []*worker

	// record how much memory allocated
	memoryAllocated uint64

	// used to update download progress
	download *download
	mu       sync.Mutex

	clientFile *dxfile.Snapshot
}

// remove a worker from the set of remaining workers in the uds
func (uds *unfinishedDownloadSegment) removeWorker() {
	uds.mu.Lock()
	uds.workersRemaining--
	uds.mu.Unlock()
	uds.cleanUp()
}

// cleanUp will check if the download has failed, and if not it will add
// any standby workers which need to be added.
//
// NOTE: calling cleanUp too many times is not harmful,
// however missing a call to cleanUp can lead to dealocks.
func (uds *unfinishedDownloadSegment) cleanUp() {

	// check if the segment is newly failed.
	uds.mu.Lock()
	if uds.workersRemaining+uds.sectorsCompleted < uds.erasureCode.MinSectors() && !uds.failed {
		uds.fail(errors.New("not enough workers to continue download"))
	}
	// return any excess memory.
	uds.returnMemory()

	// nothing to do if the segment has failed.
	if uds.failed {
		uds.mu.Unlock()
		return
	}

	// check whether standby workers are required.
	segmentComplete := uds.sectorsCompleted >= uds.erasureCode.MinSectors()
	desiredSectorsRegistered := uds.erasureCode.MinSectors() + uds.overdrive - uds.sectorsCompleted
	standbyWorkersRequired := !segmentComplete && uds.sectorsRegistered < desiredSectorsRegistered
	if !standbyWorkersRequired {
		uds.mu.Unlock()
		return
	}

	var standbyWorkers []*worker
	for i := 0; i < len(uds.workersStandby); i++ {
		standbyWorkers = append(standbyWorkers, uds.workersStandby[i])
	}
	uds.workersStandby = uds.workersStandby[:0]
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
	uds.download.fail(fmt.Errorf("segment %v failed: %v", uds.segmentIndex, err))
	uds.destination = nil
}

// returnMemory will check on the status of all the workers and sectors, and
// determine how much memory is safe to return to the client.
//
// NOTE: This should be called each time a worker returns, and also after the segment is recovered.
func (uds *unfinishedDownloadSegment) returnMemory() {

	// the maximum amount of memory is the sectors completed plus the number of workers remaining.
	maxMemory := uint64(uds.workersRemaining+uds.sectorsCompleted) * uds.sectorSize

	// if enough sectors have completed, max memory is the number of registered
	// sectors plus the number of completed sectors.
	if uds.sectorsCompleted >= uds.erasureCode.MinSectors() {
		maxMemory = uint64(uds.sectorsCompleted+uds.sectorsRegistered) * uds.sectorSize
	}

	// If the segment recovery has completed, the maximum number of sectors is the
	// number of registered.
	if uds.recoveryComplete {
		maxMemory = uint64(uds.sectorsRegistered) * uds.sectorSize
	}

	// return any memory we don't need.
	if uint64(uds.memoryAllocated) > maxMemory {
		uds.download.memoryManager.Return(uds.memoryAllocated - maxMemory)
		uds.memoryAllocated = maxMemory
	}
}

// marks the sector with sectorIndex as completed.
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

// recover data received from host into logical data, and write back to outer request destination
func (uds *unfinishedDownloadSegment) recoverLogicalData() error {

	// ensure cleanup occurs after the data is recovered, whether recovery succeeds or fails.
	defer uds.cleanUp()

	// NOTE: for not supporting partial encoding, we directly recover the whole sector
	// recover the sectors into the logical segment data.
	recoverWriter := new(bytes.Buffer)
	err := uds.erasureCode.Recover(uds.physicalSegmentData, int(uds.segmentSize), recoverWriter)
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
	start := uds.fetchOffset
	end := start + uds.fetchLength
	_, err = uds.destination.WriteAt(recoveredData[start:end], uds.writeOffset)
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
	uds.download.oneSegmentCompleted <- true
	if uds.download.segmentsRemaining == 0 {
		uds.download.markComplete()
		return err
	}
	return nil
}
