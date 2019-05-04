package storagemanager

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage/storagehost/storagemanager/bitvector"
	"os"
	"path/filepath"
)

type storageManager struct {
	persistDir string

	// TODO: currently use bytes
	sectorSalt [32]byte
	folders    map[uint16]*storageFolder
	sectors    map[sectorID]*storageSector
	log        log.Logger
}

func New(persistDir string, mode ...int) (*storageManager, error) {
	if mode != nil && len(mode) != 0 {
		buildSetting(mode[0])
	}
	return newStorageManager(persistDir)
}

// TODO: currently mock the close of storage manager
func (cm *storageManager) Close() error {
	return nil
}

// AddStorageFolder add the a storage folder by given path and size
// make sure the size is between the range and is multiple of granularity
func (sm *storageManager) AddStorageFolder(path string, size uint64) error {
	var err error

	// check the number of sectors
	sectors := size / SectorSize

	// check if the folder is valid
	if err = sm.isValidNewFolder(sectors, size, path); err != nil {
		return err
	}

	// make the dir. If the dir is already exist or the status is not health
	// the circumstance are already be handled in the MkdirAll function,
	// TODO: test use path as file, existing dir
	if err = os.MkdirAll(path, 0700); err != nil {
		return err
	}

	// create the folder instance
	sf := &storageFolder{
		path:       path,
		usage:      make([]bitvector.BitVector, sectors/granularity),
		freeSector: make(map[sectorID]uint32),
	}

	// TODO: mock make the sector files
	sf.sectorMeta, err = os.Create(filepath.Join(path, sectorMetaFileName))
	if err != nil {
		fmt.Println(err.Error())
	}
	sf.sectorData, err = os.Create(filepath.Join(path, sectorDataFileName))
	if err != nil {
		fmt.Println(err.Error())
	}

	// find a index for folder
	index, err := randomFolderIndex(sm.folders)
	if err != nil {
		return err
	}

	// use the storage index
	sf.index = index

	sm.folders[index] = sf

	//spew.Dump(sm)
	return nil
}

// addStorageFolder help to truncate and create the data and metadata file,
// and then record the instance to file
func (sm *storageManager) addStorageFolder(sf *storageFolder) error {
	var err error
	sectors := uint64(len(sf.usage) * granularity)

	// find a index for folder
	sf.index, err = randomFolderIndex(sm.folders)
	if err != nil {
		return err
	}

	// create the metadata file
	sf.sectorMeta, err = os.Create(filepath.Join(sf.path, sectorMetaFileName))
	if err != nil {
		return err
	}
	// create the sector file
	sf.sectorData, err = os.Create(filepath.Join(sf.path, sectorDataFileName))
	if err != nil {
		return err
	}

	// TODO: don't directly return the err

	// try to truncate the sector file
	err = sf.sectorData.Truncate(int64(sectors * SectorSize))
	if err != nil {
		return err
	}

	// try to truncate the metafile
	err = sf.sectorMeta.Truncate(int64(sectors * SectorMetaSize))
	if err != nil {
		return err
	}

	// map the folder index to the file folder
	sm.folders[sf.index] = sf

	return nil
}

func (sm *storageManager) isValidNewFolder(sectors uint64, size uint64, path string) error {
	// check if the number of sector is less than maximum limitation
	if sectors > MaxSectorPerFolder {
		return errors.New("folder size is too large")
	}

	// check if the number of sector is more than minimum limitation
	if sectors < MinSectorPerFolder {
		return errors.New("folder size is too small")
	}

	// check if sectors could be map to each granularity exactly
	if sectors%granularity != 0 {
		return errors.New("folder size should be a multiple of 64")
	}

	// if is in the standard mode, no worry is not the absolute path, but
	// we do need the folder be absolute path in standard mode
	if Mode == STD && !filepath.IsAbs(path) {
		return errors.New("given path is not an absolute path")
	}

	// check if the number of folder does not exceed the limitation
	if uint64(len(sm.folders)) >= MaxStorageFolders {
		return errors.New("reach the limitation of maximum number of folder")
	}

	// TODO: what if the folder is in progress but not commit: use wal lock
	// check if the folder is not a duplicate one
	for _, f := range sm.folders {
		if path == f.path {
			return errors.New("folder already exist")
		}
	}

	return nil
}
