// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

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

		genesis  *types.Header
		geneOnce sync.Once
		geneErr  error
	}
)

// NewApiHelper creates a new api helper
func NewApiHelper(bc BlockChain) *ApiHelper {
	return &ApiHelper{
		bc: bc,
	}
}

// LastElectBlockHeight return the block header of last election
func (ah *ApiHelper) lastElectBlockHeader(header *types.Header) (*types.Header, error) {
	cur := header.Number.Uint64()
	if cur <= 1 {
		return ah.genesis, nil
	}
	curNum := header.Number.Uint64() - 1
	curHeader := ah.bc.GetHeader(header.ParentHash, curNum)
	for ; curNum > 0; curNum -= 1 {
		parHeader := ah.bc.GetHeader(curHeader.ParentHash, curNum-1)
		if parHeader == nil {
			return nil, fmt.Errorf("unknown block %x at %v", curHeader.ParentHash, curNum-1)
		}
		if is, err := ah.isElectBlock(curHeader); err != nil {
			return nil, err
		} else if is {
			return curHeader, nil
		}
		curHeader = parHeader
	}
	return ah.getGenesis()
}

// candidateVoteStat return the vote stat for a candidate
func (ah *ApiHelper) CandidateVoteStat(cand common.Address, header *types.Header) ([]ValueEntry, error) {
	ctx, err := ah.epochCtxAt(header)
	if err != nil {
		return nil, err
	}
	if !isCandidate(ctx.DposContext.CandidateTrie(), cand) {
		return nil, fmt.Errorf("%x is not a valid candidate", cand)
	}
	delegators := getAllDelegatorForCandidate(ctx.DposContext, cand)
	res := make([]ValueEntry, 0, len(delegators))
	for _, delegator := range delegators {
		vote := GetVoteDeposit(ctx.stateDB, delegator)
		res = append(res, ValueEntry{
			Addr:  delegator,
			Value: vote,
		})
	}
	sort.Sort(valuesDescending(res))
	return res, nil
}

// ValidatorVoteStat return the vote stat for a validator in last epoch
func (ah *ApiHelper) ValidatorVoteStat(validator common.Address, header *types.Header) ([]ValueEntry, error) {
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
// The donation has already been subtracted from the rewards
func (ah *ApiHelper) ValidatorRewardInRange(validator common.Address, endHeader *types.Header, size uint64) (common.BigInt, error) {
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

// forEachBlockInRange apply the function f on all blocks in range
// The range ends in endBlock specified by endHash, and the range is of size `size`.
// Note: the start and end block is included within the range.
// 		 if the size is greater than end height, blocks 1 to end will be iterated.
func (ah *ApiHelper) forEachBlockInRange(endHeader *types.Header, size uint64, f func(header *types.Header) error) error {
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

// isValidator returns whether the addr is a validator in a given dposCtx
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

// CalcValidatorDistributionInRange calculate the validator reward distribution to delegators
// within a range. Ending with endHash and covers number of (size) blocks
func (ah *ApiHelper) CalcValidatorDistributionInRange(validator common.Address, endHeader *types.Header, size uint64) ([]ValueEntry, error) {
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
		delegatorVotes, err := ah.ValidatorVoteStat(validator, end)
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

// epochInterval specifies the epoch interval for calculating validator distribution in range.
// an epoch interval is specified by a endHeader and the size of the interval.
type epochInterval struct {
	endHeader *types.Header
	size      uint64
}

// calcEpochIntervalInRange calculate the epochInterval within the range endHash of size
// It will split a full interval into small intervals by elect blocks
func (ah *ApiHelper) calcEpochIntervalInRange(endHeader *types.Header, size uint64) ([]epochInterval, error) {
	var eis []epochInterval
	if size == 0 {
		return eis, nil
	}
	curEpoch := epochInterval{
		endHeader: endHeader,
		size:      0,
	}
	err := ah.forEachBlockInRange(endHeader, size, func(header *types.Header) error {
		prevHeader := ah.bc.GetHeaderByHash(header.ParentHash)
		if prevHeader == nil {
			return fmt.Errorf("unkown header %x", header.ParentHash)
		}
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
func (ah *ApiHelper) epochCtxAt(header *types.Header) (*EpochContext, error) {
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

// getValidatorRewardRatio get the validator reward ratio in the last epoch specified
// by header.
func (ah *ApiHelper) getValidatorRewardRatio(validator common.Address, header *types.Header) (uint64, error) {
	statedb, err := ah.bc.StateAt(header.Root)
	if err != nil {
		return 0, err
	}
	rewardRatio := GetRewardRatioNumeratorLastEpoch(statedb, validator)
	return rewardRatio, nil
}

// addDelegatorDistribution add the delegator distribution to dist.
func (ah *ApiHelper) addDelegatorDistribution(dist validatorDistribution, rewards common.BigInt, votes []ValueEntry, rewardRatio uint64) {
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
func (ah *ApiHelper) isElectBlock(header *types.Header) (bool, error) {
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

// loadGenesis will load genesis info to ApiHelper's genesis fields.
// If an error happens, the error will be save to ah.geneErr
func (ah *ApiHelper) getGenesisTimeStamp() (int64, error) {
	genesis, err := ah.getGenesis()
	if err != nil {
		return 0, err
	}
	return genesis.Time.Int64(), nil
}

// getGenesis return the genesis block
func (ah *ApiHelper) getGenesis() (*types.Header, error) {
	if err := ah.loadGenesis(); err != nil {
		return nil, err
	}
	return ah.genesis, nil
}

// loadGenesis load the genesis into api helper
func (ah *ApiHelper) loadGenesis() error {
	ah.geneOnce.Do(func() {
		genesis := ah.bc.GetHeaderByNumber(0)
		if genesis == nil {
			ah.geneErr = errors.New("unknown genesis block")
		}
		ah.genesis = genesis
	})
	return ah.geneErr
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
