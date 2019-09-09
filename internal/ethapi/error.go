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

	// ErrRatioNotStringFormat is returned if the ApplyCandidateTx input a none string ratio parameter
	ErrRatioNotStringFormat = errors.New("the parameter ratio is not string format")

	// ErrParseStringToUint is returned if fail to convert string ratio to uint8 in ApplyCandidateTx
	ErrParseStringToUint = errors.New("failed to parse string to uint8")

	// ErrFromNotStringFormat is returned if dpos tx input a none string from parameter
	ErrFromNotStringFormat = errors.New("the parameter from is not string format")

	// ErrDepositNotStringFormat is returned if dpos tx input a none string deposit parameter
	ErrDepositNotStringFormat = errors.New("the parameter deposit is not string format")

	// ErrParseStringToBigInt is returned if fail to convert string deposit to big int in dpos tx
	ErrParseStringToBigInt = errors.New("failed to parse string to big int")

	// ErrCandidatesNotStringFormat is returned if vote tx input a none string candidates parameter
	ErrCandidatesNotStringFormat = errors.New("the parameter candidates is not string format")

	// ErrBeyondMaxVoteSize is returned if vote beyond 30 candidates
	ErrBeyondMaxVoteSize = errors.New("vote beyond the max size 30")

	// ErrRLPEncodeCandidates is returned if rlp encode voted candidates
	ErrRLPEncodeCandidates = errors.New("failed to rlp encode candidate list")

	// ErrUnknownParameter is returned if input unknown parameter name in dpos tx
	ErrUnknownParameter = errors.New("unknown parameter name,cannot parse it")
)
