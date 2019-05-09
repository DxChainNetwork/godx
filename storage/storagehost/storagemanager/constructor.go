package storagemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// newStorageManager would help to create a storage manager by given persisDir
func newStorageManager(persistDir string) (*storageManager, error) {
	// create a storage manager object
	sm := &storageManager{
		log:        log.New(),
		persistDir: persistDir,
		stopChan:   make(chan struct{}),
		folders:    make(map[uint16]*storageFolder),
		wal:        new(writeAheadLog),
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
	if err = constructWriteAheadLog(sm.wal, sm.persistDir); err != nil {
		return nil, err
	}

	return sm, nil
}

// construct the writeAheadLog
// TODO: recover process,
func constructWriteAheadLog(wal *writeAheadLog, persisDir string) error {
	wal.entries = make([]logEntry, 0)
	var walErr, confErr error
	wal.walFileTmp, walErr = os.Create(filepath.Join(persisDir, walFileTmp))
	wal.configTmp, confErr = os.Create(filepath.Join(persisDir, configFileTmp))
	return common.ErrCompose(walErr, confErr)
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
		f := &storageFolder{
			index: folder.Index,
			path:  folder.Path,
			usage: folder.Usage,
		}

		// try to open the sector metadata file
		f.sectorMeta, err = os.OpenFile(filepath.Join(f.path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&f.atomicUnavailable, 1)
		}

		// try to open the sector data file
		f.sectorData, err = os.OpenFile(filepath.Join(f.path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			atomic.StoreUint64(&f.atomicUnavailable, 1)
		}

		// map the folder object using the index
		sm.folders[f.index] = f
	}
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
