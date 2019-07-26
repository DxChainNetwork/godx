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

func (client *StorageClient) loadPersist() error {
	// make directory
	err := os.MkdirAll(client.staticFilesDir, 0700)
	if err != nil {
		return err
	}

	// initialize logger
	client.log = log.New()

	return client.loadSettings()
}

// save StorageClient settings into storageclient.json file
func (client *StorageClient) saveSettings() error {
	return common.SaveDxJSON(settingsMetadata, filepath.Join(client.persistDir, PersistFilename), client.persist)
}

// load prior StorageClient settings
func (client *StorageClient) loadSettings() error {
	client.persist = persistence{}
	err := common.LoadDxJSON(settingsMetadata, filepath.Join(client.persistDir, PersistFilename), &client.persist)
	if os.IsNotExist(err) {
		client.persist.MaxDownloadSpeed = DefaultMaxDownloadSpeed
		client.persist.MaxUploadSpeed = DefaultMaxUploadSpeed
		err = client.saveSettings()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return client.setBandwidthLimits(client.persist.MaxUploadSpeed, client.persist.MaxUploadSpeed)
}
