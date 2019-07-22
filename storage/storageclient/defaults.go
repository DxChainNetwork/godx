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
	DefaultPacketSize       = 4 * 4096

	// frequency to check whether storage client is online
	OnlineCheckFrequency = time.Second * 10

	// the amount of time that can pass for processing activating worker pool
	WorkerActivateTimeout = time.Minute * 5

	// how long to wait for a worker after a worker failed to perform a download task.
	DownloadFailureCooldown = time.Second * 3

	// how many times a bad host's timeout/cool down can be doubled before a maximum cool down is reached.
	MaxConsecutivePenalty = 10
)

// Max memory available
const (
	DefaultMaxMemory = uint64(3 * 1 << 28)
)

// Default params about upload/download process
var (
	// healthCheckInterval defines the maximum amount of time that should pass
	// in between checking the health of a file or directory.
	HealthCheckInterval = 30 * time.Minute

	// MaxConsecutiveSegmentUploads is the maximum number of segment before rebuilding the heap.
	MaxConsecutiveSegmentUploads = 100

	// repairStuckChunkInterval defines how long the storage client sleeps between
	// trying to repair a stuck chunk. The uploadHeap prioritizes stuck chunks
	// so this interval is to allow time for unstuck chunks to be repaired.
	// Ideally the uploadHeap is spending 95% of its time repairing unstuck
	// chunks.
	RepairStuckSegmentInterval = 10 * time.Minute

	// uploadAndRepairErrorSleepDuration indicates how long a upload process
	// should sleep before retrying if there is an error fetching the metadata
	// of the root directory of the storage client's filesystem.
	UploadAndRepairErrorSleepDuration = 15 * time.Minute

	// RemoteRepairDownloadThreshold indicates the threshold in percent under
	// which the storage client starts repairing a file that is not available on disk
	RemoteRepairDownloadThreshold = 0.125

	// UploadFailureCoolDown is the initial time of punishment while upload consecutive fails
	// the punishment time shows exponential growth
	UploadFailureCoolDown = 3 * time.Second
)

var keys = []string{"fund", "hosts", "period", "renew", "storage", "upload", "download",
	"redundancy", "violation", "uploadspeed", "downloadspeed"}
