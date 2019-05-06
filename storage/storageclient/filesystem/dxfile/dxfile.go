// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"github.com/DxChainNetwork/godx/crypto"
	"os"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

const fileIDSize = 16

type fileID [fileIDSize]byte

type (
	DxFile struct {
		// metaData is the persist metadata
		metaData *Metadata

		// hostAddresses is the map of host address to whether the address is used
		hostAddresses map[common.Address]*hostAddress

		// dataSegments is a list of segments the file is split into
		dataSegments []*segment

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

	// hostAddress is the in-memory data for host address.
	hostAddress struct {
		Address common.Address
		Used    bool
	}

	segment struct {
		sectors [][]*sector
		stuck   bool
	}

	sector struct {
		MerkleRoot  common.Hash
		HostAddress common.Address
	}
)

// New creates a new dxfile
func New(filepath string, sourcePath string, wal *writeaheadlog.Wal, erasureCode erasurecode.ErasureCoder, masterKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*DxFile, error) {
	currentTime := time.Now()

}

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
