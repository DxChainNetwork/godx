// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
	"github.com/DxChainNetwork/godx/storage/storageclient/memorymanager"
)

type (

	// a file download that has been queued by the client.
	download struct {

		// incremented as data completes, will stop at 100% file progress.
		dataReceived uint64

		// incremented as data arrives, include everything from connection.
		totalDataTransferred uint64

		// the number of incomplete segments for this download
		segmentsRemaining uint64
		completeChan      chan struct{}
		err               error

		// a slice of functions which are called when completeChan is closed.
		downloadCompleteFuncs []downloadCompleteFunc

		// download completed time
		endTime time.Time

		// download created time
		startTime time.Time

		// where to write the downloaded data
		destination writeDestination

		// the destination need to report to user
		destinationString string

		// how to write the downloaded data,
		// like that "file", "buffer", "http stream" ...
		destinationType string

		// the length of data to download
		length uint64

		// the start index in file to download.
		offset uint64

		// the dx file for downloading
		dxFile *dxfile.Snapshot

		// In milliseconds.
		latencyTarget time.Duration

		// the number of extra sectors to download,
		// this can detect "Low performance host" for client.
		overdrive int

		// higher priority will complete first.
		priority uint64

		// Utilities.
		log           log.Logger
		memoryManager *memorymanager.MemoryManager
		mu            sync.Mutex
	}

	// parameters to use when downloading a file.
	downloadParams struct {

		// where to write the downloaded data
		destination writeDestination

		// how to write the downloaded data,
		// like that "file", "buffer", "http stream" ...
		destinationType string

		// the destination need to report to user
		destinationString string

		// the file to download
		file *dxfile.Snapshot

		// worker with higher latency will be put standby
		latencyTarget time.Duration

		// the length of data to download
		length uint64

		// whether need to allocate memory for this download
		needsMemory bool

		// the start index in file to download.
		offset uint64

		// the number of extra sectors to download,
		// this can detect "Low performance host" for client.
		overdrive int

		// higher priority download first
		priority uint64
	}

	// a function type that is called when the download completed.
	downloadCompleteFunc func(error) error
)

// fail will mark the download as complete, but with the provided error.
func (d *download) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If the download is already complete, extend the error.
	complete := d.isComplete()
	if complete && d.err != nil {
		return
	} else if complete && d.err == nil {
		d.log.Crit("download is marked as completed without error, but the func of fail was called with err", "error", err)
		return
	}

	// Mark the download as complete and set the error.
	d.err = err
	d.markComplete()
}

// return whether or not the download has completed.
func (d *download) isComplete() bool {
	select {
	case <-d.completeChan:
		return true
	default:
		return false
	}
}

// mark the download complete and executes the downloadCompleteFuncs.
func (d *download) markComplete() {
	if d.isComplete() {
		d.log.Warn("Can't call markComplete multiple times")
	} else {
		defer close(d.completeChan)
	}

	var errs []error
	for _, f := range d.downloadCompleteFuncs {
		err := f(d.err)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		d.log.Error("Failed to execute at least one downloadCompleteFunc", "error", errs)
	}

	// set downloadCompleteFuncs to nil to avoid executing them multiple times.
	d.downloadCompleteFuncs = nil
}

// returns the error encountered by a download
func (d *download) Err() (err error) {
	d.mu.Lock()
	err = d.err
	d.mu.Unlock()
	return err
}

// registers a function to be called when the download is completed
func (d *download) onComplete(f downloadCompleteFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	select {
	case <-d.completeChan:
		err := f(d.err)
		d.log.Error("Failed to execute downloadCompleteFunc", "error", err)
	default:
	}
	d.downloadCompleteFuncs = append(d.downloadCompleteFuncs, f)
}
