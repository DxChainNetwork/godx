// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/crypto"
	"os"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

const (
	fileIDSize = 16

	// SectorSize is the size of a sector, which is 4MiB
	SectorSize = uint64(1 << 22)
)

type fileID [fileIDSize]byte

type (
	DxFile struct {
		// metaData is the persist metadata
		metaData *Metadata

		// hostAddresses is the map of host address to whether the address is used
		hostAddresses map[common.Address]*hostAddress

		// Segments is a list of segments the file is split into
		Segments []*segment

		// utils field
		deleted bool
		lock    sync.RWMutex
		ID      fileID
		wal     *writeaheadlog.Wal

		// filePath is full file path of
		filePath string

		//cached field
		erasureCode erasurecode.ErasureCoder
		cipherKey   crypto.CipherKey
	}

	// hostAddress is the in-memory data for host address.
	hostAddress struct {
		address common.Address
		used    bool
	}

	segment struct {
		sectors [][]*sector
		offset  uint64
		stuck   bool
	}

	sector struct {
		merkleRoot  common.Hash
		hostAddress common.Address
	}
)

// New creates a new dxfile
func New(filePath string, dxPath string, sourcePath string, wal *writeaheadlog.Wal, erasureCode erasurecode.ErasureCoder, cipherKey crypto.CipherKey, fileSize uint64, fileMode os.FileMode) (*DxFile, error) {
	currentTime := time.Now()
	minSectors, numSectors, extra := erasureCodeToParams(erasureCode)
	var id fileID
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
		PagesPerChunk:   segmentPersistNumPages(numSectors),
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
		metaData:      md,
		hostAddresses: make(map[common.Address]*hostAddress),
		deleted:       false,
		ID:            id,
		wal:           wal,
		filePath:      filePath,
		erasureCode:   erasureCode,
		cipherKey:     cipherKey,
	}
	df.Segments = make([]*segment, md.numSegments())
	for i := range df.Segments {
		df.Segments[i].sectors = make([][]*sector, numSectors)
	}
	return df, df.save()
}
