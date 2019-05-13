package storagemanager

import (
	"encoding/json"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

func (sm *storageManager) startMaintenance() {
	defer sm.wg.Done()

	for {
		select {
		case <-sm.stopChan:
			if Mode == TST && MockFails["EXIT"] {
				return
			}
			sm.wal.lock.Lock()
			sm.maintenanceClose()
			sm.wal.lock.Unlock()
			return
		case <-time.After(commitFrequency):
			// do commit
			sm.wal.lock.Lock()
			sm.commit()
			sm.wal.lock.Unlock()
		}
	}
}

func (sm *storageManager) maintenanceClose() {
	sm.commit()
	if sm.wal.entries != nil || len(sm.wal.entries) != 0 {
		// it is not a clear shut down
		// TODO: log not clear shut down
		fmt.Println("not clear shutdown")
	} else {
		// TODO: remove the wal file
		err := os.RemoveAll(filepath.Join(sm.persistDir, walFileTmp))
		err = common.ErrCompose(err, os.Remove(filepath.Join(sm.persistDir, walFile)))
		err = common.ErrCompose(err, os.Remove(filepath.Join(sm.persistDir, configFileTmp)))
		fmt.Println("clear shut down")
	}
}

func (sm *storageManager) commit() {
	var wg sync.WaitGroup

	// synchronize all the files
	go sm.syncFiles(&wg)

	// synchronize wal
	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.syncWAL()

		var err error
		sm.wal.walFileTmp, err = os.Create(filepath.Join(sm.persistDir, walFileTmp))
		if err != nil {
			// TODO: log critical
			fmt.Println(err.Error())
		}

		err = sm.wal.writeWALMeta()
		if err != nil {
			// TODO: log critical
			fmt.Println(err.Error())
		}
	}()

	// synchronize the config
	wg.Add(1)
	go func() {
		defer wg.Done()
		newConfig := sm.extractConfig()
		if reflect.DeepEqual(newConfig, sm.wal.loggedConfig) {
			return
		}
		// save the things to json
		err := common.SaveDxJSON(configMetadata,
			filepath.Join(sm.persistDir, configFileTmp), newConfig)
		if err != nil {
			// TODO: log
			fmt.Println(err.Error())
		}
		sm.wal.loggedConfig = newConfig

		// synchronize temporary to persist file
		sm.syncConfig()
		// give a new pointer to new config temp file

		sm.wal.configTmp, err = os.Create(filepath.Join(sm.persistDir, configFileTmp))
		if err != nil {
			// TODO: log critical
			fmt.Println(err.Error())
		}
	}()

	wg.Wait()

	// all synchronize down, commit the changes
	UnprocessedAdditions := findUnprocessedFolderAddition(sm.wal.entries)
	// clear out the entries
	sm.wal.entries = nil

	// if all transactions are done, here only handle the addition
	if UnprocessedAdditions == nil || len(UnprocessedAdditions) == 0 {
		return
	}

	// something is not finished, write to log again
	if err := sm.wal.writeEntry(logEntry{
		PrepareAddStorageFolder: UnprocessedAdditions,
	}); err != nil {
		// TODO: log cannot add change
		fmt.Println(err.Error())
	}
}

func (sm *storageManager) recover(walF *os.File) error {
	var err error
	var entries []logEntry
	var entry logEntry

	// check the metadata of wal file
	decodeWal := json.NewDecoder(walF)
	if checkMeta(decodeWal) != nil {
		// TODO: error, log or crash
		fmt.Println(err.Error())
	}

	// loop through all entries of wal
	for err == nil {
		// decode entry and append to entries
		err = decodeWal.Decode(&entry)
		if err != nil {
			break
		}
		entries = append(entries, entry)
	}

	if err != io.EOF {
		// TODO: may only log
		fmt.Println(err.Error())
	}

	// find the unprocessed addition
	// close and destroy
	unProcessedAddition := findUnprocessedFolderAddition(entries)
	sm.clearUnprocessedAddition(unProcessedAddition)

	// find the processed addition
	processedAddition := findProcessedFolderAddition(entries)
	sm.commitProcessedAddition(processedAddition)

	// save the config to temporary file, then manage to persist the tmp config
	newConfig := sm.extractConfig()
	err = common.SaveDxJSON(configMetadata,
		filepath.Join(sm.persistDir, configFileTmp), newConfig)
	if err != nil {
		// TODO: log
		fmt.Println(err.Error())
	}

	// persist the config
	sm.syncConfig()
	sm.wal.loggedConfig = newConfig


	// all things finished
	// force to commit again
	sm.commit()

	return nil
}

// if the prepare stagge is logged, which means there is no duplication in folders,
// so no worry about delete data already exist
func (sm *storageManager) clearUnprocessedAddition(unProcessedAddition []folderPersist) {
	var err error

	for _, sf := range unProcessedAddition {
		f, exists := sm.folders[sf.Index]

		// if the folder not exist
		if !exists {
			if err := os.RemoveAll(sf.Path); err != nil{
				// TODO: log: consider if this is an dangerous operation or not
			}
			continue
		}
		if f.sectorData != nil {
			if err = f.sectorMeta.Close(); err != nil {
				// TODO: log
			}
			if err = os.RemoveAll(filepath.Join(f.path, sectorMetaFileName)); err != nil {
				// TODO: log
			}

			if err = f.sectorData.Close(); err != nil {
				// TODO: log
			}
			if err = os.RemoveAll(filepath.Join(f.path, sectorDataFileName)); err != nil {
				// TODO: log
			}
		}
		// delete the record from memory
		delete(sm.folders, sf.Index)
	}
}

func (sm *storageManager) commitProcessedAddition(processedAddition []folderPersist) {
	for _, sf := range processedAddition {
		f, exists := sm.folders[sf.Index]
		if exists {
			if f.sectorMeta != nil {
				// availability handle below
				_ = f.sectorMeta.Close()
			}

			if f.sectorData != nil {
				// availability handle below
				_ = f.sectorData.Close()
			}
		}

		f = &storageFolder{
			index: sf.Index,
			path:  sf.Path,
			usage: sf.Usage,
		}

		var err error

		// only the folder could be open can be recorded
		f.sectorMeta, err = os.OpenFile(filepath.Join(sf.Path, sectorMetaFileName), os.O_RDWR, 0700)
		if err != nil {
			// if there is error, not load the things to memory
			continue
		}

		f.sectorData, err = os.OpenFile(filepath.Join(sf.Path, sectorDataFileName), os.O_RDWR, 0700)
		if err != nil {
			continue
		}

		// load the folder to memory
		sm.folders[f.index] = f
	}
}

func checkMeta(decoder *json.Decoder) error {
	var meta common.Metadata
	err := decoder.Decode(&meta)
	if err != nil {
		return err
	} else if meta.Header != walMetadata.Header || meta.Version != walMetadata.Version {
		return errors.New("meta data does not match the expected")
	}

	return nil
}
