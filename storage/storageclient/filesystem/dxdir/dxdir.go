package dxdir

import (
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"sync"
)

const dirFileName = ".dxdir"

type (
	// DxDir is the data structure for the directory for the meta info for a directory.
	DxDir struct {
		// metadata
		metadata *Metadata

		// utilities
		deleted bool
		lock sync.Mutex
		wal *writeaheadlog.Wal

		// dirPath is the actual path the DxDir locates
		dirPath string
	}

	// Metadata is the necessary metadata to be saved in DxDir
	Metadata struct {
		// Total number of files in directory and its subdirectories
		NumFiles uint64

		// Total size of the directory and its subdirectories
		TotalSize uint64

		// Health is the min Health all files and subdirectories
		Health uint32

		// StuckHealth is the min StuckHealth for all files and subdirectories
		StuckHealth uint32

		// MinRedundancy is the minimum redundancy
		MinRedundancy uint32

		// TimeLastHealthCheck is the last health check time
		TimeLastHealthCheck uint64

		// TimeModify is the last content modification time
		TimeModify uint64

		// NumStuckSegments is the total number of segments that is stuck
		NumStuckSegments uint64

		// DxPath is the DxPath which is the actual path related to the root directory
		DxPath string
	}
)

// new create a DxDir with representing the dirPath metadata
func New(dxPath string, dirPath string, wal *writeaheadlog.Wal) (*DxDir, error) {

}
