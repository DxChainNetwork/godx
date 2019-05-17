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
	// fileIDSize is the size of fileID type
	fileIDSize = 16

	// SectorSize is the size of a Sector, which is 4MiB
	SectorSize = uint64(1 << 22)

	// Version is the version of dxfile
	Version = "1.0.0"
)

type (
	// DxFile is the type of user uploaded DxFile
	DxFile struct {
		// metadata is the persist metadata
		metadata *Metadata

		// hostTable is the map of host address to whether the address is used
		hostTable hostTable

		// segments is a list of segments the file is split into
		segments []*Segment

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

	// Segment is the Data for a Segment, which is composed of several Sectors
	Segment struct {
		Sectors [][]*Sector
		Index   uint64
		Stuck   bool
		offset  uint64
	}

	// Sector is the Data for a single Sector, which has Data of merkle root and related host address
	Sector struct {
		MerkleRoot common.Hash
		HostID     enode.ID
	}

	// FileID is the ID for a DxFile
	FileID [fileIDSize]byte
)

// New creates a new dxfile.
// filePath is the file where DxFile locates, dxPath is the user input dxPath.
// sourcePath is the file of the original data. wal is the writeaheadlog.
// erasureCode is the erasure coder for encoding. cipherKey is the key for encryption.
// fileSize is the size of the original data file. fileMode is the file privilege mode (e.g. 0777)
func New(filePath string, dxPath string, sourcePath string, wal *writeaheadlog.Wal, erasureCode erasurecode.ErasureCoder, cipherKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*DxFile, error) {
	currentTime := uint64(time.Now().Unix())
	// create the params for erasureCode and cipherKey
	minSectors, numSectors, extra := erasureCodeToParams(erasureCode)
	cipherKeyCode := crypto.CipherCodeByName(cipherKey.CodeName())
	// create a random FileID
	var id FileID
	_, err := rand.Read(id[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create a random id: %v", err)
	}
	// create a metadata
	md := &Metadata{
		Version:         Version,
		HostTableOffset: PageSize,
		SegmentOffset:   2 * PageSize,
		FileSize:        fileSize,
		SectorSize:      SectorSize - uint64(cipherKey.Overhead()),
		LocalPath:       sourcePath,
		DxPath:          dxPath,
		CipherKeyCode:   cipherKeyCode,
		CipherKey:       cipherKey.Key(),
		TimeModify:      currentTime,
		TimeCreate:      currentTime,
		FileMode:        fileMode,
		ErasureCodeType: erasureCode.Type(),
		MinSectors:      minSectors,
		NumSectors:      numSectors,
		ECExtra:         extra,
	}
	// create the DxFile
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
	// initialize the segments.
	df.segments = make([]*Segment, md.numSegments())
	for i := range df.segments {
		df.segments[i] = &Segment{Sectors: make([][]*Sector, numSectors), Index: uint64(i)}
	}
	return df, df.saveAll()
}

// Rename rename the DxFile, remove the previous dxfile and create a new file
func (df *DxFile) Rename(newDxFile string, newDxFilename string) error {
	df.lock.RLock()
	defer df.lock.RUnlock()

	dir, _ := filepath.Split(newDxFilename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return df.rename(newDxFile, newDxFilename)
}

// AddSector add a Sector to DxFile to the location specified by segmentIndex and sectorIndex.
// The sector content is filled by address and merkleRoot.
func (df *DxFile) AddSector(address enode.ID, merkleRoot common.Hash, segmentIndex, sectorIndex int) error {
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
		return fmt.Errorf("segment Index %d out of bound %d", segmentIndex, len(df.segments))
	}
	if uint32(sectorIndex) > df.metadata.NumSectors {
		return fmt.Errorf("sector Index %d out of bound %d", sectorIndex, df.metadata.NumSectors)
	}
	df.segments[segmentIndex].Sectors[sectorIndex] = append(df.segments[segmentIndex].Sectors[sectorIndex],
		&Sector{
			HostID:     address,
			MerkleRoot: merkleRoot,
		})
	df.metadata.TimeAccess = unixNow()
	df.metadata.TimeModify = df.metadata.TimeAccess
	df.metadata.TimeUpdate = df.metadata.TimeAccess

	return df.saveSegments([]int{int(segmentIndex)})
}

// Delete delete the DxFile. The function delete the DxFile on disk, and also mark
// df.deleted as true
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

// Deleted return the dxfile status of whether it is deleted
func (df *DxFile) Deleted() bool {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.deleted
}

// HostIDs return all hosts in df.hostTable(including unused ones)
func (df *DxFile) HostIDs() []enode.ID {
	df.lock.RLock()
	defer df.lock.RUnlock()

	hosts := make([]enode.ID, 0, len(df.hostTable))
	for host := range df.hostTable {
		hosts = append(hosts, host)
	}
	return hosts
}

// MarkAllHealthySegmentsAsUnstuck mark all health > 100 segments as unstuck
func (df *DxFile) MarkAllHealthySegmentsAsUnstuck(offline map[enode.ID]bool, goodForRenew map[enode.ID]bool) error {
	df.lock.Lock()
	defer df.lock.Unlock()
	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	// loop over segments and check health. If health is 200, mark the segment as unstuck
	indexes := make([]int, 0, len(df.segments))
	for i := range df.segments {
		if !df.segments[i].Stuck {
			continue
		}
		segHealth := df.segmentHealth(i, offline, goodForRenew)
		if segHealth < 200 {
			continue
		}
		df.segments[i].Stuck = false
		df.metadata.NumStuckSegments--
		indexes = append(indexes, i)
	}
	// save the segments. If error happens, revert.
	err := df.saveSegments(indexes)
	if err != nil {
		for _, i := range indexes {
			df.segments[i].Stuck = true
			df.metadata.NumStuckSegments++
		}
	}
	return err
}

// MarkAllUnhealthySegmentsAsStuck mark all unhealthy segments (health smaller than repairHealthThreshold)
// as Stuck
func (df *DxFile) MarkAllUnhealthySegmentsAsStuck(offline map[enode.ID]bool, goodForRenew map[enode.ID]bool) error {
	df.lock.Lock()
	defer df.lock.Unlock()
	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	// loop over segments. If segment health is smaller than repairHealthThreshold,
	// mark the segment as stuck.
	indexes := make([]int, 0, len(df.segments))
	for i := range df.segments {
		if df.segments[i].Stuck {
			continue
		}
		segHealth := df.segmentHealth(i, offline, goodForRenew)
		if segHealth >= repairHealthThreshold {
			continue
		}
		df.segments[i].Stuck = true
		df.metadata.NumStuckSegments++
		indexes = append(indexes, i)
	}
	// save the segments. If error happens, mark the segment as stuck
	err := df.saveSegments(indexes)
	if err != nil {
		for _, i := range indexes {
			df.segments[i].Stuck = false
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

// SectorsOfSegmentIndex returns Sectors of a specific Segment with Index.
// Notice the returned sector is the deep copy of the sectors specified by index
func (df *DxFile) SectorsOfSegmentIndex(index int) ([][]*Sector, error) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if index > len(df.segments) {
		return nil, fmt.Errorf("index %d out of range", index)
	}
	return copySectors(df.segments[index]), nil
}

// Redundancy return the redundancy of a dxfile.
// The redundancy is goodSectors * 100 / minSectors.
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

// SetStuckByIndex set a Segment of Index to the value of Stuck.
func (df *DxFile) SetStuckByIndex(index int, stuck bool) (err error) {
	df.lock.Lock()
	defer df.lock.Unlock()

	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}

	if stuck == df.segments[index].Stuck {
		return nil
	}
	// if error happens, revert the change
	prevNumStuckSegments := df.metadata.NumStuckSegments
	prevStuck := df.segments[index].Stuck
	defer func() {
		if err != nil {
			df.metadata.NumStuckSegments = prevNumStuckSegments
			df.segments[index].Stuck = prevStuck
		}
	}()
	df.segments[index].Stuck = stuck
	if stuck {
		df.metadata.NumStuckSegments++
	} else {
		df.metadata.NumStuckSegments--
	}

	err = df.saveSegments([]int{index})
	return
}

// GetStuckByIndex get the Stuck status of the indexed Segment
func (df *DxFile) GetStuckByIndex(index int) bool {
	df.lock.Lock()
	defer df.lock.Unlock()

	return df.segments[index].Stuck
}

// UID return the id of the dxfile
func (df *DxFile) UID() FileID {
	return df.ID
}

// UploadedBytes return the uploaded bytes. The uploaded bytes is calculated by number of
// sectors in df.segments
func (df *DxFile) UploadedBytes() uint64 {
	df.lock.RLock()
	defer df.lock.RUnlock()
	var uploaded uint64
	for _, segment := range df.segments {
		for _, sectors := range segment.Sectors {
			uploaded += SectorSize * uint64(len(sectors))
		}
	}
	return uploaded
}

// UploadProgress return the upload process of a dxfile.
// The upload progress is calculated based on uploadedByte divided by total bytes to upload
func (df *DxFile) UploadProgress() float64 {
	if df.FileSize() == 0 {
		return 100
	}
	uploaded := df.UploadedBytes()
	desired := uint64(df.NumSegments()) * SectorSize * uint64(df.metadata.NumSectors)
	return math.Min(100*(float64(uploaded)/float64(desired)), 100)
}

// UpdateUsedHosts update host table of the dxfile.
// hosts in df.hostTable exist in used slice are marked as used, rest are marked as unused
func (df *DxFile) UpdateUsedHosts(used []enode.ID) error {
	df.lock.Lock()
	defer df.lock.Unlock()
	if df.deleted {
		return fmt.Errorf("file %v is deleted", df.metadata.DxPath)
	}
	// record the hostTable before the change. If an error happens during the change,
	// revert to the hostTable before the change
	prevHostTable := make(map[enode.ID]bool)
	for key, value := range df.hostTable {
		prevHostTable[key] = value
	}
	// change the input used slice to a map
	usedMap := make(map[enode.ID]struct{})
	for _, host := range used {
		usedMap[host] = struct{}{}
	}
	// If the host exist in used slice, used = true
	// if not exist in used slice, used = false
	for host := range df.hostTable {
		_, exist := usedMap[host]
		df.hostTable[host] = exist
	}
	// save the updates. If error happens, revert.
	err := df.saveHostTableUpdate()
	if err != nil {
		df.hostTable = prevHostTable
	}
	return err
}

// pruneSegment try to prune Sectors from unused hosts to fit in the page size.
// The size limit is defined by segmentPersistNumPages * PageSize.
// After prune, all sectors' hosts must be used in hostTable
func (df *DxFile) pruneSegment(segIndex int) {
	maxSegmentSize := segmentPersistNumPages(df.metadata.NumSectors) * PageSize
	maxSectors := (maxSegmentSize - segmentPersistOverhead) / sectorPersistSize
	// Max number of sectors per sector index
	maxSectorsPerIndex := maxSectors / uint64(len(df.segments[segIndex].Sectors))
	// calculate the total number of sectors. If already fit in, no need to prune.
	// simply return.
	var numSectors int
	for _, sectors := range df.segments[segIndex].Sectors {
		numSectors += len(sectors)
	}
	if uint64(numSectors) <= maxSectors {
		// no need to prune the Sector
		return
	}
	// Remove sectors with unused hosts, until the segment could fit in the pages.
	for i, sectors := range df.segments[segIndex].Sectors {
		var newSectors []*Sector
		for _, sector := range sectors {
			if uint64(len(newSectors)) >= maxSectorsPerIndex {
				break
			}
			if df.hostTable[sector.HostID] {
				newSectors = append(newSectors, sector)
			}
		}
		df.segments[segIndex].Sectors[i] = newSectors
	}
}

// ApplyCachedHealthMetadata apply the cachedHealthMetadata to the DxFile
func (df *DxFile) ApplyCachedHealthMetadata(metadata CachedHealthMetadata) error {
	df.lock.Lock()
	defer df.lock.Unlock()

	var numStuckSegments uint32
	for _, seg := range df.segments {
		if seg.Stuck {
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
