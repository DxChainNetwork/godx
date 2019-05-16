// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/log"
	"os"
	"path/filepath"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// settingsMetadata contains the header and version of the JSON file
// used as compatibility purposes
var settingsMetadata = common.Metadata{
	Header:  PersistStorageHostManagerHeader,
	Version: PersistStorageHostManagerVersion,
}

// persistence is a data structure defines the what kind of information
// will be contained in the json file
type persistence struct {
	StorageHostsInfo        []storage.HostInfo
	BlockHeight             uint64
	DisableIPViolationCheck bool
	FilteredHosts           map[enode.ID]struct{}
	FilterMode              FilterMode
}

// saveSettings will save the storage host configurations into the JSON file
func (shm *StorageHostManager) saveSettings() error {
	persist := shm.persistUpdate()
	return common.SaveDxJSON(settingsMetadata, filepath.Join(shm.persistDir, PersistFilename), persist)
}

// persistUpdate contains the information that needs to be written into the
// jso nfile
func (shm *StorageHostManager) persistUpdate() (persist persistence) {
	return persistence{
		StorageHostsInfo:        shm.storageHostTree.All(),
		BlockHeight:             shm.blockHeight,
		DisableIPViolationCheck: shm.disableIPViolationCheck,
		FilteredHosts:           shm.filteredHosts,
		FilterMode:              shm.filterMode,
	}
}

// autoSaveSettings will automatically save the configurations of the storage host manager
// every 2 mins. It will be triggered at the time when the storage host manager got executed
func (shm *StorageHostManager) autoSaveSettings() {
	if err := shm.tm.Add(); err != nil {
		log.Warn("failed to start auto save settings when initializing storage")
		return
	}

	defer shm.tm.Done()

	for {
		select {
		case <-shm.tm.StopChan():
			return
		case <-time.After(saveFrequency):
			shm.lock.Lock()
			err := shm.saveSettings()
			shm.lock.Unlock()
			if err != nil {
				shm.log.Error("failed to save storage host manager settings")
			}
		}
	}
}

// loadSettings will load prior settings from the file
func (shm *StorageHostManager) loadSettings() error {
	// make directory
	err := os.MkdirAll(shm.persistDir, 0700)
	if err != nil {
		return err
	}

	var persist persistence
	persist.FilteredHosts = make(map[enode.ID]struct{})

	err = common.LoadDxJSON(settingsMetadata, filepath.Join(shm.persistDir, PersistFilename), &persist)
	if err != nil {
		return err
	}

	// assign those values to StorageHostManager
	shm.blockHeight = persist.BlockHeight
	shm.disableIPViolationCheck = persist.DisableIPViolationCheck
	shm.filteredHosts = persist.FilteredHosts
	shm.filterMode = persist.FilterMode

	// update the storage host tree
	for _, info := range persist.StorageHostsInfo {

		// insert the storage host
		err := shm.insert(info)
		if err != nil {
			shm.log.Error("could not insert storage host information while loading persistent:", info.IP)
		}

		// start storage host scanning based on the scan records
		if len(info.ScanRecords) < 2 {
			shm.scanValidation(info)
		}
	}

	return nil
}
