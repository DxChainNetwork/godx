package storagemanager

import (
	"encoding/json"
	"os"
	"sync"
)

type writeAheadLog struct {
	walFileTmp *os.File
	configTmp  *os.File

	entries      []logEntry
	loggedConfig *configPersist

	lock sync.Mutex
}

type logEntry struct {
	PrepareAddStorageFolder   []folderPersist
	ProcessedAddStorageFolder []folderPersist
	RevertAddStorageFolder    []folderPersist
	CancelAddStorageFolder    []folderPersist

	PrepareAddStorageSector []sectorPersist
}

func (wal *writeAheadLog) writeEntry(entries ...logEntry) error {
	for _, entry := range entries {
		changeBytes, err := json.MarshalIndent(entry, "", "\t")
		if err != nil {
			return err
		}
		_, err = wal.walFileTmp.Write(changeBytes)
		if err != nil {
			return err
		}
		wal.entries = append(wal.entries, entry)
	}
	return nil
}

func (wal *writeAheadLog) writeWALMeta() error {
	changeBytes, err := json.MarshalIndent(walMetadata, "", "\t")
	if err != nil {
		return err
	}
	_, err = wal.walFileTmp.Write(changeBytes)
	if err != nil {
		return err
	}
	return nil
}
