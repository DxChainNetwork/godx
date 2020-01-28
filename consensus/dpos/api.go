// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/big"
	"sort"

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
func GetValidatorInfo(stateDb *state.StateDB, validatorAddress common.Address, diskdb ethdb.Database, header *types.Header) (uint64, int64, int64, error) {
	rewardRatio := GetRewardRatioNumeratorLastEpoch(stateDb, validatorAddress)
	minedCount, err := getMinedBlocksCount(diskdb, header, validatorAddress)
	epochID := CalculateEpochID(header.Time.Int64())
	if err != nil {
		return 0, 0, 0, err
	}

	// return validator information
	return rewardRatio, minedCount, epochID, nil
}

// GetCandidateInfo will return the detailed candidates information
func GetCandidateInfo(stateDb *state.StateDB, candidateAddress common.Address, header *types.Header, trieDb *trie.Database) (common.BigInt, common.BigInt, uint64, error) {

	// get the candidateTrie
	delegateTrie, err := types.NewDelegateTrie(header.DposContext.DelegateRoot, trieDb)
	if err != nil {
		return common.BigInt0, common.BigInt0, 0, fmt.Errorf("failed to recover the candidateTrie based on the root: %s", err.Error())
	}
	candidateVotes := CalcCandidateTotalVotes(candidateAddress, stateDb, delegateTrie)
	candidateDeposit := GetCandidateDeposit(stateDb, candidateAddress)
	rewardRatio := GetRewardRatioNumerator(stateDb, candidateAddress)

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

// BlockChain is the interface for the blockchain backend for api helper
type BlockChain interface {
	GetHeaderByNumber(bn uint64) *types.Header
	GetHeaderByHash(hash common.Hash) *types.Header
	GetHeader(hash common.Hash, bn uint64) *types.Header
	StateAt(hash common.Hash) (*state.StateDB, error)
	DposCtxAt(root *types.DposContextRoot) (*types.DposContext, error)
}

type (
	// ApiHelper is the helper structure for dpos related api commands
	ApiHelper struct {
		bc BlockChain
	}
)

// LastElectBlockHeight return the block header of last election
func (ah *ApiHelper) lastElectBlockHeader(header *types.Header) (*types.Header, error) {
	genesis := ah.bc.GetHeaderByNumber(0)
	if genesis == nil {
		return 0, fmt.Errorf("nil genesis block")
	}
	genesisTime := genesis.Time.Int64()

	cur := header.Number.Uint64()
	if cur <= 1 {
		return genesis, nil
	}
	curNum := header.Number.Uint64() - 1
	curHeader := ah.bc.GetHeader(header.ParentHash, curNum)
	for ; curNum > 0; curNum -= 1 {
		parHeader := ah.bc.GetHeader(curHeader.ParentHash, curNum-1)
		if parHeader == nil {
			return nil, fmt.Errorf("unknown block %x at %v", curHeader.ParentHash, curNum-1)
		}
		if isElectBlock(curHeader, parHeader, genesisTime) {
			return curHeader, nil
		}
		curHeader = parHeader
	}
	return nil, nil
}

type (
	// VoteEntry is an entry for a delegator's vote
	VoteEntry struct {
		Addr common.Address
		Vote common.BigInt
	}

	// votesDescending is the descending sorting of the VoteEntry slice
	votesDescending []VoteEntry
)

func (vd votesDescending) Len() int           { return len(vd) }
func (vd votesDescending) Swap(i, j int)      { vd[i], vd[j] = vd[j], vd[i] }
func (vd votesDescending) Less(i, j int) bool { return vd[i].Vote.Cmp(vd[j].Vote) > 0 }

// candidateVoteStat return the vote stat for a candidate
func (ah *ApiHelper) CandidateVoteStat(cand common.Address, header *types.Header) ([]VoteEntry, error) {
	ctx, statedb, err := ah.dposCtxAndStateAt(header)
	if err != nil {
		return nil, err
	}
	if !isCandidate(ctx.CandidateTrie(), cand) {
		return nil, fmt.Errorf("%x is not a valid candidate", cand)
	}
	delegators := getAllDelegatorForCandidate(ctx, cand)
	res := make([]VoteEntry, 0, len(delegators))
	for _, delegator := range delegators {
		vote := GetVoteDeposit(statedb, delegator)
		res = append(res, VoteEntry{
			Addr: delegator,
			Vote: vote,
		})
	}
	sort.Sort(votesDescending(res))
	return res, nil
}

// ValidatorVoteStat return the vote stat for a validator in last epoch
func (ah *ApiHelper) ValidatorVoteStat(validator common.Address, header *types.Header) ([]VoteEntry, error) {
	ctx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}
	if is, err := isValidator(ctx, validator); err != nil {
		return nil, err
	} else if !is {
		return nil, fmt.Errorf("%x is not a validator", validator)
	}
	lastElect, err := ah.lastElectBlockHeader(header)
	if err != nil {
		return nil, err
	}
	return ah.CandidateVoteStat(validator, lastElect)
}

// ValidatorRewardInRange return the validator reward within range start and end.
func (ah *ApiHelper) ValidatorRewardInRange(validator common.Address, start, end uint64) (*big.Int, error) {
	return nil, nil
}

func isValidator(dposCtx *types.DposContext, addr common.Address) (bool, error) {
	validators, err := dposCtx.GetValidators()
	if err != nil {
		return false, err
	}
	for _, validator := range validators {
		if validator == addr {
			return true, nil
		}
	}
	return false, nil
}

// dposCtxAndStateAt return the dposCtx and statedb at a given block header
func (ah *ApiHelper) dposCtxAndStateAt(header *types.Header) (*types.DposContext, *state.StateDB, error) {
	ctx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, nil, err
	}
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return nil, nil, err
	}
	return ctx, statedb, nil
}

// isElect return whether the block specified by header is a block where election
// takes place or not.
func isElectBlock(header, parent *types.Header, genesisTime int64) bool {
	genesisEpoch := CalculateEpochID(genesisTime)
	prevEpoch := CalculateEpochID(parent.Time.Int64())
	curEpoch := CalculateEpochID(header.Time.Int64())
	if prevEpoch == curEpoch {
		return false
	}
	if prevEpoch == genesisEpoch {
		return false
	}
	return true
}
