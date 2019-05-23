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

// A download is a file download that has been queued by the renter.
type (
	download struct {
		// Data progress variables.
		atomicDataReceived         uint64 // Incremented as data completes, will stop at 100% file progress.
		atomicTotalDataTransferred uint64 // Incremented as data arrives, includes overdrive, contract negotiation, etc.

		// Other progress variables.
		segmentsRemaining uint64        // Number of segments whose downloads are incomplete.
		completeChan      chan struct{} // Closed once the download is complete.
		err               error         // Only set if there was an error which prevented the download from completing.

		// downloadCompleteFunc is a slice of functions which are called when
		// completeChan is closed.
		downloadCompleteFuncs []downloadCompleteFunc

		// Timestamp information.
		endTime         time.Time // Set immediately before closing 'completeChan'.
		staticStartTime time.Time // Set immediately when the download object is created.

		// Basic information about the file.
		destination           downloadDestination
		destinationString     string // The string reported to the user to indicate the download's destination.
		staticDestinationType string // "memory buffer", "http stream", "file", etc.
		staticLength          uint64 // Length to download starting from the offset.
		staticOffset          uint64 // Offset within the file to start the download.
		staticDxFilePath      string // The path of the dxfile at the time the download started.

		// Retrieval settings for the file.
		staticLatencyTarget time.Duration // In milliseconds. Lower latency results in lower total system throughput.
		staticOverdrive     int           // How many extra pieces to download to prevent slow hosts from being a bottleneck.
		staticPriority      uint64        // Downloads with higher priority will complete first.

		// Utilities.
		log           *log.Logger                  // Same log as the renter.
		memoryManager *memorymanager.MemoryManager // Same memoryManager used across the renter.
		mu            sync.Mutex                   // Unique to the download object.
	}

	// downloadParams is the set of parameters to use when downloading a file.
	downloadParams struct {
		destination       downloadDestination // The place to write the downloaded data.
		destinationType   string              // "file", "buffer", "http stream", etc.
		destinationString string              // The string to report to the user for the destination.
		file              *dxfile.Snapshot    // The file to download.

		latencyTarget time.Duration // Workers above this latency will be automatically put on standby initially.
		length        uint64        // Length of download. Cannot be 0.
		needsMemory   bool          // Whether new memory needs to be allocated to perform the download.
		offset        uint64        // Offset within the file to start the download. Must be less than the total filesize.
		overdrive     int           // How many extra pieces to download to prevent slow hosts from being a bottleneck.
		priority      uint64        // Files with a higher priority will be downloaded first.
	}

	// downloadCompleteFunc is a function called upon completion of the
	// download. It accepts an error as an argument and returns an error. That
	// way it's possible to add custom behavior for failing downloads.
	downloadCompleteFunc func(error) error
)
