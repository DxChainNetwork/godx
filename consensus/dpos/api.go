// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/syndtr/goleveldb/leveldb"
)

// API is a user facing RPC API to allow controlling the delegate and voting
// mechanisms of the delegated-proof-of-stake
type API struct {
	chain consensus.ChainReader
	dpos  *Dpos
}

// CandidateInfo stores detailed candidate information
type CandidateInfo struct {
	Candidate   common.Address `json:"candidate"`
	Deposit     common.BigInt  `json:"deposit"`
	Votes       common.BigInt  `json:"votes"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// ValidatorInfo stores detailed validator information
type ValidatorInfo struct {
	Validator   common.Address `json:"validator"`
	Votes       common.BigInt  `json:"votes"`
	EpochID     int64          `json:"current_epoch"`
	MinedBlocks int64          `json:"epoch_mined_blocks"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// DelegatorInfo stores detailed DelegatorInfo
type DelegatorInfo struct {
	Delegator common.Address   `json:"delegator"`
	Deposit   common.BigInt    `json:"deposit"`
	Votes     []common.Address `json:"votes"`
}

// GetConfirmedBlockNumber retrieves the latest irreversible block
func (api *API) GetConfirmedBlockNumber() (*big.Int, error) {
	var err error
	header := api.dpos.confirmedBlockHeader
	if header == nil {
		header, err = api.dpos.loadConfirmedBlockHeader(api.chain)
		if err != nil {

			// if it's leveldb.ErrNotFound, indicates that only genesis block in local, and return 0
			if err == leveldb.ErrNotFound {
				return new(big.Int).SetInt64(0), nil
			}

			// other errors, return nil
			return nil, err
		}
	}

	return header.Number, nil
}

// GetValidatorInfo will return the detailed validator information
func GetValidatorInfo(stateDb *state.StateDB, dposCtx *types.DposContext, header *types.Header, address common.Address) (ValidatorInfo, error) {
	if err := checkIsValidator(dposCtx, address); err != nil {
		return ValidatorInfo{}, err
	}
	votes := GetTotalVote(stateDb, address)
	rewardRatio := GetRewardRatioNumeratorLastEpoch(stateDb, address)
	epochID := CalculateEpochID(header.Time.Int64())
	minedCount, err := dposCtx.GetMinedCnt(epochID, address)
	if err != nil {
		return ValidatorInfo{}, err
	}
	// return validator information
	return ValidatorInfo{
		Validator:   address,
		Votes:       votes,
		RewardRatio: rewardRatio,
		MinedBlocks: minedCount,
		EpochID:     epochID,
	}, nil
}

// checkIsValidator checks if the given address is a validator address
func checkIsValidator(dposCtx *types.DposContext, addr common.Address) error {
	validators, err := dposCtx.GetValidators()
	if err != nil {
		return err
	}
	// check if the address is the validator address
	for _, validator := range validators {
		if validator == addr {
			return nil
		}
	}
	return fmt.Errorf("the given address %s is not a validator's address", addr.String())
}

// GetCandidateInfo returns the detailed candidates information
func GetCandidateInfo(stateDb *state.StateDB, dposCtx *types.DposContext, address common.Address) (CandidateInfo, error) {
	isCand, err := IsCandidate(dposCtx, address)
	if err != nil {
		return CandidateInfo{}, err
	}
	if !isCand {
		return CandidateInfo{}, fmt.Errorf("address %x is not a candidate", address)
	}
	// get detailed candidates information
	candidateDeposit := GetCandidateDeposit(stateDb, address)
	candidateVotes := CalcCandidateTotalVotes(address, stateDb, dposCtx.CandidateTrie())
	rewardRatio := GetRewardRatioNumerator(stateDb, address)

	return CandidateInfo{
		Candidate:   address,
		Deposit:     candidateDeposit,
		Votes:       candidateVotes,
		RewardRatio: rewardRatio,
	}, nil
}

// GetDelegatorInfo returns the detailed delegator information
func GetDelegatorInfo(stateDb *state.StateDB, dposCtx *types.DposContext, address common.Address) (DelegatorInfo, error) {
	isDelegator, err := HasVoted(dposCtx, address)
	if err != nil {
		return DelegatorInfo{}, err
	}
	if !isDelegator {
		return DelegatorInfo{}, fmt.Errorf("address %x is not a delegator", address)
	}
	deposit := GetVoteDeposit(stateDb, address)
	votes, err := dposCtx.GetVotedCandidatesByAddress(address)
	if err != nil {
		return DelegatorInfo{}, err
	}
	return DelegatorInfo{
		Delegator: address,
		Deposit:   deposit,
		Votes:     votes,
	}, nil
}
