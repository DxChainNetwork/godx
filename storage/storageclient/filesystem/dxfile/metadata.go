// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"math/big"
	"os"
	"time"
)

type (
	// metadata is the metadata of a user uploaded file.
	metadata struct {
		// size related
		FileSize      *big.Int `json:"filesize"`      // FileSize shall not change once created
		ShardSize     *big.Int `json:"shardsize"`     // ShardSize is the size for one shard, which is by default 4MiB
		PagesPerChunk uint8    `json:"pagesperChunk"` // Number of physical pages per chunk. Determined by erasure coding algorithm

		// path related
		LocalPath string `json:"localpath"` // Local path is the on-disk location for uploaded files
		DxPath    string `json:"dxpath"`    // DxPath is the uploaded path of a file

		// Encryption
		// TODO: Add the key for sharing
		CipherKey     []byte `json:"masterkey"`     // Key used to encrypt pieces
		CipherKeyCode uint8  `json:"masterkeytype"` // cipher key code defined in cipher package

		// Time fields. most of unix timestamp
		TimeModify time.Time `json:"timemodify"` // time of last content modification
		TimeUpdate time.Time `json:"timechange"` // time of last metadata update
		TimeAccess time.Time `json:"timeaccess"` // time of last access
		TimeCreate time.Time `json:"timecreate"` // time of file creation

		// Repair loop fields
		Health           float64   `json:"health"`           // Worst health of the file's unstuck chunk
		StuckHealth      float64   `json:"stuckhealth"`      // Worst health of the file's stuck chunk
		LastHealthCheck  time.Time `json:"lasthealthcheck"`  // Time of last health check happenning
		NumStuckChunks   uint64    `json:"numstuckchunks"`   // Number of stuck chunks
		RecentRepairTime time.Time `json:"recentrepairtime"` // Timestamp of last chunk repair
		LastRedundancy   float64   `json:"lastredundancy"`   // File redundancy from last check

		// File related
		FileMode     os.FileMode `json:"filemode"`     // unix file mode
		PubKeyOffset uint64      `json:"pubkeyoffset"` // the offset of the publicKey table

		// Erasure code field
		ErasureCodeType uint8  `json:"erasurecode"`    // the code for the specific erasure code
		NumDataPiece    uint64 `json:"numdatapiece"`   // params for erasure coding. The number of slice raw data split into.
		NumParityPiece  uint64 `json:"numparitypiece"` // params for erasure coding. The number of parity pieces

		// Version control for fork
		Version string `json:"version"`

		// TODO: Check whether removed field is really necessary when finished
		//  Removed fields: UserId, GroupId, ChunkMetadataSize, sharingKey.
	}

	// BubbleMetaData is the metadata to be bubbled
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
