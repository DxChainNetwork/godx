package storagemanager

import (
	"encoding/json"
	"os"
	"sync"
)

type writeAheadLog struct {
	walFileTmp *os.File

	entries []logEntry

	lock sync.Mutex
}

type logEntry struct {
	PrepareAddStorageFolder   []folderPersist
	ProcessedAddStorageFolder []folderPersist
	RevertAddStorageFolder    []folderPersist
}

// writeEntry write the wal entry
// Require: lock of wal should be handle by the caller
// Effect: append new log entry to the wal
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
