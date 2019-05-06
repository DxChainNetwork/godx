// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import "time"

// Those values are used to calculate the storage host evaluation
const (
	// TODO (mzhang): discuss those values with the team
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
