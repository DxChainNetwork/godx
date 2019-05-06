// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"encoding/binary"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"os"
	"time"
)

type (
	// Metadata is the Metadata of a user uploaded file.
	Metadata struct {
		// storage related
		HostTableOffset int32
		SegmentOffset   int32

		// size related
		FileSize      uint64
		SectorSize    uint64 // ShardSize is the size for one shard, which is by default 4MiB
		PagesPerChunk uint64 // Number of physical pages per chunk. Determined by erasure coding algorithm

		// path related
		LocalPath string // Local path is the on-disk location for uploaded files
		DxPath    string // DxPath is the user specified dxpath

		// Encryption
		// TODO: Add the key for sharing
		CipherKeyCode uint8  // cipher key code defined in cipher package
		CipherKey     []byte // Key used to encrypt pieces

		// Time fields. most of unix timestamp
		TimeModify time.Time // time of last content modification
		TimeUpdate time.Time // time of last Metadata update
		TimeAccess time.Time // time of last access
		TimeCreate time.Time // time of file creation

		// Repair loop fields
		Health           float64   // Worst health of the file's unstuck chunk
		StuckHealth      float64   // Worst health of the file's Stuck chunk
		LastHealthCheck  time.Time // Time of last health check happenning
		NumStuckChunks   uint32    // Number of Stuck chunks
		RecentRepairTime time.Time // Timestamp of last chunk repair
		LastRedundancy   float64   // File redundancy from last check

		// File related
		FileMode os.FileMode // unix file mode

		// Erasure code field
		ErasureCodeType uint8  // the code for the specific erasure code
		MinSectors      uint32 // params for erasure coding. The number of slice raw data split into.
		NumSectors      uint32 // params for erasure coding. The number of total Sectors
		ECExtra         []byte // extra parameters for erasure code

		// Version control for fork
		Version string

		// TODO: Check whether removed field is really necessary when finished
		//  Removed fields: UserId, GroupId, ChunkMetadataSize, sharingKey.
	}

	// BubbleMetaData is the Metadata to be bubbled
	UpdateMetaData struct {
		Health           float64
		StuckHealth      float64
		LastHealthCheck  time.Time
		NumStuckChunks   uint64
		RecentRepairTime time.Time
		LastRedundancy   float64
		Size             uint64
		TimeModify       time.Time
	}
)

// newErasureCode is the helper function to create the erasureCoder based on metadata params
func (md Metadata) newErasureCode() (erasurecode.ErasureCoder, error) {
	switch md.ErasureCodeType {
	case erasurecode.ECTypeStandard:
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors)
	case erasurecode.ECTypeShard:
		var shardSize int
		if len(md.ECExtra) >= 4 {
			shardSize = int(binary.LittleEndian.Uint32(md.ECExtra))
		} else {
			shardSize = erasurecode.EncodedShardUnit
		}
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors, shardSize)
	default:
		return nil, erasurecode.ErrInvalidECType
	}
}

// erasureCodeToParams is the the helper function to interpret the erasureCoder to params
// return minSectors, numSectors, and extra
func erasureCodeToParams(ec erasurecode.ErasureCoder) (uint32, uint32, []byte) {
	minSectors := ec.MinSectors()
	numSectors := ec.NumSectors()
	switch ec.Type() {
	case erasurecode.ECTypeStandard:
		return minSectors, numSectors, nil
	case erasurecode.ECTypeShard:
		extra := ec.Extra()
		extraBytes := make([]byte, 4)
		shardSize := extra[0].(int)
		binary.LittleEndian.PutUint32(extraBytes, uint32(shardSize))
		return minSectors, numSectors, extraBytes
	default:
		panic("erasure code type not recognized")
	}
}

// segmentSize is the helper function to calculate the segment size based on metadata info
func (md Metadata) segmentSize() uint64 {
	return md.SectorSize * uint64(md.MinSectors)
}

// numSegments is the number of segments of a dxfile based on metadata info
func (md Metadata) numSegments() uint64 {
	num := md.FileSize / md.segmentSize()
	if md.FileSize/md.segmentSize() != 0 || num == 0 {
		num++
	}
	return num
}
