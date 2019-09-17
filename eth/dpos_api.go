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
)

type PublicDposAPI struct {
	e *Ethereum
}

type CandidateInfo struct {
	Candidate          common.Address
	Deposit            common.BigInt
	Votes              common.BigInt
	RewardDistribution uint64
}

type ValidatorInfo struct {
	Validator          common.Address
	Votes              common.BigInt
	MinedBlocks        int64
	RewardDistribution uint64
}

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
func (d *PublicDposAPI) Validator(validatorAddress common.Address) (ValidatorInfo, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return ValidatorInfo{}, err
	}

	// get the detailed information
	votes, rewardDistribution, minedCount, err := dpos.GetValidatorInfo(statedb, validatorAddress, d.e.ChainDb(), header)
	if err != nil {

	}

	return ValidatorInfo{
		Validator:          validatorAddress,
		Votes:              votes,
		RewardDistribution: rewardDistribution,
		MinedBlocks:        minedCount,
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
func (d *PublicDposAPI) Candidate(candidateAddress common.Address) (CandidateInfo, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return CandidateInfo{}, err
	}

	// get detailed information
	candidateDeposit, candidateVotes, rewardDistribution := dpos.GetCandidateInfo(statedb, candidateAddress)

	return CandidateInfo{
		Candidate:          candidateAddress,
		Deposit:            candidateDeposit,
		Votes:              candidateVotes,
		RewardDistribution: rewardDistribution,
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
