package storagemanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

func (sm *storageManager) startMaintenance() {
	sm.wg.Add(1)
	defer sm.wg.Done()

	for {
		select {
		case <-sm.stopChan:
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
		}
		sm.wal.loggedConfig = newConfig

		// synchronize temporary to persist file
		sm.syncConfig()
		// give a new pointer to new config temp file

		sm.wal.configTmp, err = os.Create(filepath.Join(sm.persistDir, configFileTmp))
		if err != nil {
			// TODO: log critical
		}

		err = sm.wal.writeWALMeta()
		if err != nil {
			// TODO: log critical
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
	}
}
