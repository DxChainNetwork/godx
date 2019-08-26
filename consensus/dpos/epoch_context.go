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

// EpochContext define current epoch context for dpos consensus
type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	stateDB     *state.StateDB
}

// countVotes will calculate the number of votes at the beginning of current epoch
func (ec *EpochContext) countVotes() (votes map[common.Address]*big.Int, err error) {
	votes = map[common.Address]*big.Int{}
	delegateTrie := ec.DposContext.DelegateTrie()
	candidateTrie := ec.DposContext.CandidateTrie()
	statedb := ec.stateDB

	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	existCandidate := iterCandidate.Next()
	if !existCandidate {
		return votes, errors.New("no candidates")
	}
	for existCandidate {
		candidate := iterCandidate.Value
		candidateAddr := common.BytesToAddress(candidate)
		delegateIterator := trie.NewIterator(delegateTrie.PrefixIterator(candidate))
		existDelegator := delegateIterator.Next()
		if !existDelegator {
			votes[candidateAddr] = new(big.Int)
			existCandidate = iterCandidate.Next()
			continue
		}
		for existDelegator {
			delegator := delegateIterator.Value
			score, ok := votes[candidateAddr]
			if !ok {
				score = new(big.Int)
			}
			delegatorAddr := common.BytesToAddress(delegator)

			// retrieve the vote deposit of delegator
			emptyHash := common.Hash{}
			voteDepositHash := statedb.GetState(delegatorAddr, common.BytesToHash([]byte("vote-deposit")))

			// maybe current is genesis, has no vote before
			if voteDepositHash == emptyHash {
				existDelegator = delegateIterator.Next()
				continue
			}
			voteDeposit := binary.BigEndian.Uint64(voteDepositHash.Bytes())

			// retrieve the real vote weight ratio of delegator
			realVoteWeightRatioHash := statedb.GetState(delegatorAddr, common.BytesToHash([]byte("real-vote-weight-ratio")))

			// maybe current is genesis, has no vote before
			if realVoteWeightRatioHash == emptyHash {
				existDelegator = delegateIterator.Next()
				continue
			}
			realVoteWeightRatio := BytesToFloat64(realVoteWeightRatioHash.Bytes())

			// calculate the real vote weight of delegator
			realVoteWeight := float64(voteDeposit) * realVoteWeightRatio
			score.Add(score, common.NewBigIntFloat64(realVoteWeight).BigIntPtr())

			votes[candidateAddr] = score
			existDelegator = delegateIterator.Next()
		}
		existCandidate = iterCandidate.Next()
	}
	return votes, nil
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

	epochDuration := epochInterval
	// First epoch duration may lt epoch interval,
	// while the first block time wouldn't always align with epoch interval,
	// so caculate the first epoch duartion with first block time instead of epoch interval,
	// prevent the validators were kickout incorrectly.
	if ec.TimeStamp-timeOfFirstBlock < epochInterval {
		epochDuration = ec.TimeStamp - timeOfFirstBlock
	}

	needKickoutValidators := sortableAddresses{}
	for _, validator := range validators {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(epoch))
		key = append(key, validator.Bytes()...)
		cnt := int64(0)
		if cntBytes := ec.DposContext.MintCntTrie().Get(key); cntBytes != nil {
			cnt = int64(binary.BigEndian.Uint64(cntBytes))
		}
		if cnt < epochDuration/blockInterval/maxValidatorSize/2 {
			// not active validators need kickout
			needKickoutValidators = append(needKickoutValidators, &sortableAddress{validator, big.NewInt(cnt)})
		}
	}
	// no validators need kickout
	needKickoutValidatorCnt := len(needKickoutValidators)
	if needKickoutValidatorCnt <= 0 {
		return nil
	}
	sort.Sort(sort.Reverse(needKickoutValidators))

	candidateCount := 0
	iter := trie.NewIterator(ec.DposContext.CandidateTrie().NodeIterator(nil))
	for iter.Next() {
		candidateCount++
		if candidateCount >= needKickoutValidatorCnt+safeSize {
			break
		}
	}

	for i, validator := range needKickoutValidators {
		// ensure candidate count greater than or equal to safeSize
		if candidateCount <= safeSize {
			log.Info("No more candidate can be kickout", "prevEpochID", epoch, "candidateCount", candidateCount, "needKickoutCount", len(needKickoutValidators)-i)
			return nil
		}

		if err := ec.DposContext.KickoutCandidate(validator.address); err != nil {
			return err
		}
		// if kickout success, candidateCount minus 1
		candidateCount--
		log.Info("Kickout candidate", "prevEpochID", epoch, "candidate", validator.address.String(), "mintCnt", validator.weight.String())
	}
	return nil
}

// lookupValidator try to find a validator at the time of now in current epoch
func (ec *EpochContext) lookupValidator(now int64) (validator common.Address, err error) {
	validator = common.Address{}
	offset := now % epochInterval
	if offset%blockInterval != 0 {
		return common.Address{}, ErrInvalidMintBlockTime
	}
	offset /= blockInterval

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
	genesisEpoch := genesis.Time.Int64() / epochInterval
	prevEpoch := parent.Time.Int64() / epochInterval
	currentEpoch := ec.TimeStamp / epochInterval

	// TODO: 有可能既是candidate，又给他人或者自己投票，所以还需要解冻candidate质押

	// iterator whole thawing account trie, and thawing the deposit of every delegator
	epochIDStr := strconv.FormatInt(currentEpoch-2, 10)
	thawingAddress := common.BytesToAddress([]byte("thawing_" + epochIDStr))
	ec.stateDB.Exist(thawingAddress)
	thawingTrie := ec.stateDB.StorageTrie(thawingAddress)
	it := trie.NewIterator(thawingTrie.NodeIterator(nil))
	for it.Next() {
		delegator := common.BytesToAddress(it.Key)
		ec.stateDB.SetState(delegator, common.BytesToHash([]byte("vote-deposit")), common.Hash{})
	}

	prevEpochIsGenesis := prevEpoch == genesisEpoch
	if prevEpochIsGenesis && prevEpoch < currentEpoch {
		prevEpoch = currentEpoch - 1
	}

	prevEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(prevEpochBytes, uint64(prevEpoch))
	iter := trie.NewIterator(ec.DposContext.MintCntTrie().PrefixIterator(prevEpochBytes))
	for i := prevEpoch; i < currentEpoch; i++ {
		// if prevEpoch is not genesis, kickout not active candidate
		if !prevEpochIsGenesis && iter.Next() {
			if err := ec.kickoutValidators(prevEpoch); err != nil {
				return err
			}
		}

		// calculate the actual result of the vote based on the attenuation
		votes, err := ec.countVotes()
		if err != nil {
			return err
		}

		// maybe current is genesis, so has no vote on chain, just return
		if len(votes) == 0 {
			return nil
		}

		// calculate the vote weight proportion based on the vote weight
		totalScores := new(big.Int).SetInt64(0)
		voteProportions := sortableVoteProportions{}
		for _, score := range votes {
			totalScores.Add(totalScores, score)
		}

		for candidateAddr, score := range votes {
			voteProportion := &sortableVoteProportion{
				address:    candidateAddr,
				proportion: float64(score.Int64()) / float64(totalScores.Int64()),
			}
			voteProportions = append(voteProportions, voteProportion)
		}

		// sort by asc
		sort.Sort(voteProportions)
		if len(voteProportions) < safeSize {
			return errors.New("too few candidates")
		}

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

type sortableAddress struct {
	address common.Address
	weight  *big.Int
}
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

func BytesToFloat64(bytes []byte) float64 {
	bits := binary.BigEndian.Uint64(bytes)
	return math.Float64frombits(bits)
}

// LuckyTurntable elects some validators with random seed
func LuckyTurntable(voteProportions sortableVoteProportions, seed int64) []common.Address {

	// if total candidates less than maxValidatorSize, directly return all
	if len(voteProportions) <= maxValidatorSize {
		return voteProportions.ListAddresses()
	}

	// if total candidates more than maxValidatorSize, election from them
	result := make([]common.Address, 0)
	r := rand.New(rand.NewSource(seed))
	for j := 0; j < maxValidatorSize; j++ {
		selection := r.Float64()
		sumProp := float64(0)
		for i, _ := range voteProportions {
			sumProp += voteProportions[i].proportion
			if selection <= sumProp {

				// Lucky one
				result = append(result, voteProportions[i].address)

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
