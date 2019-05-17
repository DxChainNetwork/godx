package storagemanager

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

// thread for start the maintenance, for a specific
// frequency, the thread commit the things already done
func (sm *storageManager) startMaintenance() {
	defer sm.wg.Done()

	for {
		select {
		case <-sm.stopChan:
			// mock the fail or unclear shut down
			// directly return without last commit
			if Mode == TST && MockFails["EXIT"] {
				return
			}
			sm.maintenanceClose()
			return
		case <-time.After(commitFrequency):
			// do commit
			sm.wal.lock.Lock()
			sm.commit()
			sm.wal.lock.Unlock()
		}
	}
}

// close the maintenance, start the last commit,
// and close files
func (sm *storageManager) maintenanceClose() {
	sm.commit()
	if sm.wal.entries != nil || len(sm.wal.entries) != 0 {
		// it is not a clear shut down
		// TODO: log not clear shut down
		_ = sm.wal.walFileTmp.Close()
		_ = sm.wal.configTmp.Close()
		fmt.Println("not clear shutdown")
	} else {
		// TODO: remove the wal file
		// TODO: close all files resources
		err := os.RemoveAll(filepath.Join(sm.persistDir, walFileTmp))
		err = common.ErrCompose(err, os.Remove(filepath.Join(sm.persistDir, walFile)))
		err = common.ErrCompose(err, os.Remove(filepath.Join(sm.persistDir, configFileTmp)))
		fmt.Println("clear shut down")
	}
}

// thread for commit the changes:
// find out all the unprocessed files and
func (sm *storageManager) commit() {
	var wg sync.WaitGroup
	wg.Add(1)
	// synchronize all the files
	go sm.syncFiles(&wg)

	// synchronize wal
	wg.Add(1)
	go func() {
		defer wg.Done()
		// change the walTmp to wal
		sm.syncWAL()

		var err error
		// create a new tmp file, and write meta data to it
		sm.wal.walFileTmp, err = os.Create(filepath.Join(sm.persistDir, walFileTmp))
		if err != nil {
			// TODO: log critical
			fmt.Println(err.Error(), walFileTmp)
		}
		// write meta data to the newly created wal tmp
		if err = sm.wal.writeWALMeta(); err != nil {
			// TODO: log critical
			fmt.Println(err.Error(), "write wal meta")
		}
	}()

	// synchronize the config
	wg.Add(1)
	go func() {
		defer wg.Done()
		// extract the current config
		newConfig := extractConfig(*sm)
		// if the current config is the same as config logged last time, no need to do sync again
		if reflect.DeepEqual(newConfig, sm.wal.loggedConfig) {
			return
		}
		// save the things to json
		err := common.SaveDxJSON(configMetadata,
			filepath.Join(sm.persistDir, configFileTmp), newConfig)

		if err != nil {
			// TODO: log
			fmt.Println(err.Error(), configFileTmp)
		}

		// mark the new config as logged
		sm.wal.loggedConfig = newConfig

		// synchronize temporary to persist file
		sm.syncConfig()

		// close the config
		if err = sm.wal.configTmp.Close(); err != nil {
			// TODO: log or crash
			fmt.Println(err.Error())
		}

		// give a new pointer to new config temp file
		sm.wal.configTmp, err = os.Create(filepath.Join(sm.persistDir, configFileTmp))
		if err != nil {
			// TODO: log critical
			fmt.Println(err.Error(), "create", configFileTmp)
		}
	}()

	wg.Wait()

	// all synchronize down, find out the unprocessed change,
	// record those changes to wal tmp again
	UnprocessedAdditions := findUnprocessedFolderAddition(sm.wal.entries)

	// clear out the entries
	sm.wal.entries = nil

	// if all transactions are done, here only handle the addition
	if UnprocessedAdditions == nil || len(UnprocessedAdditions) == 0 {
		return
	}

	// something is not finished, write to log again
	if err := sm.wal.writeEntry(
		logEntry{PrepareAddStorageFolder: UnprocessedAdditions},
	); err != nil {
		// TODO: log or crash
		fmt.Println(err.Error())
	}
}

// recover read through the entries in wal, find out the
// unprocessed operations, revert, and commit
func (sm *storageManager) recover(walF *os.File) error {
	var err error
	var entries []logEntry
	var entry logEntry

	// check the metadata of wal file
	decodeWal := json.NewDecoder(walF)
	if err = checkMeta(decodeWal); err != nil {
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

	// find the processed sector update and get recover
	findProcessedSectorUpdate := findProcessedSectorUpdate(entries)
	sm.recoverSector(findProcessedSectorUpdate)

	// turn the pointer of wal tmp to this file, commit would manage
	// all the process
	if err := walF.Close(); err != nil {
		// TODO: log or crash
	}
	if err := os.Rename(filepath.Join(sm.persistDir, walFile), filepath.Join(sm.persistDir, walFileTmp)); err != nil {
		// TODO: log or crash
	}

	if sm.wal.walFileTmp, err = os.OpenFile(filepath.Join(sm.persistDir, walFileTmp), os.O_RDWR, 0700); err != nil {
		// TODO: log or crash
	}

	// all things finished force to commit again
	sm.commit()
	return nil
}
