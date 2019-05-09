package storagemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"time"
)

const (
	STD = iota
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
