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

// rentPayment filed validation min requirement
const (
	minHosts = 2
)

var currencyIndexMap = map[string]int{
	"ndx": 0,
	"udx": 1,
	"mdx": 2,
	"dx":  3,
	"Kdx": 4,
	"Mdx": 5,
	"Gdx": 6,
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

var keys = []string{"fund", "hosts", "period", "renew", "storage", "upload", "download",
	"redundancy", "violation", "uploadspeed", "downloadspeed"}
