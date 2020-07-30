// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import "github.com/DxChainNetwork/godx/common"

const (
	// Fixed number of extra-data prefix bytes reserved for signer vanity
	extraVanity = 32

	// Fixed number of extra-data suffix bytes reserved for signer seal
	extraSeal = 65

	// Number of recent block signatures to keep in memory
	inmemorySignatures = 4096

	// MaxValidatorSize indicates that the max number of validators in dpos consensus
	MaxValidatorSize = 21
	// Dynamic maxValidatorSize after hard fork dip 7, 8
	MaxValidatorSizeUnder60    = 21
	MaxValidatorSizeFrom60To90 = 33
	MaxValidatorSizeOver90     = 66

	// RewardRatioDenominator is the max value of reward ratio
	RewardRatioDenominator uint64 = 100

	// ThawingEpochDuration defines that if user cancel candidates or vote, the deposit will be thawed after 4 epochs
	ThawingEpochDuration = 4

	// ThawingEpochDurationAfterDip8 defines that if user cancel candidates or vote, the deposit will be thawed after 1 epochs
	ThawingEpochDurationAfterDip8 = 1

	// eligibleValidatorDenominator defines the denominator of the minimum expected block. If a validator
	// produces block less than expected by this denominator, it is considered as ineligible.
	eligibleValidatorDenominator = 2

	// BlockInterval indicates that a block will be produced every 8 seconds
	BlockInterval = int64(8)

	// EpochInterval indicates that a new epoch will be elected every a day
	EpochInterval = int64(86400)

	// MaxVoteCount is the maximum number of candidates that a vote transaction could
	// include
	MaxVoteCount = 30

	// DonationRatio is the value of donation ratio for every new block reward
	DonationRatio = uint64(5)

	// PercentageDenominator is used to calculate percentage
	PercentageDenominator = uint64(100)

	// BlockCountPerYear is the count of blocks produced per year
	BlockCountPerYear = uint64(3942000)

	// CalculateAverageForDip8 used to cal average epoch deposit and candidates length
	CalculateAverageForDip8 = 14
)

var (
	// rewardPerBlockAfterOneYear defines reward 115 dx for successfully mining a block after one year
	rewardPerBlockAfterOneYear = common.NewBigIntUint64(1e18).MultInt64(115)

	// minDeposit defines the minimum deposit of candidate
	minDeposit = common.NewBigIntUint64(1e18).MultInt64(10000)

	// minDepositAfterFork defines the minimum deposit of candidate after the hard fork
	minDepositAfterDip8 = common.NewBigIntUint64(1e18).MultInt64(20000000)

	// totalBlockReward defines the total block reward for main net
	totalBlockReward = common.NewBigInt(1e18).MultInt64(4.5e10)

	// rewardPerBlockFirstYear is block reward for the first year
	// means that the prev 3942000 blocks will be rewarded as 254 dx
	// After a hard fork, The reward becomes dynamic as epoch depost changes.
	rewardPerBlockFirstYear = common.NewBigIntUint64(1e18).MultInt64(254)

	// rewardPerBlock when the passed 14 average deposit < 150 * 100 million dx
	rewardDepositUnder150 = common.NewBigIntUint64(1e18).MultInt64(254)

	// rewardPerBlock when the passed 14 average deposit >= 150 * 100 million dx && < 200 * 100 million dx
	rewardDepositFrom150To200 = common.NewBigIntUint64(1e18).MultInt64(342)

	// rewardPerBlock when the passed 14 average deposit >= 200 * 100 million dx && < 250 * 100 million dx
	rewardDepositFrom200To250 = common.NewBigIntUint64(1e18).MultInt64(419)

	// rewardPerBlock when the passed 14 average deposit >= 250 * 100 million dx && < 300 * 100 million dx
	rewardDepositFrom250To300 = common.NewBigIntUint64(1e18).MultInt64(482)

	// rewardPerBlock when the passed 14 average deposit >= 300 * 100 million dx && < 350 * 100 million dx
	rewardDepositFrom300To350 = common.NewBigIntUint64(1e18).MultInt64(533)

	// rewardPerBlock when the passed 14 average deposit >= 350 * 100 million dx
	rewardDepositOver350 = common.NewBigIntUint64(1e18).MultInt64(571)

	totalDeposit150 = common.NewBigIntUint64(1e18).MultInt64(1e8).MultInt64(150)
	totalDeposit200 = common.NewBigIntUint64(1e18).MultInt64(1e8).MultInt64(200)
	totalDeposit250 = common.NewBigIntUint64(1e18).MultInt64(1e8).MultInt64(250)
	totalDeposit300 = common.NewBigIntUint64(1e18).MultInt64(1e8).MultInt64(300)
	totalDeposit350 = common.NewBigIntUint64(1e18).MultInt64(1e8).MultInt64(350)
)
