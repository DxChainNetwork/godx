package storageclient

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/log"
	"os"
	"path/filepath"
)

var settingsMetadata = common.Metadata{
	Header:  "Storage Client Settings",
	Version: PersistStorageClientVersion,
}

type persistence struct {
	MaxDownloadSpeed int64
	MaxUploadSpeed   int64
	StreamCacheSize  uint64
}

func (sc *StorageClient) loadPersist() error {
	// make directory
	err := os.MkdirAll(sc.staticFilesDir, 0700)
	if err != nil {
		return err
	}

	// initialize logger with multiple handlers (terminal and file)
	sc.log = log.New()
	logFileHandler, err := log.FileHandler(
		filepath.Join(sc.persistDir, PersistLogname),
		log.JSONFormatEx(false, true),
	)

	if err != nil {
		log.Warn("failed to create logger file handler, logging information will not be saved")
	} else {
		logStreamHandler := log.StreamHandler(os.Stdout, log.TerminalFormat(true))
		sc.log.SetHandler(log.MultiHandler(logFileHandler, logStreamHandler))
	}

	// TODO (mzhang): Create Write ahead logger

	// TODO (Jacky): Apply un-applied wal transactions

	// TODO (Jacky): Initialize File Management Related Fields

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
		sc.persist.StreamCacheSize = DefaultStreamCacheSize
		err = sc.saveSettings()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return sc.setBandwidthLimits(sc.persist.MaxUploadSpeed, sc.persist.MaxUploadSpeed)
}
