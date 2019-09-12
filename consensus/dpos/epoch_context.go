// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/trie"
)

const (

	// ThawingEpochDuration defines that if user cancel candidate or vote, the deposit will be thawed after 2 epochs
	ThawingEpochDuration = 2
)

// EpochContext define current epoch context for dpos consensus
type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	stateDB     *state.StateDB
}

// countVotes will calculate the number of votes at the beginning of current epoch
func (ec *EpochContext) countVotes() (votes map[common.Address]*big.Int, err error) {
	// get the needed variables
	votes = map[common.Address]*big.Int{}
	delegateTrie := ec.DposContext.DelegateTrie()
	candidateTrie := ec.DposContext.CandidateTrie()
	statedb := ec.stateDB

	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	var hasCandidate bool

	// loop through all candidates and calculate total votes for each of them
	for iterCandidate.Next() {
		// get and initialize all variables
		hasCandidate = true
		candidateAddrBytes := iterCandidate.Value
		candidateAddr := common.BytesToAddress(candidateAddrBytes)

		// sanity check
		if _, ok := votes[candidateAddr]; ok {
			return nil, fmt.Errorf("countVotes failed, get same candidates from the candidate trie: %v", candidateAddr)
		}

		// get the candidateDepositVotes
		candidateVotes := candidateDepositVotes(candidateAddr, statedb)
		delegatorVotes := delegatorDepositVotes(candidateAddrBytes, statedb, delegateTrie)

		// get the total votes, and
		totalVotes := new(big.Int).Add(candidateVotes, delegatorVotes)
		votes[candidateAddr] = totalVotes
		ec.stateDB.SetState(candidateAddr, KeyTotalVoteWeight, common.BigToHash(votes[candidateAddr]))
	}

	// if there are no candidates, return error
	if !hasCandidate {
		return votes, fmt.Errorf("countVotes failed, no candidates available")
	}

	return votes, nil
}

func candidateDepositVotes(candidateAddr common.Address, statedb *state.StateDB) *big.Int {
	// retrieve the candidate deposit votes
	candidateDepositVotes := new(big.Int).SetInt64(0)
	candidateDepositHash := statedb.GetState(candidateAddr, KeyCandidateDeposit)
	if candidateDepositHash != EmptyHash {
		candidateDepositVotes = candidateDepositHash.Big()
	}

	// return the candidateDepositVotes
	return candidateDepositVotes
}

func delegatorDepositVotes(candidateAddrBytes []byte, statedb *state.StateDB, delegateTrie *trie.Trie) *big.Int {
	delegateIterator := trie.NewIterator(delegateTrie.PrefixIterator(candidateAddrBytes))
	var delegatorVotes = common.Big0

	// loop through each delegator, get all votes
	for delegateIterator.Next() {
		delegatorAddr := common.BytesToAddress(delegateIterator.Value)
		voteDepositHash := statedb.GetState(delegatorAddr, KeyVoteDeposit)
		if voteDepositHash == EmptyHash {
			continue
		}
		voteDeposit := voteDepositHash.Big()

		// get the realVoteDeposit, after applied voteWeight
		realVoteDeposit, exists := getAndApplyVoteWeight(voteDeposit, statedb, delegatorAddr)
		if !exists {
			continue
		}

		// add the realVoteDeposit
		delegatorVotes.Add(delegatorVotes, realVoteDeposit)
	}

	return delegatorVotes
}

func getAndApplyVoteWeight(voteDeposit *big.Int, statedb *state.StateDB, delegatorAddr common.Address) (*big.Int, bool) {
	// get the voteWeight
	voteWeightHash := statedb.GetState(delegatorAddr, KeyRealVoteWeightRatio)
	if voteWeightHash == EmptyHash {
		return nil, false
	}
	voteWeight := BytesToFloat64(voteWeightHash.Bytes()[24:])

	// apply vote weight adn return the real vote deposit
	realVoteDeposit := common.PtrBigInt(voteDeposit).MultFloat64(voteWeight)
	return realVoteDeposit.BigIntPtr(), true
}

// kickoutValidators will kick out irresponsible validators of last epoch at the beginning of current epoch
func (ec *EpochContext) kickoutValidators(epoch int64) error {
	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return fmt.Errorf("failed to get validator: %s", err)
	}
	if len(validators) == 0 {
		return errors.New("no validator could be kickout")
	}

	epochDuration := EpochInterval
	// First epoch duration may lt epoch interval,
	// while the first block time wouldn't always align with epoch interval,
	// so calculate the first epoch duration with first block time instead of epoch interval,
	// prevent the validators were kickout incorrectly.
	if ec.TimeStamp-timeOfFirstBlock < EpochInterval {
		epochDuration = ec.TimeStamp - timeOfFirstBlock
	}

	needKickoutValidators := sortableAddresses{}
	for _, validator := range validators {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(epoch))
		key = append(key, validator.Bytes()...)
		cnt := int64(0)
		if cntBytes := ec.DposContext.MinedCntTrie().Get(key); cntBytes != nil {
			cnt = int64(binary.BigEndian.Uint64(cntBytes))
		}
		if cnt < epochDuration/BlockInterval/MaxValidatorSize/2 {
			// not active validators need kickout
			needKickoutValidators = append(needKickoutValidators, &sortableAddress{validator, big.NewInt(cnt)})
		}
	}

	// no validators need kickout
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
		log.Info("Kickout candidate", "prevEpochID", epoch, "candidate", validator.address.String(), "minedCnt", validator.weight.String())
	}
	return nil
}

// lookupValidator try to find a validator at the time of now in current epoch
func (ec *EpochContext) lookupValidator(now int64) (validator common.Address, err error) {
	validator = common.Address{}
	offset := now % EpochInterval
	if offset%BlockInterval != 0 {
		return common.Address{}, ErrInvalidMinedBlockTime
	}
	offset /= BlockInterval

	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return common.Address{}, err
	}
	validatorSize := len(validators)
	if validatorSize == 0 {
		return common.Address{}, errors.New("failed to lookup validator")
	}
	offset %= int64(validatorSize)
	return validators[offset], nil
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
		totalVotes := new(big.Int).SetInt64(0)
		voteProportions := sortableVoteProportions{}
		for _, vote := range candidateVotes {
			totalVotes.Add(totalVotes, vote)
		}

		for candidateAddr, vote := range candidateVotes {
			voteProportion := &sortableVoteProportion{
				address:    candidateAddr,
				proportion: float64(vote.Int64()) / float64(totalVotes.Int64()),
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

// MarkThawingAddress mark the given addr that will be thawed in next next epoch
func MarkThawingAddress(stateDB *state.StateDB, addr common.Address, currentEpochID int64, keyPrefix string) {

	// create thawing address: "thawing_" + currentEpochID
	epochIDStr := strconv.FormatInt(currentEpochID, 10)
	thawingAddress := common.BytesToAddress([]byte(PrefixThawingAddr + epochIDStr))
	if !stateDB.Exist(thawingAddress) {
		stateDB.CreateAccount(thawingAddress)

		// before thawing deposit, mark thawingAddress as not empty account to avoid being deleted by stateDB
		stateDB.SetNonce(thawingAddress, 1)
	}

	// set thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	keyAndValue := append([]byte(keyPrefix), addr.Bytes()...)
	stateDB.SetState(thawingAddress, common.BytesToHash(keyAndValue), common.BytesToHash(keyAndValue))
}

// ThawingDeposit thawing the deposit for the candidate or delegator cancel in currentEpoch-2
func ThawingDeposit(stateDB *state.StateDB, currentEpoch int64) {
	epochIDStr := strconv.FormatInt(currentEpoch-ThawingEpochDuration, 10)
	thawingAddress := common.BytesToAddress([]byte(PrefixThawingAddr + epochIDStr))
	if stateDB.Exist(thawingAddress) {

		// iterator whole thawing account trie, and thawing the deposit of every delegator
		stateDB.ForEachStorage(thawingAddress, func(key, value common.Hash) bool {
			if value == (common.Hash{}) {
				return false
			}

			addr := common.BytesToAddress(value.Bytes()[12:])

			// if candidate deposit thawing flag exists, then thawing it
			candidateThawingKey := append([]byte(PrefixCandidateThawing), addr.Bytes()...)
			if common.BytesToHash(candidateThawingKey) == value {

				// set 0 for candidate deposit
				stateDB.SetState(addr, KeyCandidateDeposit, common.Hash{})
			}

			// if vote deposit thawing flag exists, then thawing it
			voteThawingKey := append([]byte(PrefixVoteThawing), addr.Bytes()...)
			if common.BytesToHash(voteThawingKey) == value {

				// set 0 for vote deposit
				stateDB.SetState(addr, KeyVoteDeposit, common.Hash{})
			}

			// candidate or vote deposit does not allow to submit repeatedly, so thawing directly set 0
			stateDB.SetState(thawingAddress, key, common.Hash{})
			return true
		})

		// mark the thawingAddress as empty account, that will be deleted by stateDB
		stateDB.SetNonce(thawingAddress, 0)
	}
}

type sortableAddress struct {
	address common.Address
	weight  *big.Int
}

// ascending, from small to big
type sortableAddresses []*sortableAddress

func (p sortableAddresses) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p sortableAddresses) Len() int      { return len(p) }
func (p sortableAddresses) Less(i, j int) bool {
	if p[i].weight.Cmp(p[j].weight) < 0 {
		return false
	} else if p[i].weight.Cmp(p[j].weight) > 0 {
		return true
	} else {
		return p[i].address.String() < p[j].address.String()
	}
}

// BytesToFloat64 converts []byte to float64
func BytesToFloat64(bytes []byte) float64 {
	bits := binary.BigEndian.Uint64(bytes)
	return math.Float64frombits(bits)
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
				totoalPros := float64(0)
				for _, pro := range voteProportions {
					totoalPros += pro.proportion
				}

				for _, pro := range voteProportions {
					pro.proportion = pro.proportion / totoalPros
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

// descending, from big to small
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
