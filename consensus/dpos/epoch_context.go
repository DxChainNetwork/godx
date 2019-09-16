// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/trie"
)

const (

	// ThawingEpochDuration defines that if user cancel candidate or vote, the deposit will be thawed after 2 epochs
	ThawingEpochDuration = 2

	// eigibleValidatorDenominator defines the denominator of the minimum expected block. If a validator
	// produces block less than expected by this denominator, it is considered as ineligible.
	eigibleValidatorDenominator = 2
)

// EpochContext define current epoch context for dpos consensus
type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	stateDB     stateDB
}

// tryElect will process election at the beginning of current epoch
func (ec *EpochContext) tryElect(genesis, parent *types.Header) error {
	genesisEpoch := CalculateEpochID(genesis.Time.Int64())
	prevEpoch := CalculateEpochID(parent.Time.Int64())
	currentEpoch := CalculateEpochID(ec.TimeStamp)

	// if current block does not reach new epoch, directly return
	if prevEpoch == currentEpoch {
		return nil
	}

	// thawing some deposit for currentEpoch-2
	ThawingDeposit(ec.stateDB, currentEpoch)

	prevEpochIsGenesis := prevEpoch == genesisEpoch
	if prevEpochIsGenesis && prevEpoch < currentEpoch {
		prevEpoch = currentEpoch - 1
	}

	prevEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(prevEpochBytes, uint64(prevEpoch))
	iter := trie.NewIterator(ec.DposContext.MinedCntTrie().PrefixIterator(prevEpochBytes))

	// do election from prevEpoch to currentEpoch
	for i := prevEpoch; i < currentEpoch; i++ {
		// if prevEpoch is not genesis, kickout not active candidate
		if !prevEpochIsGenesis && iter.Next() {
			if err := ec.kickoutValidators(prevEpoch); err != nil {
				return err
			}
		}

		// calculate the actual result of the vote based on the attenuation
		candidateVotes, err := ec.countVotes()
		if err != nil {
			return err
		}

		// check if number of candidates is smaller than safe size
		if len(candidateVotes) < SafeSize {
			return errors.New("too few candidates")
		}

		// calculate the vote weight proportion based on the vote weight
		totalVotes := common.BigInt0
		voteProportions := sortableVoteProportions{}
		for _, vote := range candidateVotes {
			totalVotes = totalVotes.Add(vote)
		}

		for candidateAddr, vote := range candidateVotes {
			voteProportion := &sortableVoteProportion{
				address:    candidateAddr,
				proportion: vote.Float64() / totalVotes.Float64(),
			}
			voteProportions = append(voteProportions, voteProportion)
		}

		// sort by asc
		sort.Sort(voteProportions)

		// make random seed
		seed := int64(binary.LittleEndian.Uint32(crypto.Keccak512(parent.Hash().Bytes()))) + i
		r := rand.New(rand.NewSource(seed))

		// Lucky Turntable election
		result := LuckyTurntable(voteProportions, seed)

		// shuffle candidates
		for i := len(result) - 1; i > 0; i-- {
			j := int(r.Int31n(int32(i + 1)))
			result[i], result[j] = result[j], result[i]
		}

		epochTrie, _ := types.NewEpochTrie(common.Hash{}, ec.DposContext.DB())
		ec.DposContext.SetEpoch(epochTrie)
		err = ec.DposContext.SetValidators(result)
		if err != nil {
			return err
		}
		log.Info("Come to new epoch", "prevEpoch", i, "nextEpoch", i+1)
	}
	return nil
}

// countVotes will calculate the number of votes at the beginning of current epoch
func (ec *EpochContext) countVotes() (votes map[common.Address]common.BigInt, err error) {
	// get the needed variables
	votes = make(map[common.Address]common.BigInt)
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
		if _, ok := votes[candidateAddr]; ok {
			return nil, fmt.Errorf("countVotes failed, get same candidates from the candidate trie: %v", candidateAddr)
		}
		// Calculate the candidate votes
		totalVotes := ec.calcCandidateTotalVotes(candidateAddr)
		// write the totalVotes to result and state
		votes[candidateAddr] = totalVotes
		setTotalVote(statedb, candidateAddr, totalVotes)
	}
	// if there are no candidates, return error
	if !hasCandidate {
		return votes, fmt.Errorf("countVotes failed, no candidates available")
	}
	return votes, nil
}

// kickoutValidators will kick out irresponsible validators of last epoch at the beginning of current epoch
func (ec *EpochContext) kickoutValidators(epoch int64) error {
	needKickoutValidators, err := getIneligibleValidators(ec.DposContext, epoch, ec.TimeStamp)
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
	// count candidates to a safe size as a threshold to remove candidate
	candidateCount := 0
	iter := trie.NewIterator(ec.DposContext.CandidateTrie().NodeIterator(nil))
	for iter.Next() {
		candidateCount++
		if candidateCount >= needKickoutValidatorCnt+SafeSize {
			break
		}
	}
	// Loop over the first part of the needKickOutValidators to kick out
	for i, validator := range needKickoutValidators {
		// ensure candidate count greater than or equal to safeSize
		if candidateCount <= SafeSize {
			log.Info("No more candidate can be kickout", "prevEpochID", epoch, "candidateCount", candidateCount, "needKickoutCount", len(needKickoutValidators)-i)
			return nil
		}
		if err := ec.DposContext.KickoutCandidate(validator.address); err != nil {
			return err
		}
		// if successfully above, then mark the validator that will be thawed in next next epoch
		currentEpochID := CalculateEpochID(ec.TimeStamp)
		MarkThawingAddress(ec.stateDB, validator.address, currentEpochID, PrefixCandidateThawing)
		// if kickout success, candidateCount minus 1
		candidateCount--
		log.Info("Kickout candidate", "prevEpochID", epoch, "candidate", validator.address.String(), "minedCnt", validator.cnt)
	}
	return nil
}

// getIneligibleValidators return the ineligible validators in a certain epoch. An ineligible validator is
// defined as a validator who produced blocks less than half as expected
func getIneligibleValidators(ctx *types.DposContext, epoch int64, curTime int64) (addressesByCnt, error) {
	validators, err := ctx.GetValidators()
	if err != nil {
		return addressesByCnt{}, fmt.Errorf("failed to get validator: %s", err)
	}
	if len(validators) == 0 {
		return addressesByCnt{}, errors.New("no validators")
	}
	expectedBlockPerValidator := expectedBlocksPerValidatorInEpoch(timeOfFirstBlock, curTime)
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
	return gotBlockProduced >= expectedBlockProduced/eigibleValidatorDenominator
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

// LuckyTurntable elects some validators with random seed
func LuckyTurntable(voteProportions sortableVoteProportions, seed int64) []common.Address {

	// if total candidates less than maxValidatorSize, directly return all
	if len(voteProportions) <= MaxValidatorSize {
		return voteProportions.ListAddresses()
	}

	// if total candidates more than maxValidatorSize, election from them
	result := make([]common.Address, 0)
	r := rand.New(rand.NewSource(seed))
	for j := 0; j < MaxValidatorSize; j++ {
		selection := r.Float64()
		sumProp := float64(0)
		for i := range voteProportions {
			sumProp += voteProportions[i].proportion
			if selection <= sumProp {

				// Lucky one
				result = append(result, voteProportions[i].address)

				// NOTE: It's indeed that we should remove the elected one, because maybe there are some same vote proportion.
				// When removed one of the same vote proportion, the left can be elected with greater probability.
				// remove the elected one from current candidate list
				preVoteProportions := voteProportions[:i]
				sufVoteProportions := sortableVoteProportions{}
				if i != len(voteProportions)-1 {
					sufVoteProportions = voteProportions[(i + 1):]
				}
				voteProportions = sortableVoteProportions{}
				voteProportions = append(voteProportions, preVoteProportions[:]...)
				if len(sufVoteProportions) > 0 {
					voteProportions = append(voteProportions, sufVoteProportions[:]...)
				}

				// calculate the vote weight proportion of the left candidate
				totalPros := float64(0)
				for _, pro := range voteProportions {
					totalPros += pro.proportion
				}

				for _, pro := range voteProportions {
					pro.proportion = pro.proportion / totalPros
				}

				// sort by asc
				sort.Sort(voteProportions)
				break
			}
		}
	}
	return result
}

type sortableVoteProportion struct {
	address    common.Address
	proportion float64
}

// Ascending, from small to big
type sortableVoteProportions []*sortableVoteProportion

func (p sortableVoteProportions) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p sortableVoteProportions) Len() int      { return len(p) }
func (p sortableVoteProportions) Less(i, j int) bool {
	if p[i].proportion < p[j].proportion {
		return true
	} else if p[i].proportion > p[j].proportion {
		return false
	} else {
		return p[i].address.String() < p[j].address.String()
	}
}

func (p sortableVoteProportions) ListAddresses() []common.Address {
	result := make([]common.Address, 0)
	for _, votePro := range p {
		result = append(result, votePro.address)
	}
	return result
}
