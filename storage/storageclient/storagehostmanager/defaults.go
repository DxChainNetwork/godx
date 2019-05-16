// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import "time"

// Those values are used to calculate the storage host evaluation
const (
	priceFloor                = float64(0.1)
	depositFloor              = priceFloor * 2
	depositExponentialSmall   = 4
	depositExponentialLarge   = 0.5
	interactionExponentiation = 10
	priceExponentiationSmall  = 0.75
	priceExponentiationLarge  = 5
	minStorage                = uint64(20e9)
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
	//maxScanSleep            = 6 * time.Hour
	//minScanSleep            = time.Hour + time.Minute*30

	// TODO (mzhang): for testing purpose
	maxScanSleep = 30 * time.Second
	minScanSleep = 10 * time.Second

	maxWorkersAllowed = 80
	minScans          = 12
	maxDowntime       = 10 * 24 * time.Hour
)

// historical interaction with host related constants
const (
	historicInteractionDecay      = 0.9995
	historicInteractionDecayLimit = 500
	recentInteractionWeightLimit  = 0.01
)
