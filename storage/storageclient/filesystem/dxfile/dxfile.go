// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"sync"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

const fileIDSize = 16

type fileID [fileIDSize]byte

type (
	// DxFile is saved to disk as follows:
	// headerLength | ChunkOffset | PersistHeader      | dataSegment
	// 0:4          | 4:8         | 8:8+headerLength   | segmentOffset:
	DxFile struct {
		// headerLength is the size of the rlp string of header, which is put
		headerLength int32

		// segmentOffset is the offset of the first segment
		segmentOffset int32

		// header is the persist header is the header of the dxfile
		metaData Metadata

		// hostAddresses is the map of host address to whether the address is used
		hostAddresses map[common.Hash]bool

		// dataSegments is a list of segments the file is split into
		dataSegments []*PersistSegment

		// utils field
		deleted bool
		lock    sync.RWMutex
		ID      fileID
		wal     *writeaheadlog.Wal

		// filename is the file of the content locates
		filename string

		//cached field
		erasureCode erasurecode.ErasureCoder
	}

	// PersistHeader has two field: Metadata of fixed size, and HostAddresses of flexible size.
	PersistHeader struct {
		// Metadata includes all info related to dxfile that is ready to be flushed to data file
		Metadata

		// HostAddresses is a list of addresses that contains address and whether the host
		// is used
		HostAddresses []*PersistHostAddress
	}

	// PersistHostAddress is a combination of host address for a dxfile and whether the specific host is used in the dxfile
	// when encoding, the default rlp encoding algorithm is used
	PersistHostAddress struct {
		Address common.Hash
		Used    bool
	}

	// PersistSegment is the structure a dxfile is split into
	PersistSegment struct {
		// TODO: Check the ExtensionInfo could be actually removed
		Sectors [][]PersistSector // Sectors contains the recoverable message about the PersistSector in the PersistSegment
		Stuck   bool              // Stuck indicates whether the PersistSegment is Stuck or not
	}

	// PersistSector is the smallest unit of storage. It the erasure code encoded PersistSegment
	PersistSector struct {
		MerkleRoot  common.Hash
		HostOffset  int64 // hostOffset point to the location of host
	}
)

// segmentPersistSize is the helper function to calculate the persist size of the segment
func segmentPersistNumPages(numSectors uint32) int64 {
	sectorsSize := sectorPersistSize * numSectors
	sectorsSizeWithRedundancy := float64(sectorsSize) * (1 + redundancyRate)
	dataSize := segmentPersistOverhead + int(sectorsSizeWithRedundancy)
	numPages := dataSize / PageSize
	if dataSize%PageSize != 0 {
		numPages++
	}
	return int64(numPages)
}
