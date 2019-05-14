// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

const (
	fileIDSize = 16

	// SectorSize is the size of a sector, which is 4MiB
	SectorSize = uint64(1 << 22)
)

type (
	DxFile struct {
		// metadata is the persist metadata
		metadata *Metadata

		// hostTable is the map of host address to whether the address is used
		hostTable hostTable

		// TODO: create a indexed list structure for the segments
		// segments is a list of segments the file is split into
		segments []*segment

		// utils field
		deleted bool
		lock    sync.RWMutex
		ID      FileID
		wal     *writeaheadlog.Wal

		// filePath is full file path
		filePath string

		//cached field
		erasureCode erasurecode.ErasureCoder
		cipherKey   crypto.CipherKey
	}

	// hostTable is the map from host address to specific host info
	hostTable map[enode.ID]bool

	// segment is the Data for a segment, which is composed of several sectors
	segment struct {
		sectors [][]*sector
		index   uint64
		offset  uint64
		stuck   bool
	}

	// sector is the Data for a single sector, which has Data of merkle root and related host address
	sector struct {
		merkleRoot common.Hash
		hostID     enode.ID
	}

	FileID [fileIDSize]byte
)

// New creates a new dxfile
func New(filePath string, dxPath string, sourcePath string, wal *writeaheadlog.Wal, erasureCode erasurecode.ErasureCoder, cipherKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*DxFile, error) {
	currentTime := uint64(time.Now().Unix())
	minSectors, numSectors, extra := erasureCodeToParams(erasureCode)
	var id FileID
	_, err := rand.Read(id[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create a random id: %v", err)
	}
	md := &Metadata{
		Version:         "1.0.0",
		HostTableOffset: PageSize,
		SegmentOffset:   2 * PageSize,
		FileSize:        fileSize,
		SectorSize:      SectorSize - uint64(cipherKey.Overhead()),
		LocalPath:       sourcePath,
		DxPath:          dxPath,
		CipherKeyCode:   crypto.CipherCodeByName(cipherKey.CodeName()),
		CipherKey:       cipherKey.Key(),
		TimeModify:      currentTime,
		TimeCreate:      currentTime,
		FileMode:        fileMode,
		ErasureCodeType: erasureCode.Type(),
		MinSectors:      minSectors,
		NumSectors:      numSectors,
		ECExtra:         extra,
	}
	df := &DxFile{
		metadata:    md,
		hostTable:   make(map[enode.ID]bool),
		deleted:     false,
		ID:          id,
		wal:         wal,
		filePath:    filePath,
		erasureCode: erasureCode,
		cipherKey:   cipherKey,
	}
	df.segments = make([]*segment, md.numSegments())
	for i := range df.segments {
		df.segments[i] = &segment{sectors: make([][]*sector, numSectors), index: uint64(i)}
	}
	return df, df.saveAll()
}

// Rename rename the DxFile, remove the previous dxfile and create a new file to write content on
func (df *DxFile) Rename(newDxFile string, newDxFilename string) error {
	df.lock.RLock()
	defer df.lock.RUnlock()

	dir, _ := filepath.Split(newDxFilename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return df.rename(newDxFile, newDxFilename)
}

// AddSector add a sector to DxFile
func (df *DxFile) AddSector(address enode.ID, segmentIndex, sectorIndex int, merkleRoot common.Hash) error {
	df.lock.Lock()
	defer df.lock.Unlock()
	// if file already deleted, report an error
	if df.deleted {
		return fmt.Errorf("file already deleted")
	}
	// Update the hostTable
	df.hostTable[address] = true
	// Params validation
	if segmentIndex > len(df.segments) {
		return fmt.Errorf("segment index %d out of bound %d", segmentIndex, len(df.segments))
	}
	if uint32(sectorIndex) > df.metadata.NumSectors {
		return fmt.Errorf("sector index %d out of bound %d", sectorIndex, df.metadata.NumSectors)
	}
	df.segments[segmentIndex].sectors[sectorIndex] = append(df.segments[segmentIndex].sectors[sectorIndex],
		&sector{
			hostID:     address,
			merkleRoot: merkleRoot,
		})
	df.metadata.TimeAccess = unixNow()
	df.metadata.TimeModify = df.metadata.TimeAccess
	df.metadata.TimeUpdate = df.metadata.TimeAccess

	return df.saveSegments([]int{int(segmentIndex)})
}

// Delete delete the DxFile
func (df *DxFile) Delete() error {
	df.lock.RLock()
	defer df.lock.RUnlock()

	err := df.delete()
	if err != nil {
		return err
	}
	df.deleted = true
	return nil
}

// Deleted return the dxfile status of whether deleted
func (df *DxFile) Deleted() bool {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.deleted
}

// HostIDs return all hosts (including unused ones)
func (df *DxFile) HostIDs() []enode.ID {
	df.lock.RLock()
	defer df.lock.RUnlock()

	hosts := make([]enode.ID, 0, len(df.hostTable))
	for host := range df.hostTable {
		hosts = append(hosts, host)
	}
	return hosts
}

// MarkAllHealthyChunksAsUnstuck mark all health > 100 segments as unstuck
func (df *DxFile) MarkAllHealthySegmentsAsUnstuck(offline map[enode.ID]bool, goodForRenew map[enode.ID]bool) error {
	df.lock.Lock()
	defer df.lock.Unlock()

	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	indexes := make([]int, 0, len(df.segments))
	for i := range df.segments {
		if !df.segments[i].stuck {
			continue
		}
		segHealth := df.segmentHealth(i, offline, goodForRenew)
		if segHealth < 200 {
			continue
		}
		df.segments[i].stuck = false
		df.metadata.NumStuckSegments--
		indexes = append(indexes, i)
	}
	err := df.saveSegments(indexes)
	if err != nil {
		for _, i := range indexes {
			df.segments[i].stuck = true
			df.metadata.NumStuckSegments++
		}
	}
	return err
}

// MarkAllUnhealthySegmentsAsStuck mark all unhealthy segments (health smaller than repairHealthThreshold)
// as stuck
func (df *DxFile) MarkAllUnhealthySegmentsAsStuck(offline map[enode.ID]bool, goodForRenew map[enode.ID]bool) error {
	df.lock.Lock()
	defer df.lock.Unlock()

	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	indexes := make([]int, 0, len(df.segments))
	for i := range df.segments {
		if df.segments[i].stuck {
			continue
		}
		segHealth := df.segmentHealth(i, offline, goodForRenew)
		if segHealth >= repairHealthThreshold {
			continue
		}
		df.segments[i].stuck = true
		df.metadata.NumStuckSegments++
		indexes = append(indexes, i)
	}
	err := df.saveSegments(indexes)
	if err != nil {
		for _, i := range indexes {
			df.segments[i].stuck = false
			df.metadata.NumStuckSegments--
		}
	}
	return err
}

// NumSegments return the number of segments
func (df *DxFile) NumSegments() int {
	df.lock.Lock()
	defer df.lock.Unlock()

	return len(df.segments)
}

// SectorsOfSegmentIndex returns sectors of a specific segment with index
func (df *DxFile) SectorsOfSegmentIndex(index int) ([][]*sector, error) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if index > len(df.segments) {
		return nil, fmt.Errorf("index %d out of range", index)
	}
	return df.segments[index].sectors, nil
}

// Redundancy return the redundancy of a dxfile
func (df *DxFile) Redundancy(offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) uint32 {
	df.lock.RLock()
	defer df.lock.RUnlock()

	minRedundancy := uint32(math.MaxUint32)
	minRedundancyNoRenew := uint32(math.MaxUint32)
	for segIndex := range df.segments {
		numSectorsRenew, numSectorsNoRenew := df.goodSectors(segIndex, offlineMap, goodForRenewMap)
		redundancy := numSectorsRenew * 100 / df.metadata.MinSectors
		if redundancy < minRedundancy {
			minRedundancy = redundancy
		}
		redundancyNoRenew := numSectorsNoRenew / df.metadata.MinSectors
		if redundancyNoRenew < minRedundancyNoRenew {
			minRedundancyNoRenew = redundancyNoRenew
		}
	}
	if minRedundancy < 1 && minRedundancyNoRenew >= 1 {
		return 100
	} else if minRedundancy < 1 {
		return minRedundancyNoRenew
	}
	return minRedundancy
}

// SetStuck set a segment of index to the value of stuck
func (df *DxFile) SetStuckByIndex(index int, stuck bool) (err error) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}

	if stuck == df.segments[index].stuck {
		return nil
	}
	prevNumStuckChunks := df.metadata.NumStuckSegments
	prevStuck := df.segments[index].stuck
	defer func() {
		if err != nil {
			df.metadata.NumStuckSegments = prevNumStuckChunks
			df.segments[index].stuck = prevStuck
		}
	}()
	df.segments[index].stuck = stuck
	if stuck {
		df.metadata.NumStuckSegments++
	} else {
		df.metadata.NumStuckSegments--
	}

	err = df.saveSegments([]int{index})
	return
}

// GetStuckByIndex get the stuck status of the indexed segment
func (df *DxFile) GetStuckByIndex(index int) bool {
	df.lock.Lock()
	defer df.lock.Unlock()

	return df.segments[index].stuck
}

// UID return the id of the dxfile
func (df *DxFile) UID() FileID {
	return df.ID
}

// UploadedBytes return the uploaded bytes
func (df *DxFile) UploadedBytes() uint64 {
	df.lock.RLock()
	defer df.lock.RUnlock()
	var uploaded uint64
	for _, segment := range df.segments {
		for _, sectors := range segment.sectors {
			uploaded += SectorSize * uint64(len(sectors))
		}
	}
	return uploaded
}

// UploadProgress return the upload process of a dxfile
func (df *DxFile) UploadProgress() float64 {
	if df.FileSize() == 0 {
		return 100
	}
	uploaded := df.UploadedBytes()
	desired := uint64(df.NumSegments()) * SectorSize * uint64(df.metadata.NumSectors)
	return math.Min(100*(float64(uploaded)/float64(desired)), 100)
}

// UpdateUsedHosts update host table of the dxfile
func (df *DxFile) UpdateUsedHosts(used []enode.ID) error {
	df.lock.Lock()
	defer df.lock.Unlock()
	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	prevHostTable := make(map[enode.ID]bool)
	for key, value := range df.hostTable {
		prevHostTable[key] = value
	}

	usedMap := make(map[enode.ID]struct{})
	for _, host := range used {
		usedMap[host] = struct{}{}
	}
	for host := range df.hostTable {
		_, exist := usedMap[host]
		df.hostTable[host] = exist
	}
	err := df.saveHostTableUpdate()
	if err != nil {
		df.hostTable = prevHostTable
	}
	return err
}

// pruneSegment try to prune sectors from unused hosts to fit in the page size
func (df *DxFile) pruneSegment(segIndex int) {
	maxSegmentSize := segmentPersistNumPages(df.metadata.NumSectors) * PageSize
	maxSectors := (maxSegmentSize - segmentPersistOverhead) / sectorPersistSize
	maxSectorsPerIndex := maxSectors / uint64(len(df.segments[segIndex].sectors))

	var numSectors int
	for _, sectors := range df.segments[segIndex].sectors {
		numSectors += len(sectors)
	}
	if uint64(numSectors) <= maxSectors {
		// no need to prune the sector
		return
	}

	for i, sectors := range df.segments[segIndex].sectors {
		var newSectors []*sector
		for _, sector := range sectors {
			if uint64(len(newSectors)) >= maxSectorsPerIndex {
				break
			}
			if df.hostTable[sector.hostID] {
				newSectors = append(newSectors, sector)
			}
		}
		df.segments[segIndex].sectors[i] = newSectors
	}
}

// ApplyCachedHealthMetadata apply the cachedHealthMetadata to the DxFile
func (df *DxFile) ApplyCachedHealthMetadata(metadata CachedHealthMetadata) error {
	df.lock.Lock()
	defer df.lock.Unlock()

	var numStuckSegments uint32
	for _, seg := range df.segments {
		if seg.stuck {
			numStuckSegments++
		}
	}
	df.metadata.Health = metadata.Health
	df.metadata.NumStuckSegments = numStuckSegments
	df.metadata.LastRedundancy = metadata.Redundancy
	df.metadata.StuckHealth = metadata.StuckHealth
	df.metadata.TimeUpdate = unixNow()

	return df.saveMetadata()
}
