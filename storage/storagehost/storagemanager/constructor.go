package storagemanager

import (
	"encoding/binary"
	"encoding/json"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// newStorageManager would help to create a storage manager by given persisDir
func newStorageManager(persistDir string) (*storageManager, error) {
	// create a storage manager object
	sm := &storageManager{
		log:         log.New(),
		persistDir:  persistDir,
		stopChan:    make(chan struct{}, 1),
		folders:     make(map[uint16]*storageFolder),
		sectors:     make(map[sectorID]storageSector),
		sectorLocks: make(map[sectorID]*sectorLock),
		wal:         new(writeAheadLog),
		wg:          new(sync.WaitGroup),
	}

	// if there is any error , close the storage manager
	var err error
	defer func() {
		if err != nil {
			err = common.ErrCompose(err, sm.Close())
		}
	}()

	// construct the body of storage manager
	if err = constructManager(sm); err != nil {
		return nil, err
	}

	// then construct the writeAheadLog
	if err = constructWriteAheadLog(sm); err != nil {
		return nil, err
	}

	return sm, nil
}

// constructManager construct the body of storage manager, according to the
// config file, each sector and folders object would be create
func constructManager(sm *storageManager) error {
	var err error

	// make up dir for storing
	if err := os.MkdirAll(sm.persistDir, 0700); err != nil {
		return err
	}

	// loads the config from the config log
	config := &configPersist{}
	err = common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), config)
	// if cannot find any config file
	if os.IsNotExist(err) {
		// use the default setting for constructing the storage manager
		return constructManagerDefault(sm)
	} else if err != nil {
		// if the error is not caused by the FILE NOT FOUND
		return err
	}

	// loads the sector salt and construct the folders object
	sm.sectorSalt = config.SectorSalt
	constructFolders(sm, config.Folders)
	return nil
}

// constructFolders construct the folder object for storage manager
func constructFolders(sm *storageManager, folders []folderPersist) {
	var err error

	// create folders for from the persisted info
	for _, folder := range folders {
		sf := &storageFolder{
			index: folder.Index,
			path:  folder.Path,
			usage: folder.Usage,

			freeSectors: make(map[sectorID]uint32),
		}

		// try to open the sector metadata file
		sf.sectorMeta, err = os.OpenFile(filepath.Join(sf.path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&sf.atomicUnavailable, 1)
		}

		// try to open the sector data file
		sf.sectorData, err = os.OpenFile(filepath.Join(sf.path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&sf.atomicUnavailable, 1)
		}

		// map the folder object using the index
		sm.folders[sf.index] = sf

		constructSector(sm, sf)
	}
}

func constructSector(sm *storageManager, sf *storageFolder) {
	// read the metadata from the sector

	// compute the number of sector in the folder, times the metadata size per
	// sector is the total number of length needed by the buffer
	data := make([]byte, len(sf.usage)*granularity*int(SectorMetaSize))
	// read the data from beginning
	_, err := sf.sectorMeta.ReadAt(data, 0)
	if err != nil {
		// if there is error reading the metadata of the folder
		atomic.StoreUint64(&sf.atomicUnavailable, 1)
		// TODO: log the issue
	}

	sf.sectors = 0

	for usageIndex := 0; usageIndex < len(sf.usage); usageIndex++ {
		// no data in this usage group
		if sf.usage[usageIndex] == 0 {
			continue
		}

		// if there is data, extract and create the sector object
		for bitIndex := 0; bitIndex < granularity; bitIndex++ {
			// if the secotor is not free, which means there is data
			if !sf.usage[usageIndex].isFree(uint16(bitIndex)) {
				sectorIndex := usageIndex*granularity + bitIndex
				start := int(SectorMetaSize) * sectorIndex

				var id sectorID
				copy(id[:], data[start:start+12])

				count := binary.LittleEndian.Uint16(data[start+12 : start+14])

				ss := storageSector{
					index:         uint32(sectorIndex),
					storageFolder: sf.index,
					count:         count,
				}

				sm.sectors[id] = ss
				sf.sectors++
			}
		}
	}
}

func constructWriteAheadLog(sm *storageManager) error {
	// 1. if there not exist wal file and no wal file tmp: good
	// 2. if only wal file, recover from wal
	// 3. if only wal tmp, recover from wal tmp --> rename to wal
	// 4. if both wal, wal tmp, recover both --> copy entries to wal

	walName := filepath.Join(sm.persistDir, walFile)
	walTmpName := filepath.Join(sm.persistDir, walFileTmp)
	confTmpName := filepath.Join(sm.persistDir, configFileTmp)

	var walErr, walTmpErr, confErr, err error
	var existWal = false
	var existWalTmp = false

	sm.wal.entries = make([]logEntry, 0)
	// create new temporary config file
	sm.wal.configTmp, confErr = os.Create(confTmpName)
	if confErr != nil {
		// TODO: log or crash
		return confErr

	}

	// try to open wal file
	walF, walErr := os.OpenFile(walName, os.O_RDWR, 0700)
	if walErr == nil {
		existWal = true
	} else if walErr != nil && !os.IsNotExist(walErr) {
		// TODO: log or crash
		return walErr
	}

	// try to open wal temp file
	sm.wal.walFileTmp, walTmpErr = os.OpenFile(walTmpName, os.O_RDWR, 0700)
	if walTmpErr == nil {
		existWalTmp = true
	} else if walTmpErr != nil && !os.IsNotExist(walTmpErr) {
		// TODO: crash or log
		return walTmpErr
	}

	// if not exist wal and tmp, clear shut down last time
	if !existWalTmp && !existWal {
		// create tmp file, then ready for recovering
		sm.wal.walFileTmp, err = os.Create(walTmpName)
		// write metadata
		err = common.ErrCompose(err, sm.wal.writeWALMeta())
		return err
	}

	// if not exist wal tmp
	if !existWalTmp {

	} else if !existWal {
		// rename wal tmp to wal
		if err := os.Rename(walTmpName, walName); err != nil {
			// TODO: log or crash
			return err
		}
	} else {
		// both exist, merge
		if err = mergeWal(walF, sm.wal.walFileTmp); err != nil {
			// TODO: log or crash
			return err
		}

		if err != nil {
			// TODO: log or crash
			return err
		}
	}

	// create tmp file, then ready for recovering
	sm.wal.walFileTmp, err = os.Create(walTmpName)
	// write metadata
	err = common.ErrCompose(err, sm.wal.writeWALMeta())
	if err != nil {
		// TODO: log or crash
		return err
	}

	// reopen the walFile
	_ = walF.Close()
	walF, err = os.Open(walName)
	// this time should be ok
	if err != nil {
		return err
	}
	// start recover
	return sm.recover(walF)

}

// construct the default storage manager setting
func constructManagerDefault(sm *storageManager) error {
	var err error

	// random a seeds for rand, and generate a sector salt
	rand.Seed(time.Now().UTC().UnixNano())
	if _, err = rand.Read(sm.sectorSalt[:]); err != nil {
		return err
	}

	// manage to synchronize the default config to config file
	if err := sm.syncConfigForce(); err != nil {
		return err
	}

	return nil
}

// make sure the wal and walTmp are already created
func mergeWal(wal *os.File, walTmp *os.File) error {
	var err error
	// check the metadata for
	decodeWalTmp := json.NewDecoder(walTmp)
	if err = checkMeta(decodeWalTmp); err != nil {
		// TODO: log or crash
		return err
	}

	decodeWal := json.NewDecoder(wal)
	if err = checkMeta(decodeWal); err != nil {
		// TODO: log or crash
		return err
	}

	// if metadata checking is fine
	// start to merge
	var entry logEntry

	for err == nil {
		err = decodeWalTmp.Decode(&entry)
		if err != nil {
			break
		}

		changeBytes, err := json.MarshalIndent(entry, "", "\t")
		if err != nil {
			return err
		}

		n, err := wal.Seek(0, 2)
		if err != nil {

		}

		// write the things decode from tmp file to wal file
		_, err = wal.WriteAt(changeBytes, n)

		if err != nil {
			return err
		}
	}

	if err != io.EOF {
		return err
	}

	return nil
}
