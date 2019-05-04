// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import "github.com/DxChainNetwork/godx/common"

// TODO (mzhang): the blocks time is dependent on block frequency. Normally, it will be 10 min/b
// but discuss with the team
var (
	BlockPerHour = uint64(6)
	BlocksPerDay = 24 * BlockPerHour
	BlocksPerWeek = 7 * BlocksPerDay
	BlocksPerMonth = 30 * BlocksPerDay
	BlocksPerYear = 365 * BlocksPerDay
)

// TODO (mzhang): Discuss with team, came up with reasonable default value
var (
	DefaultRentPayment = RentPayment {
		Payment: common.NewBigInt(500),
		StorageHosts: 50,
		Period: 3 * BlocksPerMonth,
		RenewWindow: BlocksPerMonth,

		ExpectedStorage: 1e12, // 1 TB
		ExpectedUpload:     uint64(200e9) / BlocksPerMonth, // 200 GB per month
		ExpectedDownload:   uint64(100e9) / BlocksPerMonth, // 100 GB per month
		ExpectedRedundancy: 3.0,
	}
)
