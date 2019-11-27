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

// GetValidatorInfo will return the detailed validator information
func GetValidatorInfo(stateDb *state.StateDB, dposCtx *types.DposContext, validatorAddress common.Address, header *types.Header) (common.BigInt, uint64, int64, int64, error) {
	if err := checkIsValidator(dposCtx, validatorAddress); err != nil {
		return common.BigInt0, 0, 0, 0, err
	}
	votes := GetTotalVote(stateDb, validatorAddress)
	rewardRatio := GetRewardRatioNumeratorLastEpoch(stateDb, validatorAddress)
	epochID := CalculateEpochID(header.Time.Int64())
	minedCount, err := dposCtx.GetMinedCnt(epochID, validatorAddress)
	if err != nil {
		return common.BigInt0, 0, 0, 0, err
	}
	// return validator information
	return votes, rewardRatio, minedCount, epochID, nil
}

// GetCandidateInfo will return the detailed candidates information
func GetCandidateInfo(stateDb *state.StateDB, dposCtx *types.DposContext, candidateAddress common.Address, header *types.Header) (common.BigInt, common.BigInt, uint64, error) {
	isCand, err := IsCandidate(dposCtx, candidateAddress)
	if err != nil {
		return common.BigInt0, common.BigInt0, 0, err
	}
	if !isCand {
		return common.BigInt0, common.BigInt0, 0, fmt.Errorf("address %x is not a candidate", candidateAddress)
	}
	// get detailed candidates information
	candidateDeposit := GetCandidateDeposit(stateDb, candidateAddress)
	candidateVotes := CalcCandidateTotalVotes(candidateAddress, stateDb, dposCtx.CandidateTrie())
	rewardRatio := GetRewardRatioNumerator(stateDb, candidateAddress)

	return candidateDeposit, candidateVotes, rewardRatio, nil
}
