// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/core/state"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
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

// GetCandidates will return the candidate list based on the block header provided
func GetCandidates(diskdb ethdb.Database, header *types.Header) ([]common.Address, error) {
	// re-construct trieDB and get the candidateTrie
	trieDb := trie.NewDatabase(diskdb)
	candidateTrie, err := types.NewCandidateTrie(header.DposContext.CandidateRoot, trieDb)
	if err != nil {
		return nil, fmt.Errorf("failed to recover the candidateTrie based on the root: %s", err.Error())
	}

	// based on the candidateTrie, get the list of candidates and return
	return getCandidates(candidateTrie), nil
}

// GetValidatorInfo will return the detailed validator information
func GetValidatorInfo(stateDb *state.StateDB, validatorAddress common.Address, diskdb ethdb.Database, header *types.Header) (common.BigInt, uint64, int64, error) {
	votes := getTotalVote(stateDb, validatorAddress)
	rewardDistribution := getCandidateRewardRatioNumerator(stateDb, validatorAddress)
	minedCount, err := getMinedBlocksCount(diskdb, header, validatorAddress)
	if err != nil {
		return common.BigInt0, 0, 0, err
	}

	// return validator information
	return votes, rewardDistribution, minedCount, nil
}

// GetCandidateInfo will return the detailed candidate information
func GetCandidateInfo(stateDb *state.StateDB, candidateAddress common.Address) (common.BigInt, common.BigInt, uint64) {
	// get detailed candidate information
	candidateDeposit := getCandidateDeposit(stateDb, candidateAddress)
	candidateVotes := getTotalVote(stateDb, candidateAddress)
	rewardDistribution := getCandidateRewardRatioNumerator(stateDb, candidateAddress)

	return candidateDeposit, candidateVotes, rewardDistribution
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

// getCandidates will iterate through the candidateTrie and get all candidates
func getCandidates(candidateTrie *trie.Trie) []common.Address {
	var candidates []common.Address
	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	for iterCandidate.Next() {
		candidateAddr := common.BytesToAddress(iterCandidate.Value)
		candidates = append(candidates, candidateAddr)
	}

	return candidates
}
