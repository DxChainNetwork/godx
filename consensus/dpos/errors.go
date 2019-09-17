// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"errors"
	"fmt"
)

var (
	// errVoteZeroDeposit happens when voting with zero deposit
	errVoteZeroDeposit = errors.New("cannot vote with zero deposit")

	// errInsufficientFrozenAssets is the error happens when subtracting frozen assets, the diff value is
	// larger the stored frozen assets
	errInsufficientFrozenAssets = errors.New("not enough frozen assets to subtract")

	// errCandidateInsufficientBalance happens when processing a candidate transaction, found
	// that the candidate has balance lower than the threshold
	errCandidateInsufficientBalance = fmt.Errorf("candidate not qualified - minimum balance: %v", candidateThreshold)

	// errCandidateInsufficientDeposit happens when processing a candidate transaction, found
	// that the candidate's deposit is lower than the threshold
	errCandidateInsufficientDeposit = fmt.Errorf("candidate argument not qualified - minimum deposit: %v", minDeposit)

	// errCandidateInvalidRewardRatio happens when processing a candidate transaction, found
	// the value of reward ratio is invalid
	errCandidateInvalidRewardRatio = fmt.Errorf("candidate argument not qualified - invalid reward ratio: must between 0 to %v", RewardRatioDenominator)

	// errCandidateDecreasingDeposit happens when processing a candidate transaction, found the
	// value of deposit is decreasing.
	errCandidateDecreasingDeposit = errors.New("candidate argument not qualified - candidate deposit shall not be decreased")
)
