// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

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

// BlockChain is the interface for the blockchain backend for api helper
type BlockChain interface {
	GetHeaderByNumber(bn uint64) *types.Header
	GetHeaderByHash(hash common.Hash) *types.Header
	GetHeader(hash common.Hash, bn uint64) *types.Header
	StateAt(hash common.Hash) (*state.StateDB, error)
	DposCtxAt(root *types.DposContextRoot) (*types.DposContext, error)
}

type (
	// APIHelper is the api helper for dpos related api commands
	APIHelper struct {
		bc BlockChain

		genesis *types.Header
	}

	// CandidateInfo is the structure for candidate information
	CandidateInfo struct {
		Deposit     common.BigInt
		TotalVotes  common.BigInt
		RewardRatio uint64
	}

	// ValidatorInfo is the structure for validator information.
	// Note that in this structure, reward ratio and votes are stats from the last epoch.
	ValidatorInfo struct {
		TotalVotes  common.BigInt
		RewardRatio uint64
		MinedCount  int64
		EpochID     int64
	}
)

// NewAPIHelper creates a new api helper
func NewAPIHelper(bc BlockChain) *APIHelper {
	return &APIHelper{
		bc: bc,
	}
}

// GetCandidates return the candidate list based on the block header provided.
func (ah *APIHelper) GetCandidates(header *types.Header) ([]common.Address, error) {
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}
	return dposCtx.GetCandidates()
}

// CandidateInfo return the candidate info of a given address
// Return an error if the given addr is not a candidate
func (ah *APIHelper) CandidateInfo(addr common.Address, header *types.Header) (CandidateInfo, error) {
	if err := ah.checkIsCandidate(addr, header); err != nil {
		return CandidateInfo{}, err
	}
	return ah.candidateInfo(addr, header)
}

// candidateInfo return the candidate info of a given address
func (ah *APIHelper) candidateInfo(addr common.Address, header *types.Header) (CandidateInfo, error) {
	epochCtx, err := ah.epochCtxAt(header)
	if err != nil {
		return CandidateInfo{}, err
	}
	dposCtx, statedb := epochCtx.DposContext, epochCtx.stateDB

	totalVotes := CalcCandidateTotalVotes(addr, statedb, dposCtx.DelegateTrie())
	deposit := GetCandidateDeposit(statedb, addr)
	rewardRatio := GetRewardRatioNumerator(statedb, addr)
	return CandidateInfo{
		Deposit:     deposit,
		TotalVotes:  totalVotes,
		RewardRatio: rewardRatio,
	}, nil
}

// CandidateVoteStat return the vote stat for a candidate. The result is a list of structure of address
// and vote value, which is sorted in descending votes. Return an error if the given address
// is not a candidate.
func (ah *APIHelper) CandidateVoteStat(addr common.Address, header *types.Header) ([]ValueEntry, error) {
	if err := ah.checkIsCandidate(addr, header); err != nil {
		return nil, err
	}
	return ah.candidateVoteStat(addr, header)
}

// candidateVoteStat return the vote stat for a candidate. The result is a list of structure of address
// and vote value, which is sorted in descending votes.
func (ah *APIHelper) candidateVoteStat(addr common.Address, header *types.Header) ([]ValueEntry, error) {
	ctx, err := ah.epochCtxAt(header)
	if err != nil {
		return nil, err
	}
	statedb, dposCtx := ctx.stateDB, ctx.DposContext
	// For each delegator who voted the candidate, count the delegator vote and add to result.
	delegators := getAllDelegatorForCandidate(dposCtx, addr)
	res := make([]ValueEntry, 0, len(delegators))
	for _, delegator := range delegators {
		vote := GetVoteDeposit(statedb, delegator)
		res = append(res, ValueEntry{
			Addr:  delegator,
			Value: vote,
		})
	}
	// Sort the entries in descending order and return result
	sort.Sort(valuesDescending(res))
	return res, nil
}

// GetCandidateDeposit return the candidate deposit of the address in the given header.
// Return an error if the given address is not a candidate.
func (ah *APIHelper) GetCandidateDeposit(addr common.Address, header *types.Header) (common.BigInt, error) {
	if err := ah.checkIsCandidate(addr, header); err != nil {
		return common.BigInt0, err
	}
	return ah.getCandidateDeposit(addr, header)
}

// getCandidateDeposit return the candidate deposit of the address in the given header
func (ah *APIHelper) getCandidateDeposit(addr common.Address, header *types.Header) (common.BigInt, error) {
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return common.BigInt0, err
	}
	return GetCandidateDeposit(statedb, addr), nil
}

// checkIsCandidate checks whether the addr is a candidate in the given header. If not,
// return an error.
func (ah *APIHelper) checkIsCandidate(addr common.Address, header *types.Header) error {
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return err
	}
	if is, err := isCandidate(dposCtx, addr); err != nil {
		return err
	} else if !is {
		return fmt.Errorf("%x is not a valid candidate", addr)
	}
	return nil
}

// GetValidators return the validator list based on the block header provided
func (ah *APIHelper) GetValidators(header *types.Header) ([]common.Address, error) {
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}
	return dposCtx.GetValidators()
}

// GetValidatorTotalVotes returns the total votes for a validator in the last epoch.
// The votes include the deposit from himself and votes from delegator.
// If the given address is not a validator, return an error.
func (ah *APIHelper) GetValidatorTotalVotes(addr common.Address, header *types.Header) (common.BigInt, error) {
	if err := ah.checkIsValidator(addr, header); err != nil {
		return common.BigInt0, err
	}
	return ah.getValidatorTotalVotes(addr, header)
}

// getValidatorTotalVotes returns the total votes for a validator in the last epoch.
// The votes include the deposit from himself and votes from delegator.
func (ah *APIHelper) getValidatorTotalVotes(addr common.Address, header *types.Header) (common.BigInt, error) {
	ves, err := ah.ValidatorVoteStat(addr, header)
	if err != nil {
		return common.BigInt0, err
	}
	votes := common.BigInt0
	for _, ve := range ves {
		votes = votes.Add(ve.Value)
	}
	deposit, err := ah.getValidatorDeposit(addr, header)
	if err != nil {
		return common.BigInt0, err
	}
	return votes.Add(deposit), nil
}

// GetValidatorInfo returns the validator info for an address with a given header.
// If the given addr is not a validator, return an error.
func (ah *APIHelper) GetValidatorInfo(addr common.Address, header *types.Header) (ValidatorInfo, error) {
	if err := ah.checkIsValidator(addr, header); err != nil {
		return ValidatorInfo{}, err
	}
	return ah.getValidatorInfo(addr, header)
}

// getValidatorInfo returns the validator info for an address with a given header.
func (ah *APIHelper) getValidatorInfo(addr common.Address, header *types.Header) (ValidatorInfo, error) {
	votes, err := ah.getValidatorTotalVotes(addr, header)
	if err != nil {
		return ValidatorInfo{}, err
	}
	rewardRatio, err := ah.getValidatorRewardRatio(addr, header)
	if err != nil {
		return ValidatorInfo{}, err
	}
	minedCnt, err := ah.getValidatorMinedBlocksCount(addr, header)
	if err != nil {
		return ValidatorInfo{}, err
	}
	epochID := CalculateEpochID(header.Time.Int64())
	return ValidatorInfo{
		TotalVotes:  votes,
		RewardRatio: rewardRatio,
		MinedCount:  minedCnt,
		EpochID:     epochID,
	}, nil
}

// ValidatorVoteStat return the vote stat for a validator in last epoch.
// First identify whether the given address is a validator, and then call validatorVoteStat
// to obtain the vote stat from last elect block.
func (ah *APIHelper) ValidatorVoteStat(addr common.Address, header *types.Header) ([]ValueEntry, error) {
	if err := ah.checkIsValidator(addr, header); err != nil {
		return nil, err
	}
	return ah.validatorVoteStat(addr, header)
}

// validatorVoteStat return the vote stat for a validator in last epoch.
func (ah *APIHelper) validatorVoteStat(addr common.Address, header *types.Header) ([]ValueEntry, error) {
	lastElect, err := ah.lastElectBlockHeader(header)
	if err != nil {
		return nil, err
	}
	if err := ah.checkIsCandidate(addr, lastElect); err != nil {
		return nil, err
	}
	return ah.candidateVoteStat(addr, lastElect)
}

// ValidatorRewardInRange return the validator reward within range start and end.
// The donation has already been subtracted from the rewards
func (ah *APIHelper) ValidatorRewardInRange(validator common.Address, endHeader *types.Header, size uint64) (common.BigInt, error) {
	total := common.BigInt0
	err := ah.forEachBlockInRange(endHeader, size, func(header *types.Header) error {
		ec, err := ah.epochCtxAt(header)
		if err != nil {
			return err
		}
		expValidator, err := ec.lookupValidator(header.Time.Int64())
		if err != nil {
			return err
		}
		if expValidator == validator {
			vReward := calcValidatorReward(header.Number, ec.stateDB)
			total = total.Add(vReward)
		}
		return nil
	})
	if err != nil {
		return common.BigInt0, err
	}
	return total, nil
}

// CalcValidatorDistributionInRange calculate the validator reward distribution to delegators
// within a range. Ending with endHash and covers number of (size) blocks
//  1. Split the full interval by elect blocks.
//  2. For each interval
//     2.1 calculate validator rewards
//     2.2 calculate validator vote stats (in the last epoch)
//     2.3 find reward ratio of validator (in the last epoch)
//     2.4 split shared rewards among delegators and add to result
//  3. return result
func (ah *APIHelper) CalcValidatorDistributionInRange(validator common.Address, endHeader *types.Header, size uint64) ([]ValueEntry, error) {
	// First, divide the whole range into epochs
	eis, err := ah.calcEpochIntervalInRange(endHeader, size)
	if err != nil {
		return nil, err
	}
	dist := make(validatorDistribution)
	// For each epoch calculate the distribution
	for _, interval := range eis {
		end := interval.endHeader
		size := interval.size
		totalReward, err := ah.ValidatorRewardInRange(validator, end, size)
		if err != nil {
			return nil, err
		}
		delegatorVotes, err := ah.validatorVoteStat(validator, end)
		if err != nil {
			return nil, err
		}
		rewardRatio, err := ah.getValidatorRewardRatio(validator, end)
		if err != nil {
			return nil, err
		}
		ah.addDelegatorDistribution(dist, totalReward, delegatorVotes, rewardRatio)
	}
	return dist.sortedList(), nil
}

// IsValidator checks whether the given address is a validator in the block header provided.
func (ah *APIHelper) checkIsValidator(addr common.Address, header *types.Header) error {
	validators, err := ah.GetValidators(header)
	if err != nil {
		return err
	}
	for _, validator := range validators {
		if validator == addr {
			return nil
		}
	}
	return fmt.Errorf("%x is not a valid validator", addr)
}

// getValidatorRewardRatio get the validator reward ratio in the last epoch specified
// by header.
func (ah *APIHelper) getValidatorRewardRatio(validator common.Address, header *types.Header) (uint64, error) {
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return 0, err
	}
	rewardRatio := GetRewardRatioNumeratorLastEpoch(statedb, validator)
	return rewardRatio, nil
}

// getValidatorDeposit return the validator deposit in the last epoch specified by
// header.
func (ah *APIHelper) getValidatorDeposit(validator common.Address, header *types.Header) (common.BigInt, error) {
	lastElect, err := ah.lastElectBlockHeader(header)
	if err != nil {
		return common.BigInt0, err
	}
	statedb, err := ah.bc.StateAt(lastElect.Root)
	if err != nil {
		return common.BigInt0, err
	}
	deposit := GetCandidateDeposit(statedb, validator)
	return deposit, nil
}

// GetVoteDeposit return the vote deposit of a given delegator address
func (ah *APIHelper) GetVoteDeposit(addr common.Address, header *types.Header) (common.BigInt, error) {
	if err := ah.checkHasVoted(addr, header); err != nil {
		return common.BigInt0, err
	}
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return common.BigInt0, err
	}
	return GetVoteDeposit(statedb, addr), nil
}

// GetVotedCandidates return the voted candidates for an address in the specified header
func (ah *APIHelper) GetVotedCandidates(addr common.Address, header *types.Header) ([]common.Address, error) {
	if err := ah.checkHasVoted(addr, header); err != nil {
		return nil, err
	}
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}
	return dposCtx.GetVotedCandidatesByAddress(addr)
}

// checkHasVoted checks whether the address has voted in the header.
// If not voted, an error will be returned.
func (ah *APIHelper) checkHasVoted(addr common.Address, header *types.Header) error {
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return err
	}
	if is, err := hasVoted(addr, dposCtx); err != nil {
		return err
	} else if !is {
		return fmt.Errorf("%x is not a delegator", addr)
	}
	return nil
}

// LastElectBlockHeight return the block header of last election
func (ah *APIHelper) lastElectBlockHeader(header *types.Header) (*types.Header, error) {
	cur := header.Number.Uint64()
	if cur <= 1 {
		return ah.getGenesis()
	}
	curHeader := header
	for curNum := header.Number.Uint64() - 1; curNum > 0; curNum -= 1 {
		curHeader = ah.bc.GetHeader(curHeader.ParentHash, curNum)
		if curHeader == nil {
			return nil, fmt.Errorf("unknown block %x at %v", curHeader.ParentHash, curNum-1)
		}
		if is, err := ah.isElectBlock(curHeader); err != nil {
			return nil, err
		} else if is {
			return curHeader, nil
		}
	}
	return ah.getGenesis()
}

// forEachBlockInRange apply the function f on all blocks in range
// The range ends in endBlock specified by endHash, and the range is of size `size`.
// Note: the start and end block is included within the range.
// 		 if the size is greater than end height, blocks 1 to end will be iterated.
func (ah *APIHelper) forEachBlockInRange(endHeader *types.Header, size uint64, f func(header *types.Header) error) error {
	// Validation. Avoid size > endHeader.Number
	endHeight := endHeader.Number.Uint64()
	if size > endHeight {
		size = endHeight
	}
	// Initialization
	curHeader := endHeader
	// Iterate number of size times
	for i := uint64(0); i < size; i++ {
		if f != nil {
			if err := f(curHeader); err != nil {
				return err
			}
		}
		// Proceed to next iteration
		parHash := curHeader.ParentHash
		curHeader = ah.bc.GetHeaderByHash(parHash)
		if curHeader == nil {
			return fmt.Errorf("unknwon block %x", parHash)
		}
	}
	return nil
}

// calcValidatorReward calculate for the total assigned reward to a validator of
// a given block.
func calcValidatorReward(number *big.Int, state stateDB) common.BigInt {
	blockReward := getBlockReward(number, state)
	if blockReward.Cmp(common.BigInt0) == 0 {
		return common.BigInt0
	}
	donation := blockReward.MultUint64(DonationRatio).DivUint64(PercentageDenominator)
	return blockReward.Sub(donation)
}

// epochInterval specifies the epoch interval for calculating validator distribution in range.
// an epoch interval is specified by a endHeader and the size of the interval.
type epochInterval struct {
	endHeader *types.Header
	size      uint64
}

// calcEpochIntervalInRange calculate the epochInterval within the range endHash of size
// It will split a full interval into small intervals by elect blocks
func (ah *APIHelper) calcEpochIntervalInRange(endHeader *types.Header, size uint64) ([]epochInterval, error) {
	var eis []epochInterval
	if size == 0 {
		return eis, nil
	}
	curEpoch := epochInterval{
		endHeader: endHeader,
		size:      0,
	}
	err := ah.forEachBlockInRange(endHeader, size, func(header *types.Header) error {
		// If the current header is not an elect block, simply increment curEpoch.size
		// and move on
		if is, err := ah.isElectBlock(header); err != nil {
			return err
		} else if !is {
			curEpoch.size += 1
			return nil
		}
		// Else add the epoch to eis and create a new epoch
		if curEpoch.size != 0 {
			eis = append(eis, curEpoch)
		}
		curEpoch = epochInterval{
			endHeader: header,
			size:      1,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if curEpoch.size != 0 {
		eis = append(eis, curEpoch)
	}
	return eis, nil
}

// dposCtxAndStateAt return the dposCtx and statedb at a given block header
func (ah *APIHelper) epochCtxAt(header *types.Header) (*EpochContext, error) {
	ctx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return nil, err
	}
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return nil, err
	}
	return &EpochContext{
		TimeStamp:   header.Time.Int64(),
		DposContext: ctx,
		stateDB:     statedb,
	}, nil
}

// addDelegatorDistribution add the delegator distribution to dist.
func (ah *APIHelper) addDelegatorDistribution(dist validatorDistribution, rewards common.BigInt, votes []ValueEntry, rewardRatio uint64) {
	sharedRewards := rewards.MultUint64(rewardRatio).DivUint64(RewardRatioDenominator)
	totalVotes := common.BigInt0
	for _, entry := range votes {
		totalVotes = totalVotes.Add(entry.Value)
	}
	for _, entry := range votes {
		addr := entry.Addr
		reward := sharedRewards.Mult(entry.Value).Div(totalVotes)
		dist.addValue(addr, reward)
	}
}

// isElectBlock returns whether the given header is a block where election happens.
func (ah *APIHelper) isElectBlock(header *types.Header) (bool, error) {
	if header.Number.Cmp(big.NewInt(0)) == 0 {
		// genesis is not a elect block
		return false, nil
	}
	geneTime, err := ah.getGenesisTimeStamp()
	if err != nil {
		return false, err
	}
	parent := ah.bc.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return false, fmt.Errorf("unknown block %x", header.ParentHash)
	}
	return isElectBlock(header, parent, geneTime), nil
}

// loadGenesis will load genesis info to APIHelper's genesis fields.
// If an error happens, the error will be save to ah.geneErr
func (ah *APIHelper) getGenesisTimeStamp() (int64, error) {
	genesis, err := ah.getGenesis()
	if err != nil {
		return 0, err
	}
	return genesis.Time.Int64(), nil
}

// getGenesis return the genesis block. If the genesis block has not read, read into memory.
func (ah *APIHelper) getGenesis() (*types.Header, error) {
	if ah.genesis == nil {
		if err := ah.loadGenesis(); err != nil {
			return nil, err
		}
	}
	return ah.genesis, nil
}

// loadGenesis load the genesis into api helper
func (ah *APIHelper) loadGenesis() error {
	ah.genesis = ah.bc.GetHeaderByNumber(0)
	var err error
	if ah.genesis == nil {
		err = errors.New("unknown genesis block")
	}
	return err
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

// getValidatorMinedBlocksCount will return the number of blocks mined by the validator within the current epoch
func (ah *APIHelper) getValidatorMinedBlocksCount(addr common.Address, header *types.Header) (int64, error) {
	dposCtx, err := ah.bc.DposCtxAt(header.DposContext)
	if err != nil {
		return 0, err
	}
	epochID := CalculateEpochID(header.Time.Int64())
	return dposCtx.GetMinedCnt(epochID, addr), nil
}

type (
	// ValueEntry is an entry for a a certain value for an address
	ValueEntry struct {
		Addr  common.Address
		Value common.BigInt
	}

	// valuesDescending is the descending sorting of the ValueEntry slice
	valuesDescending []ValueEntry
)

func (vd valuesDescending) Len() int           { return len(vd) }
func (vd valuesDescending) Swap(i, j int)      { vd[i], vd[j] = vd[j], vd[i] }
func (vd valuesDescending) Less(i, j int) bool { return vd[i].Value.Cmp(vd[j].Value) > 0 }

// validatorDistribution is the data structure to record the tokens to be given to
// delegators of a certain validator
type validatorDistribution map[common.Address]common.BigInt

// merge merges two validatorDistributions. After the function calls, the result will be
// updated in the receiver.
func (vd validatorDistribution) merge(vd2 validatorDistribution) {
	for addr, v2 := range vd2 {
		if v1, ok := vd[addr]; !ok {
			// Create a new address entry in vd
			vd[addr] = v2
		} else {
			vd[addr] = v1.Add(v2)
		}
	}
}

// addReward add value to an address in ValidatorDistribution
func (vd validatorDistribution) addValue(addr common.Address, value common.BigInt) {
	if v, ok := vd[addr]; !ok {
		vd[addr] = value
	} else {
		vd[addr] = v.Add(value)
	}
}

// sortedList change the validatorDistribution to a sorted list
func (vd validatorDistribution) sortedList() []ValueEntry {
	ves := make([]ValueEntry, 0, len(vd))
	for addr, v := range vd {
		ves = append(ves, ValueEntry{addr, v})
	}
	sort.Sort(valuesDescending(ves))
	return ves
}
