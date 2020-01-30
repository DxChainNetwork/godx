// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
)

// PublicDposAPI object is used to implement all
// DPOS related APIs
type PublicDposAPI struct {
	bc *core.BlockChain
	ah *dpos.APIHelper
}

// CandidateInfo stores detailed candidate information
type CandidateInfo struct {
	Candidate   common.Address `json:"candidate"`
	Deposit     *big.Int       `json:"deposit"`
	TotalVotes  *big.Int       `json:"total_votes"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// ValidatorInfo stores detailed validator information
type ValidatorInfo struct {
	Validator   common.Address `json:"validator"`
	TotalVotes  *big.Int       `json:"total_votes"`
	EpochID     int64          `json:"current_epoch"`
	MinedBlocks int64          `json:"epoch_mined_blocks"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// DelegatorInfo stores delegator info
type DelegatorInfo struct {
	Delegator common.Address `json:"delegator"`
	Vote      *big.Int       `json:"vote"`
}

// DelegatorReward stores how many reward should be distributed to delegator
type DelegatorReward struct {
	Delegator common.Address `json:"delegator"`
	Reward    *big.Int       `json:"reward"`
}

// NewPublicDposAPI will create a PublicDposAPI object that is used
// to access all DPOS API Method
func NewPublicDposAPI(e *Ethereum) *PublicDposAPI {
	return &PublicDposAPI{
		bc: e.BlockChain(),
		ah: dpos.NewAPIHelper(e.BlockChain()),
	}
}

// Validators will return a list of validators based on the blockNumber provided
func (d *PublicDposAPI) Validators(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	// return the list of validators
	return d.ah.GetValidators(header)
}

// Validator will return detailed validator's information based on the validator address provided
func (d *PublicDposAPI) Validator(addr common.Address, blockNr *rpc.BlockNumber) (ValidatorInfo, error) {
	// based on the block number, get the block header
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return ValidatorInfo{}, err
	}
	vi, err := d.ah.GetValidatorInfo(addr, header)
	if err != nil {
		return ValidatorInfo{}, err
	}
	return ValidatorInfo{
		Validator:   addr,
		TotalVotes:  vi.TotalVotes.BigIntPtr(),
		RewardRatio: vi.RewardRatio,
		MinedBlocks: vi.MinedCount,
		EpochID:     vi.EpochID,
	}, nil
}

// Candidates will return a list of candidates information based on the blockNumber provided
func (d *PublicDposAPI) Candidates(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	return d.ah.GetCandidates(header)
}

// Candidate will return detailed candidate's information based on the candidate address provided
func (d *PublicDposAPI) Candidate(addr common.Address, blockNr *rpc.BlockNumber) (CandidateInfo, error) {
	// based on the block number, retrieve the header
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return CandidateInfo{}, err
	}
	ci, err := d.ah.GetCandidateInfo(addr, header)
	if err != nil {
		return CandidateInfo{}, err
	}
	return CandidateInfo{
		Candidate:   addr,
		Deposit:     ci.Deposit.BigIntPtr(),
		TotalVotes:  ci.TotalVotes.BigIntPtr(),
		RewardRatio: ci.RewardRatio,
	}, nil

}

// CandidateDeposit is used to check how much deposit a candidate has put in
func (d *PublicDposAPI) CandidateDeposit(addr common.Address, blockNr *rpc.BlockNumber) (*big.Int, error) {
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	deposit, err := d.ah.GetCandidateDeposit(addr, header)
	if err != nil {
		return nil, err
	}
	return deposit.BigIntPtr(), nil
}

// VoteDeposit checks the vote deposit paid based on the voteAddress provided
func (d *PublicDposAPI) VoteDeposit(addr common.Address, blockNr *rpc.BlockNumber) (*big.Int, error) {
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	voteDeposit, err := d.ah.GetVoteDeposit(addr, header)
	if err != nil {
		return nil, err
	}
	return voteDeposit.BigIntPtr(), nil
}

// EpochID will calculates the epoch id based on the block number provided
func (d *PublicDposAPI) EpochID(blockNr *rpc.BlockNumber) (int64, error) {
	// get the block header information based on the block number
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return 0, nil
	}
	// calculate epochID and return
	return dpos.CalculateEpochID(header.Time.Int64()), nil
}

// GetVotedCandidatesByAddress query all voted candidates by the delegator on the block
func (d *PublicDposAPI) GetVotedCandidatesByAddress(delegator common.Address, blockNr *rpc.BlockNumber) ([]common.Address, error) {
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	return d.ah.GetVotedCandidates(delegator, header)
}

// GetAllDelegatorsOfCandidate query all delegators that voted the candidate on the block
func (d *PublicDposAPI) GetAllVotesOfCandidate(candidate common.Address, blockNr *rpc.BlockNumber) ([]DelegatorInfo, error) {
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	ves, err := d.ah.CandidateVoteStat(candidate, header)
	if err != nil {
		return nil, err
	}
	return delegatorInfoFromValueEntries(ves), nil
}

// GetAllVotesOfValidator returns all delegator that voted the validator in the last epoch
// before blockNr.
func (d *PublicDposAPI) GetAllVotesOfValidator(validator common.Address, blockNr *rpc.BlockNumber) ([]DelegatorInfo, error) {
	header, err := getHeaderByNumber(blockNr, d.bc)
	if err != nil {
		return nil, err
	}
	ves, err := d.ah.ValidatorVoteStat(validator, header)
	if err != nil {
		return nil, err
	}
	return delegatorInfoFromValueEntries(ves), nil
}

// GetValidatorReward returns validator's earned reward within a range of [startNr, endNr]
func (d *PublicDposAPI) GetValidatorReward(validator common.Address, startNr, endNr *rpc.BlockNumber) (*big.Int, error) {
	endHeader, size, err := parseStartEndBlockNumber(startNr, endNr, d.bc)
	if err != nil {
		return nil, err
	}
	reward, err := d.ah.ValidatorRewardInRange(validator, endHeader, size)
	if err != nil {
		return nil, err
	}
	return reward.BigIntPtr(), nil
}

// GetValidatorDistribution get the validator distribution to delegator from startNr to endNr.
// Return a slice of DelegatorReward
func (d *PublicDposAPI) GetValidatorDistribution(validator common.Address, startNr, endNr *rpc.BlockNumber) ([]DelegatorReward, error) {
	endHeader, size, err := parseStartEndBlockNumber(startNr, endNr, d.bc)
	if err != nil {
		return nil, err
	}
	ves, err := d.ah.CalcValidatorDistributionInRange(validator, endHeader, size)
	if err != nil {
		return nil, err
	}
	return delegatorRewardFromValueEntries(ves), nil
}

// parseStartEndBlockNumber validate and parse start and end number to the end header and size of interval.
// Return an error if validation failed.
func parseStartEndBlockNumber(start, end *rpc.BlockNumber, bc *core.BlockChain) (*types.Header, uint64, error) {
	if start == nil {
		return nil, 0, fmt.Errorf("start block number must be specified")
	}
	if (start != nil && start.Int64() < 0) || (end != nil && end.Int64() < 0) {
		return nil, 0, fmt.Errorf("invalid interval: [%v, %v]", start, end)
	}
	var endHeader *types.Header
	if end == nil {
		endHeader = bc.CurrentHeader()
	} else {
		endHeader = bc.GetHeaderByNumber(uint64(end.Int64()))
	}
	endNr := endHeader.Number.Int64()
	startNr := start.Int64()
	if startNr > endNr {
		return nil, 0, fmt.Errorf("invalid interval: [%v, %v]", startNr, endNr)
	}
	return endHeader, uint64(endNr - startNr), nil
}

// delegatorInfoFromValueEntries convert []dpos.ValueEntry to []DelegatorInfo
func delegatorInfoFromValueEntries(ves []dpos.ValueEntry) []DelegatorInfo {
	dis := make([]DelegatorInfo, 0, len(ves))
	for _, ve := range ves {
		dis = append(dis, DelegatorInfo{
			Delegator: ve.Addr,
			Vote:      ve.Value.BigIntPtr(),
		})
	}
	return dis
}

// delegatorRewardFromValueEntries convert []dpos.ValueEntry to []DelegatorReward
func delegatorRewardFromValueEntries(ves []dpos.ValueEntry) []DelegatorReward {
	drs := make([]DelegatorReward, 0, len(ves))
	for _, ve := range ves {
		drs = append(drs, DelegatorReward{
			Delegator: ve.Addr,
			Reward:    ve.Value.BigIntPtr(),
		})
	}
	return drs
}

// getHeaderByNumber will return the block header information based on the block number provided
func getHeaderByNumber(blockNr *rpc.BlockNumber, bc *core.BlockChain) (*types.Header, error) {
	// based on the block number, get the block header
	var header *types.Header
	if blockNr == nil {
		header = bc.CurrentHeader()
	} else {
		header = bc.GetHeaderByNumber(uint64(blockNr.Int64()))
	}
	// sanity check
	if header == nil {
		return nil, fmt.Errorf("unknown block")
	}
	// return
	return header, nil
}
