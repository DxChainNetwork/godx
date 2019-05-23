// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"github.com/DxChainNetwork/godx/storage/storageclient/filesystem/dxfile"
)

// downloadPieceInfo contains all the information required to download and
// recover a piece of a segment from a host. It is a value in a map where the key
// is the file contract id.
type downloadPieceInfo struct {
	index uint64
	root  common.Hash
}

// unfinishedDownloadSegment contains a segment for a download that is in progress.
type unfinishedDownloadSegment struct {
	// Fetch + Write instructions - read only or otherwise thread safe.
	destination downloadDestination // Where to write the recovered logical segment.
	erasureCode erasurecode.ErasureCoder
	masterKey   crypto.CipherKey

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticSegmentIndex uint64                       // Required for deriving the encryption keys for each piece.
	staticCacheID      string                       // Used to uniquely identify a segment in the segment cache.
	staticSegmentMap   map[string]downloadPieceInfo // Maps from host PubKey to the info for the piece associated with that host
	staticSegmentSize  uint64
	staticFetchLength  uint64 // Length within the logical segment to fetch.
	staticFetchOffset  uint64 // Offset within the logical segment that is being downloaded.
	staticPieceSize    uint64
	staticWriteOffset  int64 // Offset within the writer to write the completed data.

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticLatencyTarget time.Duration
	staticNeedsMemory   bool // Set to true if memory was not pre-allocated for this segment.
	staticOverdrive     int
	staticPriority      uint64

	// Download segment state - need mutex to access.
	completedPieces     []bool    // Which pieces were downloaded successfully.
	failed              bool      // Indicates if the segment has been marked as failed.
	physicalSegmentData [][]byte  // Used to recover the logical data.
	pieceUsage          []bool    // Which pieces are being actively fetched.
	piecesCompleted     int       // Number of pieces that have successfully completed.
	piecesRegistered    int       // Number of pieces that workers are actively fetching.
	recoveryComplete    bool      // Whether or not the recovery has completed and the segment memory released.
	workersRemaining    int       // Number of workers still able to fetch the segment.
	workersStandby      []*worker // Set of workers that are able to work on this download, but are not needed unless other workers fail.

	// Memory management variables.
	memoryAllocated uint64

	// The download object, mostly to update download progress.
	download *download
	mu       sync.Mutex

	// The SiaFile from which data is being downloaded.
	renterFile *dxfile.Snapshot

	// Caching related fields
	staticStreamCache *streamCache
}
