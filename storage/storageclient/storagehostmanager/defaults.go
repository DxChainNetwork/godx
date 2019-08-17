// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"time"

	"github.com/DxChainNetwork/godx/common/unit"
)

// Those values are used to calculate the storage host evaluation
const (
	priceFloor               = float64(0.1)
	priceExponentiationSmall = 0.75
	priceExponentiationLarge = 5
)

// StorageHostManager related constant
const (
	saveFrequency                    = 2 * time.Minute
	PersistStorageHostManagerHeader  = "Storage Host Manager Settings"
	PersistStorageHostManagerVersion = "1.0"
	PersistFilename                  = "storagehostmanager.json"
)

// Scan related constants
const (
	scanOnlineCheckDuration = 30 * time.Second
	scanCheckDuration       = time.Second
	scanQuantity            = 2500
	maxScanSleep            = 6 * time.Hour
	minScanSleep            = time.Hour + time.Minute*30

	maxWorkersAllowed = 80
	minScans          = 12

	maxDowntime = 10 * 24 * time.Hour
)

// Presence factor related constants
const (
	lowValueLimit  = 0.50
	lowTimeLimit   = 0
	highValueLimit = 1.00
	highTimeLimit  = 100 * unit.BlocksPerDay
)

// deposit factor related constants
const (
	depositBaseDivider float64 = 3
)

// storage factor related constants
const (
	storageBaseDivider float64 = 10
)

// interaction related fields
const (
	initialSuccessfulInteractionFactor         = 10
	initialFailedInteractionFactor             = 0
	interactionDecay                   float64 = 0.999999
	interactionExponentialIndex                = 4
	maxNumInteractionRecord                    = 10
)

// uptime related fields
const (
	initialAccumulatedUptime   = 21600
	initialAccumulatedDowntime = 0
	uptimeDecay                = 0.999999
	uptimeExponentialIndex     = 4

	// If the host has an uptime rate as 0.98, it has full score in uptimeFactor
	uptimeCap               = 0.98
	uptimeMaxNumScanRecords = 5
)

// host manager remove criteria
const (
	critIntercept  = 0.30
	critRemoveBase = unit.BlocksPerDay * 3
)
