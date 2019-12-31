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

	// SafeSize indicates that the least number of validators in dpos consensus
	SafeSize = MaxValidatorSize*2/3 + 1

	// ConsensusSize indicates that a confirmed block needs the least number of validators to approve
	ConsensusSize = MaxValidatorSize*2/3 + 1

	// RewardRatioDenominator is the max value of reward ratio
	RewardRatioDenominator uint64 = 100

	// ThawingEpochDuration defines that if user cancel candidates or vote, the deposit will be thawed after 4 epochs
	ThawingEpochDuration = 4

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
	DonationRatio = uint64(10)

	// PercentageDenominator is used to calculate percentage
	PercentageDenominator = uint64(100)

	// MaxRewardedCandidateCount is the max count of candidates that not became validator should be rewarded
	MaxRewardedCandidateCount = 79

	// ValidatorRewardRatio is the ratio of reward for validator in a whole block reward
	ValidatorRewardRatio = uint64(80)

	// SubstituteCandidatesRewardRatio is the ratio of reward for substitute candidates in a whole block reward
	SubstituteCandidatesRewardRatio = uint64(20)
)

var (
	// reward 115 dx for successfully mining a block
	rewardPerBlock = common.NewBigIntUint64(1e18).MultInt64(115)

	// minDeposit defines the minimum deposit of candidate
	minDeposit = common.NewBigIntUint64(1e18).MultInt64(10000)
)
