// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"time"

	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
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
	scanCheckDuration       = 200 * time.Millisecond
	scanQuantity            = 2500
	maxScanSleep            = 6 * time.Hour
	minScanSleep            = time.Hour + time.Minute*30
	maxWorkersAllowed       = 80
)

const (
	// scoreDefaultBase is the multiplier of the score as a base.
	scoreDefaultBase = 1000

	// minScore is the minimum score of a host evaluation. Host evaluation score starts at
	// this value.
	minScore = 1
)

// Presence factor related constants
const (
	// lowValueLimit is the presenceScore when block difference between the current block
	// height and host first seen is smaller than lowTimeLimit.
	lowValueLimit = 0.50

	// lowTimeLimit set the low limit of the block difference. If block passed is smaller than
	// this value, the presenceScore is of value lowValueLimit.
	lowTimeLimit = 0

	// highValueLimit is the presenceScore when block difference between the current block
	// height and host first seen is larger than highTimeLimit.
	highValueLimit = 1.00

	// highTimeLimit set the high limit of the block difference. If block passed is larger than
	// this value, the presenceScore is of value highValueLimit.
	highTimeLimit = 100 * unit.BlocksPerDay
)

// deposit factor related constants
const (
	// depositBaseDivider is the parameter to be used in depositRemaining calculation.
	// The larger the divider, the slower the function approaching asymptote y = 1 as
	// deposit grows.
	depositBaseDivider float64 = 3
)

// storage factor related constants
const (
	// storageBaseDivider is the parameter to be used in storageRemainingScore calculation.
	// The larger the divider, the slower the function approaching asymptote y = 1 as storage grows.
	storageBaseDivider float64 = 10
)

// interaction related fields
const (
	// initialSuccessfulInteractionFactor is the initial value for hostInfo.SuccessfulInteractionFactor.
	// The value is in unit the same as interaction weight.
	initialSuccessfulInteractionFactor = 10

	// initialFailedInteractionFactor is the initial value for hostInfo.FailedInteractionFactor.
	// The value is in unit the same as interaction weight. A low initial value is aimed to give
	// a new host an initial boost in scores
	initialFailedInteractionFactor = 0

	// interactionDecay is the decay factor to be multiplied to hostInfo.SuccessfulInteractionFactor
	// and hostInfo.FailedInteractionFactor each second. The value implies that the weight of
	// record 7 days ago is halved, a.k.a, the half-life of the factor is about 7 days
	interactionDecay float64 = 0.999999

	// interactionExponentialIndex is the exponential index for calculating the interactionScore.
	// Roughly, an interaction successful rate of 90% is about to give an interaction score of value
	// 0.64
	interactionExponentialIndex = 4

	// maxNumInteractionRecord is the maximum number of interaction records to be saved in
	// nodeInfo
	maxNumInteractionRecord = 30
)

// uptime related fields
const (
	// initialAccumulatedUptime is the initial value for hostInfo.AccumulatedUptimeFactor.
	// The initial value is in unit second, thus the initial uptime factor has value 6 hours,
	// which is the same as the maxScanSleep.
	initialAccumulatedUptime = 21600

	// initialAccumulatedDowntime is the initial value for hostInfo.AccumulatedDowntimeFactor.
	// The value is in unit second and has value 0. The low initial downtime is aimed to
	// give a boost for newly added hosts.
	initialAccumulatedDowntime = 0

	// uptimeDecay is the decay factor to be multiplied to hostInfo.AccumulatedUptimeFactor
	// and hostInfo.AccumulatedDowntimeFactor each second. The value implies that the
	// weight of the record 7 days ago is halved, a.k.a, the half-life of the factor is
	// about 7 days.
	uptimeDecay = 0.999999

	// uptimeExponentialIndex is the exponential index for calculating the uptimeScore.
	// Roughly, an uptimeRate of 90% is about to give an uptime score of value 0.64
	uptimeExponentialIndex = 4

	// uptimeCap is the upper cap of the uptimeRate. Any uptimeRate larger than this value
	// will have a full score (1.00) in uptimeScore
	uptimeCap = 0.98

	// uptimeMaxNumScanRecords is the maximum number of ScanRecords to be saved in nodeInfo.
	uptimeMaxNumScanRecords = 20
)

// host manager remove criteria
const (
	// critIntercept is the criteria's intercept with y axis, which is the upRate criteria when
	// upRate is 0
	critIntercept = 0.30

	// critRemoveBase is the parameter to be used in criteria calculation. The larger this
	// parameter, the slower the criteria function approaching asymptote y = 1.
	critRemoveBase = unit.BlocksPerDay * 3
)

// host market related constants
const (
	// priceUpdateInterval is the time to be passed before the host market price shall be
	// updated.
	priceUpdateInterval = 1 * time.Hour

	// floorRatio is the ratio below which the price does not count for the average
	floorRatio float64 = 0.2

	// ceilRatio is the ratio of total where the highest price does not count for the average
	ceilRatio float64 = 0.2
)

var defaultMarketPrice = storage.MarketPrice{
	ContractPrice: storage.DefaultContractPrice,
	StoragePrice:  storage.DefaultStoragePrice,
	UploadPrice:   storage.DefaultUploadBandwidthPrice,
	DownloadPrice: storage.DefaultDownloadBandwidthPrice,
	Deposit:       storage.DefaultDeposit,
	MaxDeposit:    storage.DefaultMaxDeposit,
}
