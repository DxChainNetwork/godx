package storagemanager

import "github.com/DxChainNetwork/godx/common"

const (
	// STD and TST is the mode of setting for initialization
	// of to fits both the testing and standard requirement
	STD = iota
	TST

	// configFile is name of the config file
	configFile = "config.sys"
	// walFileTmp is the file which record the change of the system
	walFile = "wal.log"
	// sectorMetaFileName is the name of sector metadata file
	// which store the sector information
	sectorMetaFileName = "sectormeta.dat"
	// sectorDataFileName is the name of sector file,
	// which store the actual data of sector
	sectorDataFileName = "sectordata.dat"
	// specify the granularity of storage, means 64 sector would form
	// as a granularity
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
)

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
