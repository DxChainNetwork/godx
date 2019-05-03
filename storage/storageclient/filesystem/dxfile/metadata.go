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
		// size related
		FileSize      int32
		SectorSize    int32 // ShardSize is the size for one shard, which is by default 4MiB
		PagesPerChunk uint8    // Number of physical pages per chunk. Determined by erasure coding algorithm

		// path related
		LocalPath string // Local path is the on-disk location for uploaded files
		DxPath    string // DxPath is the uploaded path of a file

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
		FileMode     os.FileMode // unix file mode

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

func (md Metadata) newErasureCode() (erasurecode.ErasureCoder, error) {
	switch md.ErasureCodeType {
	case erasurecode.ECTypeStandard:
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors)
	case erasurecode.ECTypeShard:
		shardSize := int(binary.LittleEndian.Uint32(md.ECExtra))
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors, shardSize)
	default:
		return nil, erasurecode.ErrInvalidECType
	}
}
