package storagemanager

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/DxChainNetwork/godx/common"
)

// writeAheadLog manage log the operations
type writeAheadLog struct {
	// wal record the prepare and finish of operations
	walFileTmp *os.File

	// config save the configuration of data
	configTmp *os.File

	// entries use to log each entries
	entries []logEntry

	// logged Config represent the last config that logged in file
	loggedConfig *configPersist

	lock sync.Mutex
}

// entry of wal, storing each operations
// and corresponding information
type logEntry struct {
	PrepareAddStorageFolder   []folderPersist
	ProcessedAddStorageFolder []folderPersist
	RevertAddStorageFolder    []folderPersist
	CancelAddStorageFolder    []folderPersist

	SectorUpdate []sectorPersist
}

// writeWALMeta write the metadata of walFileTmp
// @ Require: this function need to be called the before writing any entry
// or it would corrupt the walFileTmp
func (wal *writeAheadLog) writeWALMeta() error {
	// marshal the metadata to json format
	changeBytes, err := json.MarshalIndent(walMetadata, "", "\t")
	if err != nil {
		return err
	}
	// simply write at the beginning
	_, err = wal.walFileTmp.Write(changeBytes)
	return err
}

// write entry write the entry to files
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

// checkMeta check the wal metadata by given decoder
func checkMeta(decoder *json.Decoder) error {
	var meta common.Metadata
	// try to load things to metadata
	err := decoder.Decode(&meta)
	if err != nil {
		return err
	} else if meta.Header != walMetadata.Header || meta.Version != walMetadata.Version {
		return errors.New("meta data does not match the expected")
	}

	return nil
}
