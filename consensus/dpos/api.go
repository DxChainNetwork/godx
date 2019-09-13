// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/trie"
	"github.com/syndtr/goleveldb/leveldb"
)

// API is a user facing RPC API to allow controlling the delegate and voting
// mechanisms of the delegated-proof-of-stake
type API struct {
	chain consensus.ChainReader
	dpos  *Dpos
}

// GetValidators retrieves the list of the validators at specified block
func (api *API) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}

	if header == nil {
		return nil, errUnknownBlock
	}

	db := trie.NewDatabase(api.dpos.db)
	epochTrie, err := types.NewEpochTrie(header.DposContext.EpochRoot, db)
	if err != nil {
		return nil, err
	}

	dposContext := types.DposContext{}
	dposContext.SetEpoch(epochTrie)
	return dposContext.GetValidators()
}

func (api *API) GetCandidates(number *rpc.BlockNumber) ([]common.Address, error) {
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}

	if header == nil {
		return nil, errUnknownBlock
	}

	// get the new candidate trie
	db := trie.NewDatabase(api.dpos.db)
	candidateTrie, err := types.NewCandidateTrie(header.DposContext.CandidateRoot, db)
	if err != nil {
		return nil, err
	}

	// iterate through the candidates trie
	var candidates []common.Address
	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	for iterCandidate.Next() {
		candidateAddr := common.BytesToAddress(iterCandidate.Value)
		candidates = append(candidates, candidateAddr)
	}

	// return candidates
	return candidates, nil
}

func (api *API) GetCandidateDeposit(candidateAddress common.Address) (*big.Int, error) {
	header := api.chain.CurrentHeader()
	statedb, err := state.New(header.Root, state.NewDatabase(api.dpos.db))
	if err != nil {
		return nil, err
	}

	candidateDepositHash := statedb.GetState(candidateAddress, KeyCandidateDeposit)
	return candidateDepositHash.Big(), nil
}

func (api *API) GetVoteDeposit(voteAddress common.Address) (*big.Int, error) {
	header := api.chain.CurrentHeader()
	statedb, err := state.New(header.Root, state.NewDatabase(api.dpos.db))
	if err != nil {
		return nil, err
	}

	voteDepositHash := statedb.GetState(voteAddress, KeyVoteDeposit)
	return voteDepositHash.Big(), nil
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

// GetVotedCandidatesByAddress retrieve all voted candidates of given delegator at now
func (api *API) GetVotedCandidatesByAddress(delegator common.Address) ([]common.Address, error) {
	currentHeader := api.chain.CurrentHeader()
	db := trie.NewDatabase(api.dpos.db)
	voteTrie, err := types.NewVoteTrie(currentHeader.DposContext.VoteRoot, db)
	if err != nil {
		return nil, err
	}

	dposContext := types.DposContext{}
	dposContext.SetVote(voteTrie)
	return dposContext.GetVotedCandidatesByAddress(delegator)
}
