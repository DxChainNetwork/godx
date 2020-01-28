// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/trie"
)

// PublicDposAPI object is used to implement all
// DPOS related APIs
type PublicDposAPI struct {
	e *Ethereum
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
	EpochID     int64          `json:"current_epoch"`
	MinedBlocks int64          `json:"epoch_mined_blocks"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// DelegatorReward stores detailed delegator reward
type DelegatorReward struct {
	Delegator common.Address `json:"delegator"`
	Reward    *big.Int       `json:"reward"`
}

// DelegatorInfo stores delegator info
type DelegatorInfo struct {
	Delegator common.Address `json:"delegator"`
	Deposit   *big.Int       `json:"deposit"`
}

// NewPublicDposAPI will create a PublicDposAPI object that is used
// to access all DPOS API Method
func NewPublicDposAPI(e *Ethereum) *PublicDposAPI {
	return &PublicDposAPI{
		e: e,
	}
}

// Validators will return a list of validators based on the blockNumber provided
func (d *PublicDposAPI) Validators(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	// return the list of validators
	return dpos.GetValidators(d.e.ChainDb(), header)
}

// Validator will return detailed validator's information based on the validator address provided
func (d *PublicDposAPI) Validator(validatorAddress common.Address, blockNr *rpc.BlockNumber) (ValidatorInfo, error) {
	// based on the block number, get the block header
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return ValidatorInfo{}, err
	}

	// check if the given address is a validator's address
	if err := dpos.IsValidator(d.e.ChainDb(), header, validatorAddress); err != nil {
		return ValidatorInfo{}, err
	}

	// get statedb for retrieving detailed information
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return ValidatorInfo{}, err
	}

	// get the detailed information
	rewardRatio, minedCount, epochID, err := dpos.GetValidatorInfo(statedb, validatorAddress, d.e.ChainDb(), header)
	if err != nil {
		return ValidatorInfo{}, err
	}

	return ValidatorInfo{
		Validator:   validatorAddress,
		RewardRatio: rewardRatio,
		MinedBlocks: minedCount,
		EpochID:     epochID,
	}, nil
}

// Candidates will return a list of candidates information based on the blockNumber provided
func (d *PublicDposAPI) Candidates(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	return dpos.GetCandidates(d.e.ChainDb(), header)
}

// Candidate will return detailed candidate's information based on the candidate address provided
func (d *PublicDposAPI) Candidate(candidateAddress common.Address, blockNr *rpc.BlockNumber) (CandidateInfo, error) {
	// based on the block number, retrieve the header
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return CandidateInfo{}, err
	}

	// based on the block header root, get the statedb
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return CandidateInfo{}, err
	}

	// check if the given address is candidate address
	if !dpos.IsCandidate(candidateAddress, header, d.e.ChainDb()) {
		return CandidateInfo{}, fmt.Errorf("the given address %s is not a candidate", candidateAddress.String())
	}

	// get detailed information
	trieDb := trie.NewDatabase(d.e.ChainDb())
	candidateDeposit, candidateVotes, rewardRatio, err := dpos.GetCandidateInfo(statedb, candidateAddress, header, trieDb)
	if err != nil {
		return CandidateInfo{}, err
	}

	return CandidateInfo{
		Candidate:   candidateAddress,
		Deposit:     candidateDeposit,
		Votes:       candidateVotes,
		RewardRatio: rewardRatio,
	}, nil

}

// CandidateDeposit is used to check how much deposit a candidate has put in
func (d *PublicDposAPI) CandidateDeposit(candidateAddress common.Address) (*big.Int, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	// get the candidate deposit from the stateDB
	candidateDepositHash := statedb.GetState(candidateAddress, dpos.KeyCandidateDeposit)
	return candidateDepositHash.Big(), nil
}

// VoteDeposit checks the vote deposit paid based on the voteAddress provided
func (d *PublicDposAPI) VoteDeposit(voteAddress common.Address) (*big.Int, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	// get the vote deposit from the stateDB
	voteDepositHash := statedb.GetState(voteAddress, dpos.KeyVoteDeposit)
	return voteDepositHash.Big(), nil
}

// EpochID will calculates the epoch id based on the block number provided
func (d *PublicDposAPI) EpochID(blockNr *rpc.BlockNumber) (int64, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return 0, nil
	}

	// calculate epochID and return
	return dpos.CalculateEpochID(header.Time.Int64()), nil
}

// getHeaderBasedOnNumber will return the block header information based on the block number provided
func getHeaderBasedOnNumber(blockNr *rpc.BlockNumber, e *Ethereum) (*types.Header, error) {
	// based on the block number, get the block header
	var header *types.Header
	if blockNr == nil {
		header = e.BlockChain().CurrentHeader()
	} else {
		header = e.BlockChain().GetHeaderByNumber(uint64(blockNr.Int64()))
	}

	// sanity check
	if header == nil {
		return nil, fmt.Errorf("unknown block")
	}

	// return
	return header, nil
}

// GetVotedCandidatesByAddress query all voted candidates by the delegator on the block
func (d *PublicDposAPI) GetVotedCandidatesByAddress(delegator common.Address, blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	// get dpos context on the block
	dctx, err := d.e.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}

	return dctx.GetVotedCandidatesByAddress(delegator)
}

// GetAllDelegatorsOfCandidate query all delegators that voted the candidate on the block
func (d *PublicDposAPI) GetAllVotesOfCandidate(candidate common.Address, blockNr *rpc.BlockNumber) ([]DelegatorInfo, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	// get dpos context on the block
	dctx, err := d.e.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}

	var result []DelegatorInfo
	delegators, err := dctx.GetAllDelegatorsOfCandidate(candidate)
	if err != nil {
		return nil, err
	}

	for _, addr := range delegators {
		deposit, err := d.VoteDeposit(addr)
		if err != nil {
			return nil, err
		}
		result = append(result, DelegatorInfo{addr, deposit})
	}
	return result, nil
}
