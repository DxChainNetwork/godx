// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/pkg/errors"
)

type opType int

const (
	opTypeDoNothing opType = iota
	opTypeAddCandidate
	opTypeCancelCandidate
	opTypeVoteIncreaseDeposit
	opTypeVoteDecreaseDeposit
	opTypeCancelVote
)

var (
	r = rand.New(rand.NewSource(time.Now().UnixNano()))

	balance = minDeposit.MultInt64(10)
)

type (
	testEpochContext struct {
		curTime int64
		stateDB *state.StateDB
		ctx     *types.DposContext
		ec      *expectContext
	}

	expectContext struct {
		// All available users
		userRecords userRecords
		// expected candidates
		candidateRecords          candidateRecords
		candidateRecordsLastEpoch candidateRecords
		delegatorRecords          delegatorRecords
		delegatorRecordsLastEpoch delegatorRecords
		thawing                   map[int64]map[common.Address]common.BigInt
		frozenAssets              map[common.Address]common.BigInt
		balance                   map[common.Address]common.BigInt
	}

	candidateRecords map[common.Address]candidateRecord

	candidateRecord struct {
		deposit     common.BigInt
		rewardRatio uint64
		votes       map[common.Address]struct{}
	}

	delegatorRecords map[common.Address]delegatorRecord

	delegatorRecord struct {
		deposit common.BigInt
		votes   map[common.Address]struct{}
	}

	userRecords map[common.Address]userRecord

	userRecord struct {
		addr        common.Address
		isCandidate bool
		isDelegator bool
	}
)

func TestDPOSIntegration(t *testing.T) {

}

func NewTestEpochContext(num int) (*testEpochContext, error) {
	curTime := time.Now().Unix() / BlockInterval * BlockInterval
	stateDB, ctx, err := newStateAndDposContext()
	if err != nil {
		return nil, err
	}
	tec := &testEpochContext{
		curTime: curTime,
		stateDB: stateDB,
		ctx:     ctx,
		ec: &expectContext{
			userRecords:               makeUsers(num),
			candidateRecords:          make(candidateRecords),
			candidateRecordsLastEpoch: make(candidateRecords),
			delegatorRecords:          make(delegatorRecords),
			delegatorRecordsLastEpoch: make(delegatorRecords),
			thawing:                   make(map[int64]map[common.Address]common.BigInt),
			frozenAssets:              make(map[common.Address]common.BigInt),
			balance:                   make(map[common.Address]common.BigInt),
		},
	}
	// Add balance for all users
	for addr := range tec.ec.userRecords {
		addAccountInState(stateDB, addr, minDeposit.MultInt64(100), common.BigInt0)
		tec.ec.balance[addr] = minDeposit.MultInt64(100)
	}
	// initial validators
	if err := tec.addInitialValidator(MaxValidatorSize); err != nil {
		return nil, err
	}
	return tec, nil
}

func (tec *testEpochContext) addInitialValidator(num int) error {
	validators := make([]common.Address, 0, num)
	var added int
	for addr, user := range tec.ec.userRecords {
		if !user.isCandidate {
			continue
		}
		validators = append(validators, addr)
		added++
		if added == num {
			break
		}
	}
	for _, v := range validators {
		if err := ProcessAddCandidate(tec.stateDB, tec.ctx, v, minDeposit, uint64(50)); err != nil {
			return err
		}
		tec.ec.candidateRecords[v] = candidateRecord{
			deposit:     minDeposit,
			rewardRatio: uint64(50),
			votes:       make(map[common.Address]struct{}),
		}
		tec.ec.frozenAssets[v] = minDeposit
	}
	if err := tec.ctx.SetValidators(validators); err != nil {
		return err
	}
	return nil
}

func makeUsers(num int) userRecords {
	records := make(userRecords)
	for i := 0; i != num; i++ {
		record := makeUserRecord()
		records[record.addr] = record
	}
	return records
}

func makeUserRecord() userRecord {
	// at ratio 4/5, the user will become a delegator
	n := r.Intn(5)
	var isDelegator bool
	if n != 0 {
		isDelegator = true
	}
	// at ratio 1/3, the user will become a delegator
	n = r.Intn(3)
	var isCandidate bool
	if n == 0 {
		isCandidate = true
	}
	return userRecord{
		addr:        randomAddress(),
		isCandidate: isCandidate,
		isDelegator: isDelegator,
	}
}

func (tec *testEpochContext) executeRandomOperation() error {
	var randomUser userRecord
	for _, randomUser = range tec.ec.userRecords {
		break
	}
	addr := randomUser.addr
	opType := randomUser.randomOp(tec.ec)
	switch opType {
	case opTypeAddCandidate:
		return executeTestAddCandidate(tec, addr)
	case opTypeCancelCandidate:
		return executeTestCancelCandidate(tec, addr)
	case opTypeVoteIncreaseDeposit:
		return executeTestVoteIncreaseDeposit(tec, addr)
	case opTypeVoteDecreaseDeposit:
		return executeTestVoteDecreaseDeposit(tec, addr)
	case opTypeCancelVote:
		return executeTestCancelVote(tec, addr)
	case opTypeDoNothing:
		return nil
	default:
	}
	return errors.New("unknown op type")
}

func (user *userRecord) randomOp(ec *expectContext) opType {
	ops := user.availOpList(ec)
	return ops[r.Intn(len(ops))]
}

func (user *userRecord) availOpList(ec *expectContext) []opType {
	ops := make([]opType, 0, 6)
	ops = append(ops, opTypeDoNothing)
	if user.isCandidate {
		ops = append(ops, opTypeAddCandidate)
		if _, exist := ec.candidateRecords[user.addr]; exist {
			ops = append(ops, opTypeCancelCandidate)
		}
	}
	if user.isDelegator {
		ops = append(ops, opTypeVoteIncreaseDeposit)
		if _, exist := ec.delegatorRecords[user.addr]; exist {
			ops = append(ops, opTypeVoteDecreaseDeposit, opTypeCancelVote)
		} else {
			ops = append(ops)
		}
	}
	return ops
}

func executeTestAddCandidate(tec *testEpochContext, addr common.Address) error {
	// The new deposit is defined as previous deposit plus 1/10 of the remaining balance
	prevCandidateRecord, exist := tec.ec.candidateRecords[addr]
	prevDeposit, prevRewardRatio, prevVotes := common.BigInt0, uint64(0), make(map[common.Address]struct{})
	if exist {
		prevDeposit, prevRewardRatio = prevCandidateRecord.deposit, prevCandidateRecord.rewardRatio
		prevVotes = prevCandidateRecord.votes
	}
	newDeposit := prevDeposit.Add(getAvailableBalance(tec.stateDB, addr).DivUint64(10))
	newRewardRatio := (RewardRatioDenominator-prevRewardRatio)/4 + prevRewardRatio
	// Process Add candidate
	if err := ProcessAddCandidate(tec.stateDB, tec.ctx, addr, newDeposit, newRewardRatio); err != nil {
		return err
	}
	// Update the expected result
	tec.ec.addCandidate(addr, newDeposit, newRewardRatio, prevVotes)
	tec.ec.addFrozenAssets(addr, newDeposit.Sub(newDeposit.Sub(prevDeposit)))
	return nil
}

func executeTestCancelCandidate(tec *testEpochContext, addr common.Address) error {
	prevCandidateRecord, exist := tec.ec.candidateRecords[addr]
	if !exist {
		return fmt.Errorf("address %x not previously in candidateRecords", addr)
	}
	if err := ProcessCancelCandidate(tec.stateDB, tec.ctx, addr, tec.curTime); err != nil {
		return err
	}
	// Update the expected result
	tec.ec.deleteCandidate(addr)
	tec.ec.addThawing(addr, prevCandidateRecord.deposit, tec.curTime)
	return nil
}

func executeTestVoteIncreaseDeposit(tec *testEpochContext, addr common.Address) error {
	// get the previous info
	prevVoteRecord, exist := tec.ec.delegatorRecords[addr]
	prevDeposit := common.BigInt0
	if exist {
		prevDeposit = prevVoteRecord.deposit
	}
	// Create the params for the new vote transaction.
	newDeposit := prevDeposit.Add(getAvailableBalance(tec.stateDB, addr).DivUint64(100))
	votes := randomPickCandidates(tec.ec.candidateRecords, 30)
	if _, err := ProcessVote(tec.stateDB, tec.ctx, addr, newDeposit, votes, tec.curTime); err != nil {
		return err
	}
	// Update expected context
	tec.ec.deleteDelegateVotes(addr)
	tec.ec.addDelegateVotes(addr, newDeposit, votes)
	tec.ec.addFrozenAssets(addr, newDeposit.Sub(prevDeposit))
	return nil
}

func executeTestVoteDecreaseDeposit(tec *testEpochContext, addr common.Address) error {
	// Get the previous info
	prevVoteRecord, exist := tec.ec.delegatorRecords[addr]
	if !exist {
		return errors.New("when decreasing vote deposit, entry not exist in delegator records")
	}
	prevDeposit := prevVoteRecord.deposit

	// Create params for the new params and vote
	newDeposit := prevDeposit.MultInt64(2).DivUint64(3)
	votes := randomPickCandidates(tec.ec.candidateRecords, 30)
	if _, err := ProcessVote(tec.stateDB, tec.ctx, addr, newDeposit, votes, tec.curTime); err != nil {
		return err
	}
	// Update expected context
	tec.ec.deleteDelegateVotes(addr)
	tec.ec.addDelegateVotes(addr, newDeposit, votes)
	tec.ec.addThawing(addr, newDeposit.Sub(prevDeposit), tec.curTime)
	return nil
}

func executeTestCancelVote(tec *testEpochContext, addr common.Address) error {
	prevVoteRecord, exist := tec.ec.delegatorRecords[addr]
	if !exist {
		return errors.New("vote record previously not in record map")
	}
	if err := ProcessCancelVote(tec.stateDB, tec.ctx, addr, tec.curTime); err != nil {
		return err
	}
	prevDeposit := prevVoteRecord.deposit
	// Write to expected context. Delete candidate votes and add thawing
	tec.ec.deleteDelegateVotes(addr)
	tec.ec.addThawing(addr, prevDeposit, tec.curTime)
	return nil
}

func (ec *expectContext) addCandidate(candidate common.Address, deposit common.BigInt, rewardRatio uint64,
	prevVotes map[common.Address]struct{}) {

	ec.candidateRecords[candidate] = candidateRecord{
		deposit:     deposit,
		rewardRatio: rewardRatio,
		votes:       prevVotes,
	}
}

func (ec *expectContext) deleteCandidate(candidate common.Address) {
	record, exist := ec.candidateRecords[candidate]
	if !exist {
		return
	}
	for c := range record.votes {
		delete(ec.delegatorRecords[c].votes, candidate)
	}
	delete(ec.candidateRecords, candidate)
}

func (ec *expectContext) addDelegateVotes(delegator common.Address, deposit common.BigInt, votes []common.Address) {
	voteMap := make(map[common.Address]struct{})
	for _, v := range votes {
		ec.candidateRecords[v].votes[delegator] = struct{}{}
		voteMap[v] = struct{}{}
	}
	ec.delegatorRecords[delegator] = delegatorRecord{
		deposit: deposit,
		votes:   voteMap,
	}
}

// delegatorCancelVotes is the action of a delegator cancel the votes
func (ec *expectContext) deleteDelegateVotes(delegator common.Address) {
	record, exist := ec.delegatorRecords[delegator]
	if !exist {
		return
	}
	for v := range record.votes {
		delete(ec.candidateRecords[v].votes, delegator)
	}
	delete(ec.delegatorRecords, delegator)
}

func (ec *expectContext) addThawing(addr common.Address, diff common.BigInt, curTime int64) {
	thawEpoch := calcThawingEpoch(CalculateEpochID(curTime))
	prevThawing, exist := ec.thawing[thawEpoch][addr]
	if !exist {
		prevThawing = common.BigInt0
	}
	ec.thawing[thawEpoch][addr] = prevThawing.Add(diff)
}

func (ec *expectContext) addFrozenAssets(addr common.Address, diff common.BigInt) {
	prevFrozenAssets, exist := ec.frozenAssets[addr]
	if !exist {
		prevFrozenAssets = common.BigInt0
	}
	newFrozenAssets := prevFrozenAssets.Add(diff)
	ec.frozenAssets[addr] = newFrozenAssets
}

func randomPickCandidates(candidates candidateRecords, num int) []common.Address {
	votes := make([]common.Address, 0, num)
	added := 0
	for candidate := range candidates {
		votes = append(votes, candidate)
		added++
		if added >= num {
			return votes
		}
	}
	return votes
}
