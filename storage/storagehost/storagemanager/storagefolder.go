package storagemanager

import (
	"github.com/DxChainNetwork/godx/storage/storagehost/storagemanager/bitvector"
	"os"
)

type storageFolder struct {
	atomicUnavailable uint64

	// the Index of the storageFolder
	index uint16
	// the Path of the storageFolder
	path string
	// track if the storageSector is used or not
	usage []bitvector.BitVector

	// freeSectors represents the true status of the sector
	// where key is the id of sector, the value is the Index of sector
	freeSector map[sectorID]uint32

	sectorMeta *os.File
	sectorData *os.File
}
