// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/trie"
)

// EpochContext define current epoch context for dpos consensus
type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	stateDB     stateDB
}

// tryElect will process election at the beginning of current epoch
func (ec *EpochContext) tryElect(config *params.ChainConfig, genesis, parent *types.Header) error {
	genesisEpoch := CalculateEpochID(genesis.Time.Int64())
	prevEpoch := CalculateEpochID(parent.Time.Int64())
	currentEpoch := CalculateEpochID(ec.TimeStamp)
	// if current block does not reach new epoch, directly return
	if prevEpoch == currentEpoch {
		return nil
	}
	// thawing some deposit for currentEpoch-2
	if err := thawAllFrozenAssetsInEpoch(ec.stateDB, currentEpoch); err != nil {
		return fmt.Errorf("system not consistent: %v", err)
	}

	// if previous epoch is genesis epoch, return directly
	if prevEpoch == genesisEpoch {
		return nil
	}

	currentBlockNumber := parent.Number.Int64() + 1
	dposConfig := config.Dpos
	prevEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(prevEpochBytes, uint64(prevEpoch))
	iter := trie.NewIterator(ec.DposContext.MinedCntTrie().PrefixIterator(prevEpochBytes))
	// do election from prevEpoch to currentEpoch
	for i := prevEpoch; i < currentEpoch; i++ {
		// Get target validators numbers
		target := calMaxValidatorsNumbers(currentBlockNumber, ec.TimeStamp, ec.stateDB, dposConfig)
		safeSize := calSafeSize(target)
		// if prevEpoch is not genesis, kick out not active candidates
		if iter.Next() {
			if err := ec.kickoutValidators(prevEpoch, safeSize, currentBlockNumber, dposConfig); err != nil {
				return err
			}
		}
		// calculate the actual validators of the vote based on the attenuation
		candidateVotes, err := ec.countVotes()
		if err != nil {
			return err
		}
		// check if number of candidates is smaller than safe size
		if len(candidateVotes) < safeSize {
			return errors.New("too few candidates")
		}
		// Create the seed and pseudo-randomly select the validators
		seed := makeSeed(parent.Hash(), i)
		typeRandomSelector := ec.getRandomSelectorType(config)
		validators, err := selectValidator(candidateVotes, seed, typeRandomSelector, target)
		if err != nil {
			return err
		}
		// Set the new validators
		epochTrie, _ := types.NewEpochTrie(common.Hash{}, ec.DposContext.DB())
		ec.DposContext.SetEpoch(epochTrie)
		err = ec.DposContext.SetValidators(validators)
		if err != nil {
			return err
		}

		// Set rewardRatioLastEpoch for each validator
		for _, validator := range validators {
			ratio := GetRewardRatioNumerator(ec.stateDB, validator)
			SetRewardRatioNumeratorLastEpoch(ec.stateDB, validator, ratio)
		}

		if !dposConfig.IsDip8(currentBlockNumber) {
			// Set vote last epoch for all delegators who select the validators.
			allDelegators := allDelegatorForValidators(ec.DposContext, validators)
			for delegator := range allDelegators {
				// get the vote deposit and set it in vote last epoch
				vote := GetVoteDeposit(ec.stateDB, delegator)
				SetVoteLastEpoch(ec.stateDB, delegator, vote)
			}
		}

		// Save epoch deposit info and candidates length after dip8
		if config.Dpos.IsDip8(currentBlockNumber) {
			// Save epoch deposit
			// 1. Get candidates list in this epoch
			candidates, err := ec.DposContext.GetCandidates()
			candidatesLength := int64(len(candidates))
			if err != nil {
				log.Error("Fail to get epoch candidate, epoch = ", i)
				return err
			}
			// 2. Sum up add candidate's deposit
			totalDepositThisEpoch := common.BigInt0
			delegatorList := make(map[common.Address]common.BigInt)
			for _, candidate := range candidates {
				// candidate deposit
				deposit := GetCandidateDeposit(ec.stateDB, candidate)
				totalDepositThisEpoch = totalDepositThisEpoch.Add(deposit)
				// delegators vote deposit
				delegatorVoteDeposit := ec.getDelegatorsVoteDeposit(candidate, delegatorList)
				totalDepositThisEpoch = totalDepositThisEpoch.Add(delegatorVoteDeposit)
			}
			// 3. Save total deposit
			SetEpochTotalDeposit(ec.stateDB, currentEpoch, totalDepositThisEpoch)
			SetCandidatesNumber(ec.stateDB, currentEpoch, common.NewBigInt(candidatesLength))
		}

		log.Info("Come to new epoch", "prevEpoch", i, "nextEpoch", i+1)
	}

	if !dposConfig.IsDip8(currentBlockNumber) {
		// Finally, set the snapshot delegate trie root for accumulateRewards
		setPreEpochSnapshotDelegateTrieRoot(ec.stateDB, ec.DposContext.DelegateTrie().Hash())
	}
	return nil
}

// getDelegatorsVoteDeposit will sum all vote deposit comes from delegators for a candidate
func (ec *EpochContext) getDelegatorsVoteDeposit(candidate common.Address, delegatorList map[common.Address]common.BigInt) common.BigInt {
	delegators := getAllDelegatorForCandidate(ec.DposContext, candidate)
	totalDelegatorVoteDeposit := common.BigInt0

	for _, d := range delegators {
		if _, ok := delegatorList[d]; !ok {
			vote := GetVoteDeposit(ec.stateDB, d)
			delegatorList[d] = vote
			totalDelegatorVoteDeposit = totalDelegatorVoteDeposit.Add(vote)
		}
	}

	return totalDelegatorVoteDeposit
}

// countVotes will calculate the number of votes at the beginning of current epoch
func (ec *EpochContext) countVotes() (votes randomSelectorEntries, err error) {
	// get the needed variables
	candidateTrie := ec.DposContext.CandidateTrie()
	statedb := ec.stateDB

	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	var hasCandidate bool

	// loop through all candidates and calculate total votes for each of them
	for iterCandidate.Next() {
		// get and initialize all variables
		hasCandidate = true
		candidateAddr := common.BytesToAddress(iterCandidate.Value)
		// sanity check
		// calculate the candidates votes
		totalVotes := CalcCandidateTotalVotes(candidateAddr, statedb, ec.DposContext.DelegateTrie())
		// write the totalVotes to result and state
		votes = append(votes, &randomSelectorEntry{addr: candidateAddr, vote: totalVotes})
	}
	// if there are no candidates, return error
	if !hasCandidate {
		return votes, fmt.Errorf("countVotes failed, no candidates available")
	}
	return votes, nil
}

// kickoutValidators will kick out irresponsible validators of last epoch at the beginning of current epoch
func (ec *EpochContext) kickoutValidators(epoch int64, safeSize int, blockNumber int64, config *params.DposConfig) error {
	needKickoutValidators, err := getIneligibleValidators(ec.DposContext, epoch, blockNumber, ec.TimeStamp, ec.stateDB, config)
	if err != nil {
		return err
	}
	// no validators need kicked out
	needKickoutValidatorCnt := len(needKickoutValidators)
	if needKickoutValidatorCnt <= 0 {
		return nil
	}
	// ascend needKickoutValidators, the prev candidates have smaller mined count,
	// and they will be remove firstly
	sort.Sort(needKickoutValidators)
	// count candidates to a safe size as a threshold to remove candidates
	candidateCount := 0
	iter := trie.NewIterator(ec.DposContext.CandidateTrie().NodeIterator(nil))
	for iter.Next() {
		candidateCount++
		if candidateCount >= needKickoutValidatorCnt+safeSize {
			break
		}
	}
	// Loop over the first part of the needKickOutValidators to kick out
	for i, validator := range needKickoutValidators {
		// ensure candidates count greater than or equal to safeSize
		if candidateCount <= safeSize {
			log.Info("No more candidates can be kick out", "prevEpochID", epoch, "candidateCount", candidateCount, "needKickoutCount", len(needKickoutValidators)-i)
			return nil
		}
		// If the candidate has already canceled candidate, continue to the next
		// validator
		if is, err := isCandidate(ec.DposContext, validator.address); err != nil {
			return err
		} else if !is {
			continue
		}
		if err := ec.DposContext.KickoutCandidate(validator.address); err != nil {
			return err
		}
		// if successfully above, then mark the validator that will be thawed in next next epoch
		currentEpochID := CalculateEpochID(ec.TimeStamp)
		deposit := GetCandidateDeposit(ec.stateDB, validator.address)
		markThawingAddressAndValue(ec.stateDB, validator.address, currentEpochID, deposit, blockNumber, config)
		// set candidates deposit to 0
		SetCandidateDeposit(ec.stateDB, validator.address, common.BigInt0)
		SetRewardRatioNumerator(ec.stateDB, validator.address, 0)
		// if kickout success, candidateCount minus 1
		candidateCount--
		log.Info("Kickout candidates", "prevEpochID", epoch, "candidates", validator.address.String(), "minedCnt", validator.cnt)
	}
	return nil
}

// getIneligibleValidators return the ineligible validators in a certain epoch. An ineligible validator is
// defined as a validator who produced blocks less than half as expected
func getIneligibleValidators(ctx *types.DposContext, epoch int64, blockNumber int64, curTime int64, state stateDB, config *params.DposConfig) (addressesByCnt, error) {
	validators, err := ctx.GetValidators()
	if err != nil {
		return addressesByCnt{}, fmt.Errorf("failed to get validator: %s", err)
	}
	if len(validators) == 0 {
		return addressesByCnt{}, errors.New("no validators")
	}
	expectedBlockPerValidator := expectedBlocksPerValidatorInEpoch(timeOfFirstBlock, blockNumber, curTime, state, config)
	var ineligibleValidators addressesByCnt
	for _, validator := range validators {
		cnt := ctx.GetMinedCnt(epoch, validator)
		if !isEligibleValidator(cnt, expectedBlockPerValidator) {
			ineligibleValidators = append(ineligibleValidators, &addressByCnt{validator, cnt})
		}
	}
	return ineligibleValidators, nil
}

// isEligibleValidator check whether the validator is still ok to be an eligible validator in
// the next epoch. The criteria is that the validator produces more than half of the expected
// blocks being mined in the epoch
func isEligibleValidator(gotBlockProduced, expectedBlockProduced int64) bool {
	// Get the addr's mined block
	return gotBlockProduced >= expectedBlockProduced/eligibleValidatorDenominator
}

// getRandomSelectorType get the random selector type should be used
func (ec *EpochContext) getRandomSelectorType(config *params.ChainConfig) int {
	if config.Dpos.IsLuckySpinner(ec.TimeStamp) {
		return typeLuckySpinner
	}
	return typeLuckyWheel
}

// selectValidator select validators randomly based on candidates votes and seed
func selectValidator(candidateVotes randomSelectorEntries, seed int64, typeRandomSelector int, target int) ([]common.Address, error) {
	return randomSelectAddress(typeRandomSelector, candidateVotes, seed, target)
}

// calMaxValidatorsNumbers get the validators number in this epoch after hardfork
func calMaxValidatorsNumbers(blockNumber int64, blockTime int64, state stateDB, config *params.DposConfig) int {
	epoch := CalculateEpochID(blockTime)

	if !config.IsDip8(blockNumber) || epoch <= CalculateAverageForDip8 {
		return MaxValidatorSize
	}

	// After the hardfork
	// 1. Calculate the lastest 14 epochs average candidates numbers
	totalCandidates := common.BigInt0
	for e := epoch - CalculateAverageForDip8; e < epoch; e++ {
		cn := GetCandidatesNumber(state, e)
		totalCandidates = totalCandidates.Add(cn)
	}
	averageCandidates := totalCandidates.DivUint64(CalculateAverageForDip8)
	// 2. Choose different validators number according to the average result
	switch {
	case averageCandidates.CmpUint64(60) < 0:
		return MaxValidatorSizeUnder60
	case averageCandidates.CmpUint64(60) >= 0 && averageCandidates.CmpUint64(90) < 0:
		return MaxValidatorSizeFrom60To90
	case averageCandidates.CmpUint64(90) >= 0:
		return MaxValidatorSizeOver90
	}
	return 0
}

// calSafeSize get the safe validators size after hard fork
func calSafeSize(maxValidatorSize int) int {
	return maxValidatorSize*2/3 + 1
}

// allDelegatorForValidators returns a map containing all delegators who vote for the validators
func allDelegatorForValidators(ctx *types.DposContext, validators []common.Address) map[common.Address]struct{} {
	res := make(map[common.Address]struct{})
	for _, validator := range validators {
		delegators := getAllDelegatorForCandidate(ctx, validator)
		for _, delegator := range delegators {
			res[delegator] = struct{}{}
		}
	}
	return res
}

// lookupValidator returns the validator responsible for producing the block in the curTime.
// If not a valid timestamp, an error is returned
func (ec *EpochContext) lookupValidator(blockTime int64) (validator common.Address, err error) {
	validator = common.Address{}
	slot, err := calcBlockSlot(blockTime)
	if err != nil {
		return common.Address{}, err
	}
	// Get validators and the expected validator
	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return common.Address{}, err
	}
	validatorSize := len(validators)
	if validatorSize == 0 {
		return common.Address{}, errors.New("failed to lookup validator")
	}
	index := slot % int64(validatorSize)
	return validators[index], nil
}

type (
	// addressesByMinedCnt is a sortable address list of address with a count by ascending order
	addressesByCnt []*addressByCnt

	addressByCnt struct {
		address common.Address
		cnt     int64
	}
)

func (a addressesByCnt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a addressesByCnt) Len() int      { return len(a) }
func (a addressesByCnt) Less(i, j int) bool {
	if a[i].cnt != a[j].cnt {
		return a[i].cnt < a[j].cnt
	}
	return a[i].address.String() < a[j].address.String()
}

// makeSeed makes the seed for random selection in try elect
func makeSeed(h common.Hash, i int64) int64 {
	return int64(binary.LittleEndian.Uint32(crypto.Keccak512(h.Bytes()))) + i
}
