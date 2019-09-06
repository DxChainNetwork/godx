// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import "errors"

var (
	// ErrBalanceNotEnoughCandidateThreshold is returned if the balance of caller is
	// less than candidate threshold
	ErrBalanceNotEnoughCandidateThreshold = errors.New("balance not enough candidate threshold")

	// ErrDepositValueNotSuitable is returned if the specified value of transaction is
	// not suitable to deposit
	ErrDepositValueNotSuitable = errors.New("deposit value not suitable")

	// ErrCandidateDepositTooLow is returned if the specified value of transaction is
	// too low to apply candidate
	ErrCandidateDepositTooLow = errors.New("deposit value too low to apply candidate")

	// ErrNotCandidate is returned if the caller want to cancel candidate but not become it yet
	ErrNotCandidate = errors.New("not become candidate yet")

	// ErrEmptyInput is returned if the vote tx with empty input
	ErrEmptyInput = errors.New("empty input for vote tx")

	// ErrHasNotVote is returned if the caller want to cancel vote but not voted before
	ErrHasNotVote = errors.New("has not voted before")

	// ErrUnknownPrecompileContractAddress is returned if the transaction is sent to a
	// invalid precompile contract address
	ErrUnknownPrecompileContractAddress = errors.New("invalid precompiled contract address")

	// ErrInvalidAwardDistributionRatio is returned if the ApplyCandidateTx has a invalid ratio parameter
	ErrInvalidAwardDistributionRatio = errors.New("invalid award distribution ratio,must be an integer within [0,100]")
)
