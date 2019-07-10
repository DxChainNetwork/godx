// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"github.com/DxChainNetwork/godx/common"
	"time"
)

// The block generation rate for Ethereum is 15s/block. Therefore, 240 blocks
// can be generated in an hour
var (
	BlockPerMin    = uint64(4)
	BlockPerHour   = uint64(240)
	BlocksPerDay   = 24 * BlockPerHour
	BlocksPerWeek  = 7 * BlocksPerDay
	BlocksPerMonth = 30 * BlocksPerDay
	BlocksPerYear  = 365 * BlocksPerDay

	ResponsibilityLockTimeout = 60 * time.Second
)

// Default rentPayment values
var (
	DefaultRentPayment = RentPayment{
		Fund:         common.NewBigInt(500),
		StorageHosts: 50,
		Period:       3 * BlocksPerMonth,
		RenewWindow:  BlocksPerMonth,

		ExpectedStorage:    1e12,                           // 1 TB
		ExpectedUpload:     uint64(200e9) / BlocksPerMonth, // 200 GB per month
		ExpectedDownload:   uint64(100e9) / BlocksPerMonth, // 100 GB per month
		ExpectedRedundancy: 3.0,
	}
)
