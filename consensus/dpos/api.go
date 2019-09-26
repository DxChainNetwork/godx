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
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/trie"
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

// GetValidators will return the validator list based on the block header provided
func GetValidators(diskdb ethdb.Database, header *types.Header) ([]common.Address, error) {
	// re-construct trieDB and get the epochTrie
	trieDb := trie.NewDatabase(diskdb)
	epochTrie, err := types.NewEpochTrie(header.DposContext.EpochRoot, trieDb)
	if err != nil {
		return nil, fmt.Errorf("failed to recover the epochTrie based on the root: %s", err.Error())
	}

	// construct dposContext and get validators
	dposContext := types.DposContext{}
	dposContext.SetEpoch(epochTrie)
	return dposContext.GetValidators()
}

// IsValidator checks if the given address is a validator address
func IsValidator(diskdb ethdb.Database, header *types.Header, addr common.Address) error {
	validators, err := GetValidators(diskdb, header)
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

// GetCandidates will return the candidates list based on the block header provided
func GetCandidates(diskdb ethdb.Database, header *types.Header) ([]common.Address, error) {
	// re-construct trieDB and get the candidateTrie
	trieDb := trie.NewDatabase(diskdb)
	candidateTrie, err := types.NewCandidateTrie(header.DposContext.CandidateRoot, trieDb)
	if err != nil {
		return nil, fmt.Errorf("failed to recover the candidateTrie based on the root: %s", err.Error())
	}

	// based on the candidateTrie, get the list of candidates and return
	dposContext := types.DposContext{}
	dposContext.SetCandidate(candidateTrie)
	return dposContext.GetCandidates(), nil
}

// GetValidatorInfo will return the detailed validator information
func GetValidatorInfo(stateDb *state.StateDB, validatorAddress common.Address, diskdb ethdb.Database, header *types.Header) (common.BigInt, uint64, int64, int64, error) {
	votes := GetTotalVote(stateDb, validatorAddress)
	rewardRatio := GetRewardRatioNumeratorLastEpoch(stateDb, validatorAddress)
	minedCount, err := getMinedBlocksCount(diskdb, header, validatorAddress)
	epochID := CalculateEpochID(header.Time.Int64())
	if err != nil {
		return common.BigInt0, 0, 0, 0, err
	}

	// return validator information
	return votes, rewardRatio, minedCount, epochID, nil
}

// GetCandidateInfo will return the detailed candidates information
func GetCandidateInfo(stateDb *state.StateDB, candidateAddress common.Address, header *types.Header, trieDb *trie.Database) (common.BigInt, common.BigInt, uint64, error) {
	// get detailed candidates information
	candidateDeposit := GetCandidateDeposit(stateDb, candidateAddress)

	// get the candidateTrie
	delegateTrie, err := types.NewDelegateTrie(header.DposContext.DelegateRoot, trieDb)
	if err != nil {
		return common.BigInt0, common.BigInt0, 0, fmt.Errorf("failed to recover the candidateTrie based on the root: %s", err.Error())
	}
	candidateVotes := CalcCandidateTotalVotes(candidateAddress, stateDb, delegateTrie)
	rewardRatio := GetRewardRatioNumeratorLastEpoch(stateDb, candidateAddress)

	return candidateDeposit, candidateVotes, rewardRatio, nil
}

// getMinedBlocksCount will return the number of blocks mined by the validator within the current epoch
func getMinedBlocksCount(diskdb ethdb.Database, header *types.Header, validatorAddress common.Address) (int64, error) {
	// re-construct the minedCntTrie
	trieDb := trie.NewDatabase(diskdb)
	minedCntTrie, err := types.NewMinedCntTrie(header.DposContext.MinedCntRoot, trieDb)
	if err != nil {
		return 0, fmt.Errorf("failed to recover the minedCntTrie based on the root: %s", err.Error())
	}

	// based on the header, calculate the epochID
	epochID := CalculateEpochID(header.Time.Int64())

	// construct dposContext and get mined count
	dposContext := types.DposContext{}
	dposContext.SetMinedCnt(minedCntTrie)
	return dposContext.GetMinedCnt(epochID, validatorAddress), nil
}
