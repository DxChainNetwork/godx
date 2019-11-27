// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"context"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
)

// PublicDposAPI is the public dpos api
type PublicDposAPI struct {
	b Backend
}

// NewPublicDposAPI creates a PublicDposAPI object that is used
// to access all DPOS API Method
func NewPublicDposAPI(b Backend) *PublicDposAPI {
	return &PublicDposAPI{
		b: b,
	}
}

// Validators returns a list of validators based on the blockNumber provided
func (d *PublicDposAPI) Validators(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	_, dposCtx, _, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return []common.Address{}, err
	}
	return dposCtx.GetValidators()
}

// Validator returns a detailed info about the validator address provided.
func (d *PublicDposAPI) Validator(address common.Address, blockNr *rpc.BlockNumber) (dpos.ValidatorInfo, error) {
	statedb, dposCtx, header, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return dpos.ValidatorInfo{}, err
	}
	return dpos.GetValidatorInfo(statedb, dposCtx, header, address)
}

// Candidates returns a list of candidates information based on the blockNumber provided
func (d *PublicDposAPI) Candidates(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	_, dposCtx, _, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return []common.Address{}, err
	}
	return dposCtx.GetCandidates()
}

// Candidate returns detailed candidate's information based on the candidate address provided
func (d *PublicDposAPI) Candidate(address common.Address, blockNr *rpc.BlockNumber) (dpos.CandidateInfo, error) {
	statedb, dposCtx, _, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return dpos.CandidateInfo{}, err
	}
	return dpos.GetCandidateInfo(statedb, dposCtx, address)
}

func (d *PublicDposAPI) Delegator(address common.Address, blockNr *rpc.BlockNumber) (dpos.DelegatorInfo, error) {
	statedb, dposCtx, _, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return dpos.DelegatorInfo{}, err
	}
	return dpos.GetDelegatorInfo(statedb, dposCtx, address)
}

// EpochID  calculates the epoch id based on the block number provided
func (d *PublicDposAPI) EpochID(blockNr *rpc.BlockNumber) (int64, error) {
	_, _, header, err := d.stateDposCtxAndHeaderByNumber(blockNr)
	if err != nil {
		return 0, nil
	}
	return dpos.CalculateEpochID(header.Time.Int64()), nil
}

// stateDposCtxAndHeaderByNumber is the adapter function for PublicDposAPI.StateDposCtxAndHeaderByNumber
func (d *PublicDposAPI) stateDposCtxAndHeaderByNumber(blockNr *rpc.BlockNumber) (*state.StateDB, *types.DposContext, *types.Header, error) {
	num := blockNumberConverter(blockNr)
	ctx := context.Background()
	statedb, dposCtx, header, err := d.b.StateDposCtxAndHeaderByNumber(ctx, num)
	if err != nil {
		return nil, nil, nil, err
	}
	if header == nil || statedb == nil || dposCtx == nil {
		return nil, nil, nil, ErrUnknownBlockNumber
	}
	return statedb, dposCtx, header, err
}

func blockNumberConverter(blockNumber *rpc.BlockNumber) rpc.BlockNumber {
	if blockNumber == nil {
		return rpc.LatestBlockNumber
	}
	return *blockNumber
}
