// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"errors"
	"fmt"
)

var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")
	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")
	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte suffix signature missing")
	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash  = errors.New("non empty uncle hash")
	errInvalidDifficulty = errors.New("invalid difficulty")

	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// ErrWaitForPrevBlock is returned if the last block has not arrived and it is not
	// time for producing the next block.
	ErrWaitForPrevBlock = errors.New("wait for last block arrived")

	// ErrMinedFutureBlock is returned if the block is expected to be mined in the future
	ErrMinedFutureBlock = errors.New("mined the future block")

	// ErrMismatchSignerAndValidator is returned if the signer and validator is not matched
	ErrMismatchSignerAndValidator = errors.New("mismatch block signer and validator")

	// ErrInvalidBlockValidator is returned if the validator is not valid for a block
	ErrInvalidBlockValidator = errors.New("invalid block validator")

	// ErrNilBlockHeader is returned if returning a nil block header in api functions
	ErrNilBlockHeader = errors.New("nil block header returned")
)

var (
	// errVoteZeroOrNegativeDeposit happens when voting with zero or negative deposit
	errVoteZeroOrNegativeDeposit = errors.New("cannot vote with zero or negative deposit")

	// errVoteZeroCandidates happens when voting with zero candidates
	errVoteZeroCandidates = errors.New("cannot vote with zero candidates")

	// errVoteTooManyCandidates happens when voting more than MaxVoteCount candidates
	errVoteTooManyCandidates = fmt.Errorf("cannot vote more than %v candidates", MaxVoteCount)

	// errVoteInsufficientBalance happens when voting, the account has less balance than needed.
	errVoteInsufficientBalance = errors.New("insufficient balance to vote")

	// errCandidateInsufficientDeposit happens when processing a candidate transaction, found
	// that the candidate's deposit is lower than the threshold
	errCandidateInsufficientDeposit = fmt.Errorf("candidate argument not qualified - minimum deposit: %v", minDeposit)

	// errCandidateInvalidRewardRatio happens when processing a candidate transaction, found
	// the value of reward ratio is invalid
	errCandidateInvalidRewardRatio = fmt.Errorf("candidate argument not qualified - invalid reward ratio: must between 0 to %v", RewardRatioDenominator)

	// errCandidateDecreasingDeposit happens when processing a candidate transaction, found the
	// value of deposit is decreasing.
	errCandidateDecreasingDeposit = errors.New("candidate argument not qualified - candidate deposit shall not be decreased")

	// errCandidateDecreasingRewardRatio happens when processing a candidate transaction, found
	// the value of reward ratio is decreasing.
	errCandidateDecreasingRewardRatio = errors.New("candidate argument not qualified - candidate reward ratio shall not be decreased")

	// errCandidateInsufficientBalance happens when a user has insufficient balance for the candidate
	// transaction
	errCandidateInsufficientBalance = errors.New("candidate not qualified - candidate does not have enough balance")

	// errInsufficientFrozenAssets is the error happens when subtracting frozen assets, the diff value is
	// larger the stored frozen assets
	errInsufficientFrozenAssets = errors.New("not enough frozen assets to subtract")

	// errRandomSelectNotEnoughEntries happens if in random selection, entries is not sufficient for selection target number of
	// entries in lucky wheel algorithm
	errRandomSelectNotEnoughEntries = errors.New("not enough entries for selection")

	// errInvalidMinedBlockTime is the error indicating the block mining time is not in the right
	// time slot
	errInvalidMinedBlockTime = errors.New("invalid time to mined the block")

	// errUnknownRandomAddressSelectorType is the error type for unknown randomAddressSelector type.
	errUnknownRandomAddressSelectorType = errors.New("unknown randomAddressSelector type")

	// errCandidateInsufficientBalance indicates the candidates does not have enough balance to pay for candidate deposit
	errCandidateInsufficientBalance = errors.New("candidate does not have enough balance to pay for deposit")

	// errDelegatorInsufficientBalance indicates the delegator does not have enough balance to pay for the vote deposit
	errDelegatorInsufficientBalance = errors.New("delegator does not have enough balance to pay for the vote deposit")
)
