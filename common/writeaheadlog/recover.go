package writeaheadlog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
)

func (w *Wal) recoverWal(data []byte) ([]*Transaction, error) {
	recoveryState, err := readMetadata(data[:])
	if err != nil {
		return nil, fmt.Errorf("unable to read wal metadata: %v", err)
	}
	// Previous Wal was closed using Close function and no uncommitted transactions
	if recoveryState == stateClean {
		// Now after reopen the wal, the status changed to unclean again
		if err := w.writeRecoveryState(stateUnclean); err != nil {
			return nil, fmt.Errorf("unable to write Wal state: %v", err)
		}
		return nil, nil
	}
	// There are some unfinished transactions logged in the file.
	// define type diskPage as the temporary in-memory page
	type diskPage struct {
		page
		nextPageOffset uint64
	}
	pages := make(map[uint64]*diskPage) // offset - page
	for i := uint64(PageSize); i+PageSize <= uint64(len(data)); i += PageSize {
		nextOffset := binary.LittleEndian.Uint64(data[i:])
		if nextOffset < PageSize {
			// it is a transaction status
			continue
		}
		pages[i] = &diskPage{
			page: page{
				offset:  i,
				payload: data[i+pageMetaSize : i+PageSize],
			},
			nextPageOffset: nextOffset,
		}
	}
	// fill in each nextPage pointer
	for _, p := range pages {
		if nextDiskPage, ok := pages[p.nextPageOffset]; ok {
			p.nextPage = &nextDiskPage.page
		}
	}

	// reconstruct transactions
	var txns []*Transaction
nextTxn:
	for i := PageSize; i+PageSize <= len(data); i += PageSize {
		status := binary.LittleEndian.Uint64(data[i:])
		if status != txnStatusCommitted {
			// Only Transactions after commit is processed
			continue
		}
		// decode metadata and first page
		seq := binary.LittleEndian.Uint64(data[i+8:])
		var diskChecksum [checksumSize]byte
		n := copy(diskChecksum[:], data[i+16:i+16+checksumSize])
		nextPageOffset := binary.LittleEndian.Uint64(data[i+16+n:])
		firstPage := &page{
			offset:  uint64(i),
			payload: data[i+txnMetaSize : i+PageSize],
		}
		if nextDiskPage, ok := pages[nextPageOffset]; ok {
			firstPage.nextPage = &nextDiskPage.page
		}

		// Check if the pages of the transaction form a loop
		visited := make(map[uint64]struct{})
		for page := firstPage; page != nil; page = page.nextPage {
			if _, exists := visited[page.offset]; exists {
				// Loop detected
				continue nextTxn
			}
			visited[page.offset] = struct{}{}
		}
		txn := &Transaction{
			status:         status,
			setupComplete:  true,
			commitComplete: true,
			ID:             seq,
			headPage:       firstPage,
			wal:            w,
		}

		// validate checksum
		if !bytes.Equal(txn.checksum(), diskChecksum[:]) {
			continue
		}

		// decode updates
		var updateBytes []byte
		for page := txn.headPage; page != nil; page = page.nextPage {
			updateBytes = append(updateBytes, page.payload...)
		}
		ops, err := unmarshalOps(updateBytes)
		if err != nil {
			continue
		}
		txn.Operations = ops

		txns = append(txns, txn)
	}

	sort.Slice(txns, func(i, j int) bool {
		return txns[i].ID < txns[j].ID
	})
	// number of pages - meta page
	w.pageCount = uint64(len(data)) / PageSize
	if len(data)%PageSize != 0 {
		w.pageCount++
	}
	if w.pageCount > 0 {
		w.pageCount--
	}

	usedPages := make(map[uint64]struct{})
	for _, txn := range txns {
		for page := txn.headPage; page != nil; page = page.nextPage {
			usedPages[page.offset] = struct{}{}
		}
	}
	for offset := uint64(PageSize); offset < w.pageCount*PageSize; offset += PageSize {
		if _, exists := usedPages[offset]; !exists {
			w.availablePages = append(w.availablePages, offset)
		}
	}
	atomic.StoreInt64(&w.numUnfinishedTxns, int64(len(txns)))

	return txns, nil
}

// readWALMetadata reads WAL metadata from the input file, returning an error
// if the result is unexpected.
func readMetadata(data []byte) (uint8, error) {
	// The metadata should at least long enough to contain all the fields.
	if len(data) < len(metadataHeader)+len(metadataVersion)+2 {
		return 0, errors.New("unable to read wal metadata")
	}

	// Check that the header and version match.
	if !bytes.Equal(data[:len(metadataHeader)], metadataHeader[:]) {
		return 0, fmt.Errorf("invalid file header: %v", data[:len(metadataHeader)])
	}
	if !bytes.Equal(data[len(metadataHeader):len(metadataHeader)+len(metadataVersion)], metadataVersion[:]) {
		return 0, fmt.Errorf("invalid version: %v", data[len(metadataHeader):len(metadataHeader)+len(metadataVersion)])
	}
	// Determine and return the current status of the file.
	fileState := uint8(data[len(metadataHeader)+len(metadataVersion)])
	if fileState <= 0 || fileState > 3 {
		fileState = stateUnclean
	}
	return fileState, nil
}
