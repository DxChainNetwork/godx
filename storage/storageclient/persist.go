// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"os"
	"path/filepath"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
)

var settingsMetadata = common.Metadata{
	Header:  "storage client Settings",
	Version: PersistStorageClientVersion,
}

type persistence struct {
	MaxDownloadSpeed int64
	MaxUploadSpeed   int64
}

func (sc *StorageClient) loadPersist() error {
	// make directory
	err := os.MkdirAll(sc.staticFilesDir, 0700)
	if err != nil {
		return err
	}

	// initialize logger
	sc.log = log.New()

	return sc.loadSettings()
}

// save StorageClient settings into storageclient.json file
func (sc *StorageClient) saveSettings() error {
	return common.SaveDxJSON(settingsMetadata, filepath.Join(sc.persistDir, PersistFilename), sc.persist)
}

// load prior StorageClient settings
func (sc *StorageClient) loadSettings() error {
	sc.persist = persistence{}
	err := common.LoadDxJSON(settingsMetadata, filepath.Join(sc.persistDir, PersistFilename), &sc.persist)
	if os.IsNotExist(err) {
		sc.persist.MaxDownloadSpeed = DefaultMaxDownloadSpeed
		sc.persist.MaxUploadSpeed = DefaultMaxUploadSpeed
		err = sc.saveSettings()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return sc.setBandwidthLimits(sc.persist.MaxUploadSpeed, sc.persist.MaxUploadSpeed)
}
