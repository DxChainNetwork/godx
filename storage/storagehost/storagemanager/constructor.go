package storagemanager

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
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
	atomic.StoreUint64(&sm.atomicSwitch, 0)

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

	sm.wg.Add(1)
	go sm.startMaintenance()

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
	// using the config, construct each folder object
	constructFolders(sm, config.Folders)
	return nil
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
	config := extractConfig(*sm)
	if err := common.SaveDxJSON(configMetadata,
		filepath.Join(sm.persistDir, configFile), config); err != nil {
		return err
	}

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
		atomic.StoreUint64(&sf.atomicUnavailable, 0)

		// try to open the sector metadata file
		sf.sectorMeta, err = os.OpenFile(filepath.Join(sf.path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			// TODO: log the issue
			atomic.StoreUint64(&sf.atomicUnavailable, 1)
		}

		// try to open the sector data file
		sf.sectorData, err = os.OpenFile(filepath.Join(sf.path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			// TODO: log the issue
			atomic.StoreUint64(&sf.atomicUnavailable, 1)
		}

		// map the folder object using the index
		sm.folders[sf.index] = sf
		// construct the sector
		constructSector(sm, sf)
	}
}

// constructSector construct the sector by reading the
// metadata of the given folder
func constructSector(sm *storageManager, sf *storageFolder) {
	// read the metadata from the sector

	// compute data length, which is the number of sector in the folder, multiply the metadata size per
	// sector is the total number of length needed by the buffer
	data := make([]byte, len(sf.usage)*granularity*int(SectorMetaSize))
	// read the data from beginning
	_, err := sf.sectorMeta.ReadAt(data, 0)
	if err != nil {
		// TODO: log the issue
		// if there is error reading the metadata of the folder
		atomic.StoreUint64(&sf.atomicUnavailable, 1)
	}

	sf.sectors = 0

	// loop through each sector by usage index
	for usageIndex := 0; usageIndex < len(sf.usage); usageIndex++ {
		// no data in this usage group
		if sf.usage[usageIndex] == 0 {
			continue
		}

		// if there is data, extract and create the sector object
		for bitIndex := 0; bitIndex < granularity; bitIndex++ {
			// if the sector is free, which means there is no data
			if sf.usage[usageIndex].isFree(uint16(bitIndex)) {
				continue
			}

			// compute the sector index
			sectorIndex := usageIndex*granularity + bitIndex
			// start is the start point to read the metadata
			start := int(SectorMetaSize) * sectorIndex

			var id sectorID
			// get the 12 byte id
			copy(id[:], data[start:start+12])
			// get the number of virtual sector
			count := binary.LittleEndian.Uint16(data[start+12 : start+14])

			ss := storageSector{
				index:         uint32(sectorIndex),
				storageFolder: sf.index,
				count:         count,
			}
			// map the sector on storage manager
			sm.sectors[id] = ss
			sf.sectors++
		}
	}
}

// constructWriteAheadLog construct write ahead log, check
// the existence of the wal, if wal exist, which means not clear
// shut down last time, other wise, clear shut down, simply creat the
// new wal
func constructWriteAheadLog(sm *storageManager) error {
	// 1. if there not exist wal file and no wal file tmp: good
	// 2. if only wal file, recover from wal
	// 3. if only wal tmp, recover from wal tmp: rename tmp to wal and recover
	// 4. if both wal, wal tmp, recover both: copy entries to wal and recover

	walName := filepath.Join(sm.persistDir, walFile)
	walTmpName := filepath.Join(sm.persistDir, walFileTmp)
	confTmpName := filepath.Join(sm.persistDir, configFileTmp)

	var walErr, walTmpErr, confErr, err error
	var existWal = false
	var existWalTmp = false

	// make the log entry for wal
	sm.wal.entries = make([]logEntry, 0)
	// create new temporary config file, note: if exist confTmp
	// TODO: check if need conf tmp or not
	sm.wal.configTmp, confErr = os.Create(confTmpName)
	if confErr != nil {
		// TODO: log or crash
		return confErr
	}

	// try to open wal file
	walF, walErr := os.OpenFile(walName, os.O_RDWR, 0700)
	if walErr == nil {
		// if there is no error, which means wal is cleared
		existWal = true
	} else if walErr != nil && !os.IsNotExist(walErr) {
		// the error is caused by something else, continue
		// do operation may override the files and lead corruption
		// of data file, my consider crash
		// TODO: log or crash
		return walErr
	}

	// try to open wal temp file
	sm.wal.walFileTmp, walTmpErr = os.OpenFile(walTmpName, os.O_RDWR, 0700)
	if walTmpErr == nil {
		// if there is no wal tmp error, which means wal tmp is cleared
		existWalTmp = true
	} else if walTmpErr != nil && !os.IsNotExist(walTmpErr) {
		// the error is caused by something else, continue
		// do operation may override the files and lead corruption
		// of data file, my consider crash
		// TODO: crash or log
		return walTmpErr
	}

	// if not exist wal and tmp, clear shut down last time
	// create a temporary wal to use
	if !existWalTmp && !existWal {
		// create tmp file, then ready for recovering
		sm.wal.walFileTmp, err = os.Create(walTmpName)
		// write metadata
		err = common.ErrCompose(err, sm.wal.writeWALMeta())
		return err
	}

	// if only wal tmp, recover from wal tmp, rename to wal
	if !existWal && existWalTmp {
		// close the wal Tmp, directly rename to wal
		if err := sm.wal.walFileTmp.Close(); err != nil {
			// TODO: log or crash
			return err
		}

		// rename wal tmp to wal
		if err := os.Rename(walTmpName, walName); err != nil {
			// TODO: log or crash
			return err
		}

		sm.wal.walFileTmp = nil
	}

	//if both wal, wal tmp, merge entries to wal and start recover
	if existWal && existWalTmp {
		// both exist, merge to wal
		if err = mergeWal(walF, sm.wal.walFileTmp); err != nil {
			// TODO: log or crash
			return err
		}
		// all entries are recorded to wal
		// create tmp file, then ready for recovering
		if err := sm.wal.walFileTmp.Close(); err != nil {
			// TODO: log or crash
			return err
		}

		// refresh reopen the walFile
		if err = walF.Close(); err != nil {
			return err
		}

		sm.wal.walFileTmp = nil
	}

	// redirect the pointer
	if walF, err = os.OpenFile(walName, os.O_RDWR, 0700); err != nil {
		// TODO: log or crash
		return err
	}

	return sm.recover(walF)
}

// mergeWal to make sure the wal and walTmp are already created,
// and merge all the entries in wal tmp to wal
// @ Require: wal and walTmp only open and do no operation to file
// @ effect: the files remain unclosed, write pointer cannot predicted
func mergeWal(wal *os.File, walTmp *os.File) error {
	var err error
	// check the metadata for temporary wal
	decodeWalTmp := json.NewDecoder(walTmp)
	if err = checkMeta(decodeWalTmp); err != nil {
		return err
	}
	// check the metadata for wal
	decodeWal := json.NewDecoder(wal)
	if err = checkMeta(decodeWal); err != nil {
		return err
	}

	for err == nil {
		// if metadata checking is fine
		// start to merge
		var entry logEntry

		// continue decoding on the tmp file is fine
		// the error might be caused by reaching to the end
		if err = decodeWalTmp.Decode(&entry); err != nil {
			break
		}

		// marshal the decoded entry to json format
		changeBytes, err := json.MarshalIndent(entry, "", "\t")
		if err != nil {
			return err
		}

		// wal seek the last ending position, and insert
		n, err := wal.Seek(0, 2)
		if err != nil {
			return err
		}

		// write the things decode from tmp file to wal file
		if _, err = wal.WriteAt(changeBytes, n); err != nil {
			return err
		}
	}

	// check if the error is caused by the EOF, if not, return
	if err != io.EOF {
		return err
	}

	return nil
}
