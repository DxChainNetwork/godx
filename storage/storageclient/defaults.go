// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import "time"

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

// DxFile Related
const (
	DxFileExtension = ".dx"
)

var currencyIndexMap = map[string]uint64{
	"wei":        1,
	"kwei":       1e3,
	"mwei":       1e6,
	"gwei":       1e9,
	"microether": 1e12,
	"milliether": 1e15,
	"ether":      1e18,
}

var dataSizeMultiplier = map[string]uint64{
	"kb":  1e3,
	"mb":  1e6,
	"gb":  1e9,
	"tb":  1e12,
	"kib": 1 << 10,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
}

var speedMultiplier = map[string]int64{
	"bps":  1,
	"kbps": 1e3,
	"mbps": 1e6,
	"gbps": 1e9,
	"tbps": 1e12,
}

var keys = []string{"fund", "hosts", "period", "renew", "storage", "upload", "download",
	"redundancy", "violation", "uploadspeed", "downloadspeed"}
