package storagemanager

import (
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

const (
	// STD and TST is the mode of setting for initialization
	// of to fits both the testing and standard requirement
	STD = iota

	// TST is the mode of testing, the setting would be set to
	// value of testing to increase the testing speed
	TST

	configFile = "config.sys"
	walFile    = "wal.log"

	configFileTmp = "configTmp.sys"
	walFileTmp    = "walTmp.log"

	sectorMetaFileName = "sectormeta.dat"
	sectorDataFileName = "sectordata.dat"

	commitFrequency = 500 * time.Millisecond

	granularity = 64
)

var (
	// configMetadata is the metadata for indicating the config file
	configMetadata = common.Metadata{
		Header:  "Gdx storage manager config",
		Version: "1.0",
	}

	// wal file
	walMetadata = common.Metadata{
		Header:  "Gdx storage manager WAL",
		Version: "1.0",
	}

	// Mode indicate the current mode
	Mode int
	// MockFails to disrupt the system
	MockFails map[string]bool
	// SectorSize is the size of a sector
	SectorSize uint64
	// SectorMetaSize is the size of the sector's metadata
	SectorMetaSize uint64
	// MaxSectorPerFolder is the number of sector every folder would contain
	MaxSectorPerFolder uint64
	// MinSectorPerFolder is the minimum number of sector every folder would contain
	MinSectorPerFolder uint64
	// MaxStorageFolders is the limitation of maximum folder
	MaxStorageFolders uint64
	// configLock manage save and load of the temporary config file, make sure it won't lead race between thread
	configLock sync.Mutex
)

// SELECT is a map recording the setting value to used in each mode
type SELECT map[int]interface{}

// init first initialize the settings to a standard mode, if further
// testing environment is needed, buildSetting would be called again to
// switch the setting value
func init() {
	buildSetting(STD)
}

// buildSetting help the initializer init the global vars. In order to fits
// the data comfortable for both standard mode and testing mode, this function
// may be called to switch data for caller's need
func buildSetting(mode int) {
	Mode = mode
	MockFails = make(map[string]bool)

	SectorSize = SELECT{
		STD: uint64(1 << 22),
		TST: uint64(1 << 12),
	}[mode].(uint64)

	MaxSectorPerFolder = SELECT{
		STD: uint64(1 << 32),
		TST: uint64(1 << 12),
	}[mode].(uint64)

	MinSectorPerFolder = SELECT{
		STD: uint64(1 << 6),
		TST: uint64(1 << 6),
	}[mode].(uint64)

	MaxStorageFolders = SELECT{
		STD: uint64(1 << 16),
		TST: uint64(1 << 3),
	}[mode].(uint64)

	SectorMetaSize = 14
}
