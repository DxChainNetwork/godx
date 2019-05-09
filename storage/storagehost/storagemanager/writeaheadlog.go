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
}

func (wal *writeAheadLog) Close() {

}

func (wal *writeAheadLog) writeEntry(entries ...logEntry) error {
	changeBytes, err := json.MarshalIndent(entries, "", "\t")
	if err != nil {
		return err
	}

	_, err = wal.walFileTmp.Write(changeBytes)

	if err != nil {
		return err
	}

	wal.entries = append(wal.entries, entries...)

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
