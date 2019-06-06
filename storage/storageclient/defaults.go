// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"time"
)

// Files and directories related constant
const (
	PersistDirectory            = "storageclient"
	PersistFilename             = "storageclient.json"
	PersistStorageClientVersion = "1.0"
	DxPathRoot                  = "dxfiles"
)

// StorageClient Settings, where 0 means unlimited
const (
	DefaultMaxDownloadSpeed = 0
	DefaultMaxUploadSpeed   = 0
	DefaultStreamCacheSize  = 2

	// frequency to check whether storage client is online
	OnlineCheckFrequency = time.Second * 10

	// the amount of time that can pass for processing activating worker pool
	WorkerActivateTimeout = time.Minute * 5

	// how long to wait for a worker after a worker failed to perform a download task.
	DownloadFailureCooldown = time.Second * 3

	// how many times a bad host's timeout/cooldown can be doubled before a maximum cooldown is reached.
	MaxConsecutivePenalty = 10
)

// Max memory available
const (
	DefaultMaxMemory = uint64(3 * 1 << 28)
)

// Backup Header
const (
	encryptionPlaintext = "plaintext"
	encryptionTwofish   = "twofish"
	encryptionVersion   = "1.0"
)

// Default params about upload/download process
var (
	// fileRepairInterval defines how long the renter should wait before
	// continuing to repair a file that was recently repaired.
	FileRepairInterval = 5 * time.Minute

	// healthCheckInterval defines the maximum amount of time that should pass
	// in between checking the health of a file or directory.
	HealthCheckInterval = 1 * time.Hour

	// MaxConsecutiveSegmentUploads is the maximum number of segment before rebuilding the heap.
	MaxConsecutiveSegmentUploads = 100

	// offlineCheckFrequency is how long the renter will wait to check the
	// online status if it is offline.
	OfflineCheckFrequency = 10 * time.Second

	// rebuildChunkHeapInterval defines how long the renter sleeps between
	// checking on the filesystem health.
	RebuildSegmentHeapInterval = 15 * time.Minute

	// repairStuckChunkInterval defines how long the renter sleeps between
	// trying to repair a stuck chunk. The uploadHeap prioritizes stuck chunks
	// so this interval is to allow time for unstuck chunks to be repaired.
	// Ideally the uploadHeap is spending 95% of its time repairing unstuck
	// chunks.
	RepairStuckSegmentInterval = 10 * time.Minute

	// uploadAndRepairErrorSleepDuration indicates how long a repair process
	// should sleep before retrying if there is an error fetching the metadata
	// of the root directory of the renter's filesystem.
	UploadAndRepairErrorSleepDuration = 15 * time.Minute

	// RemoteRepairDownloadThreshold indicates the threshold in percent under
	// which the renter starts repairing a file that is not available on disk
	RemoteRepairDownloadThreshold = 0.25

	UploadFailureCoolDown = 1 * time.Minute
)
