// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
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

	errNoEntriesInDelegateTrie = errors.New("no entries in delegate trie")

	maxNumValidators = 21

	maxVotes = 30

	emptyAddress common.Address

	emptyHash common.Hash
)

type (
	genesisConfig struct {
		config *params.ChainConfig
		alloc  genesisAlloc
	}

	genesisAlloc map[common.Address]common.BigInt

	testEpochContext struct {
		genesis *types.Header
		curTime int64
		stateDB *state.StateDB
		ctx     *types.DposContext
		ec      *expectContext
	}

	expectContext struct {
		// All available users
		userRecords userRecords

		// expected fields
		validators                []common.Address
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

func TestDposIntegrationInitializeGenesis(t *testing.T) {
	genesis := getGenesis()
	tec, err := newTestEpochContext(1000, genesis)
	if err != nil {
		t.Fatal(err)
	}
	if err = tec.checkConsistency(); err != nil {
		t.Fatal(err)
	}
}

func TestDposIntegrationCandidate(t *testing.T) {
	genesis := getGenesis()
	tec, err := newTestEpochContext(1000, genesis)
	if err != nil {
		t.Fatal(err)
	}
	newCandidate := tec.ec.pickPotentialCandidate().addr
	if newCandidate == emptyAddress {
		t.Fatal("no potential candidate found")
	}
	if err = tec.executeAddCandidate(newCandidate); err != nil {
		t.Fatal(err)
	}
	existCandidate, _ := tec.ec.pickCandidate()
	if existCandidate == emptyAddress {
		t.Fatal("no candidate found")
	}
	if err = tec.executeAddCandidate(existCandidate); err != nil {
		t.Fatal(err)
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
	// Cancel a candidate
	existCandidate, _ = tec.ec.pickCandidate()
	if existCandidate == emptyAddress {
		t.Fatal("no candidate found")
	}
	if err = tec.executeCancelCandidate(existCandidate); err != nil {
		t.Fatal(err)
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
}

func TestDposIntegrationVote(t *testing.T) {
	genesis := getGenesis()
	tec, err := newTestEpochContext(1000, genesis)
	if err != nil {
		t.Fatal(err)
	}
	// First case, a new delegator with 21 votes
	newDelegator := tec.ec.pickPotentialDelegator().addr
	if newDelegator == emptyAddress {
		t.Fatal("no potential delegator found")
	}
	if err := tec.executeVoteIncreaseDeposit(newDelegator); err != nil {
		t.Fatal(err)
	}
	if len(tec.ec.delegatorRecords[newDelegator].votes) != len(tec.ec.candidateRecords) {
		t.Fatal(fmt.Errorf("vote with only 21 candidate gives delegator of votes %v", len(tec.ec.delegatorRecords[newDelegator].votes)))
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
	// Second case, a delegator with 30 votes
	numCandidates := 50
	for i := 0; i != numCandidates; i++ {
		newCandidate := tec.ec.pickPotentialCandidate().addr
		if newCandidate == emptyAddress {
			t.Fatal("no potential candidate found")
		}
		if err := tec.executeAddCandidate(newCandidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := tec.executeVoteIncreaseDeposit(newDelegator); err != nil {
		t.Fatal(err)
	}
	if gotVotes := len(tec.ec.delegatorRecords[newDelegator].votes); gotVotes != maxVotes {
		t.Fatal(fmt.Errorf("vote 30 candidates gives delegator of votes %v", gotVotes))
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
	// Third, decrease vote deposit
	if err := tec.executeVoteDecreaseDeposit(newDelegator); err != nil {
		t.Fatal(err)
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
	// Fourth, cancel votes
	if err := tec.executeCancelVote(newDelegator); err != nil {
		t.Fatal(err)
	}
	if err := tec.commitAndCheckConsistency(); err != nil {
		t.Fatal(err)
	}
}

// newTestEpochContext create a new testEpochContext.
func newTestEpochContext(num int, genesisConfig *genesisConfig) (*testEpochContext, error) {
	if num < len(genesisConfig.config.Dpos.Validators) {
		return nil, fmt.Errorf("number of users must be greater than number of validators")
	}
	curTime := time.Now().Unix() / BlockInterval * BlockInterval
	db := ethdb.NewMemDatabase()
	// Set genesis
	genesisBlock := genesisConfig.toBlock(db)
	genesisHeader := genesisBlock.Header()
	// Get state, context from genesis block
	statedb, err := state.New(genesisBlock.Root(), state.NewDatabase(db))
	if err != nil {
		return nil, fmt.Errorf("cannot create a new stateDB: %v", err)
	}
	ctx := genesisBlock.DposCtx()
	// initialize ec with genesis
	ec := newExpectedContextWithGenesis(genesisConfig)
	// construct the epoch context
	tec := &testEpochContext{
		genesis: genesisHeader,
		curTime: curTime,
		stateDB: statedb,
		ctx:     ctx,
		ec:      ec,
	}
	// Add random users and give them some money
	numUsersToAdd := num - len(genesisConfig.config.Dpos.Validators)
	users := makeUsers(numUsersToAdd)
	for addr, user := range users {
		statedb.CreateAccount(addr)
		statedb.SetBalance(addr, minDeposit.MultInt64(100).BigIntPtr())
		tec.ec.balance[addr] = minDeposit.MultInt64(100)
		tec.ec.userRecords[addr] = user
	}
	return tec, nil
}

// executeRandomOperation picks a random user and execute a random operation.
func (tec *testEpochContext) executeRandomOperation() error {
	var randomUser userRecord
	for _, randomUser = range tec.ec.userRecords {
		break
	}
	addr := randomUser.addr
	opType := randomUser.randomOp(tec.ec)
	switch opType {
	case opTypeAddCandidate:
		return tec.executeAddCandidate(addr)
	case opTypeCancelCandidate:
		return tec.executeCancelCandidate(addr)
	case opTypeVoteIncreaseDeposit:
		return tec.executeVoteIncreaseDeposit(addr)
	case opTypeVoteDecreaseDeposit:
		return tec.executeVoteDecreaseDeposit(addr)
	case opTypeCancelVote:
		return tec.executeCancelVote(addr)
	case opTypeDoNothing:
		return nil
	default:
	}
	return errors.New("unknown op type")
}

// executeAddCandidate execute add candidate.
func (tec *testEpochContext) executeAddCandidate(addr common.Address) error {
	// The new deposit is defined as previous deposit plus 1/10 of the remaining balance
	prevCandidateRecord, exist := tec.ec.candidateRecords[addr]
	prevDeposit, prevRewardRatio := common.BigInt0, uint64(0)
	if exist {
		prevDeposit = prevCandidateRecord.deposit
		prevRewardRatio = prevCandidateRecord.rewardRatio
	}
	newDeposit := prevDeposit.Add(GetAvailableBalance(tec.stateDB, addr).DivUint64(10))
	newRewardRatio := (RewardRatioDenominator-prevRewardRatio)/4 + prevRewardRatio
	// Process Add candidate
	if err := ProcessAddCandidate(tec.stateDB, tec.ctx, addr, newDeposit, newRewardRatio); err != nil {
		return err
	}
	// Update the expected result
	tec.ec.addCandidate(addr, newDeposit, newRewardRatio)
	return nil
}

// executeCancelCandidate execute cancel candidate.
func (tec *testEpochContext) executeCancelCandidate(addr common.Address) error {
	_, exist := tec.ec.candidateRecords[addr]
	if !exist {
		return fmt.Errorf("address %x not previously in candidateRecords", addr)
	}
	if err := ProcessCancelCandidate(tec.stateDB, tec.ctx, addr, tec.curTime); err != nil {
		return err
	}
	// Update the expected result
	tec.ec.cancelCandidate(addr, tec.curTime)
	return nil
}

// executeVoteIncreaseDeposit execute vote with increasing deposit.
func (tec *testEpochContext) executeVoteIncreaseDeposit(addr common.Address) error {
	// get the previous info
	prevVoteRecord, exist := tec.ec.delegatorRecords[addr]
	prevDeposit := common.BigInt0
	if exist {
		prevDeposit = prevVoteRecord.deposit
	}
	// Create the params for the new vote transaction.
	newDeposit := prevDeposit.Add(GetAvailableBalance(tec.stateDB, addr).DivUint64(100))
	votes := randomPickCandidates(tec.ec.candidateRecords, maxVotes)
	if _, err := ProcessVote(tec.stateDB, tec.ctx, addr, newDeposit, votes, tec.curTime); err != nil {
		return err
	}
	// Update expected context
	tec.ec.voteIncreaseDeposit(addr, newDeposit, votes)
	return nil
}

// executeVoteDecreaseDeposit execute vote wiht decreasing deposit.
func (tec *testEpochContext) executeVoteDecreaseDeposit(addr common.Address) error {
	// Get the previous info
	prevVoteRecord, exist := tec.ec.delegatorRecords[addr]
	if !exist {
		return errors.New("when decreasing vote deposit, entry not exist in delegator records")
	}
	prevDeposit := prevVoteRecord.deposit
	// Create params for the new params and vote
	newDeposit := prevDeposit.MultInt64(2).DivUint64(3)
	votes := randomPickCandidates(tec.ec.candidateRecords, maxVotes)
	if _, err := ProcessVote(tec.stateDB, tec.ctx, addr, newDeposit, votes, tec.curTime); err != nil {
		return err
	}
	// Update expected context
	tec.ec.voteDecreaseDeposit(addr, newDeposit, votes, tec.curTime)
	return nil
}

// executeCancelVote execute cancel vote
func (tec *testEpochContext) executeCancelVote(addr common.Address) error {
	_, exist := tec.ec.delegatorRecords[addr]
	if !exist {
		return errors.New("vote record previously not in record map")
	}
	if err := ProcessCancelVote(tec.stateDB, tec.ctx, addr, tec.curTime); err != nil {
		return err
	}
	tec.ec.cancelVote(addr, tec.curTime)
	return nil
}

// commitAndCheckConsistency commit all data structure and check for consistency
func (tec *testEpochContext) commitAndCheckConsistency() error {
	if _, err := tec.stateDB.Commit(true); err != nil {
		return err
	}
	if _, err := tec.ctx.Commit(); err != nil {
		return err
	}
	if err := tec.checkConsistency(); err != nil {
		return err
	}
	return nil
}

// checkConsistent checks whether the expected context is consistent with the state and context.
func (tec *testEpochContext) checkConsistency() error {
	if err := tec.ec.checkSelfConsistency(); err != nil {
		return fmt.Errorf("check expect context self consistency: %v", err)
	}
	if err := tec.checkValidatorsConsistency(); err != nil {
		return fmt.Errorf("check validators consistency: %v", err)
	}
	if err := tec.checkCandidateRecordsConsistency(); err != nil {
		return fmt.Errorf("check candidate records consistency: %v", err)
	}
	if err := tec.checkCandidateRecordsLastEpochConsistency(); err != nil {
		return fmt.Errorf("check candidate records last epoch consistency: %v", err)
	}
	if err := tec.checkDelegatorRecordsConsistency(); err != nil {
		return fmt.Errorf("check delegator records consistency: %v", err)
	}
	if err := tec.checkDelegatorRecordsLastEpochConsistency(); err != nil {
		return fmt.Errorf("check delegator records last epoch consistency: %v", err)
	}
	if err := tec.checkThawingConsistency(); err != nil {
		return fmt.Errorf("check thawing logic consistency: %v", err)
	}
	if err := tec.checkFrozenAssetsConsistency(); err != nil {
		return fmt.Errorf("check frozen assets consistency: %v", err)
	}
	if err := tec.checkBalanceConsistency(); err != nil {
		return fmt.Errorf("check balance consisstency: %v", err)
	}
	return nil
}

// checkValidatorsConsistency checks the consistency of validators
func (tec *testEpochContext) checkValidatorsConsistency() error {
	gotValidators, err := tec.ctx.GetValidators()
	if err != nil {
		return fmt.Errorf("cannot get validators: %v", err)
	}
	exectValidators := tec.ec.getValidators()
	if len(exectValidators) != len(gotValidators) {
		return fmt.Errorf("size of validators not same. Got %d, Expect %d", len(gotValidators), len(exectValidators))
	}
	for i := range exectValidators {
		gotValidator, expectValidator := gotValidators[i], exectValidators[i]
		if gotValidator != expectValidator {
			return fmt.Errorf("validators[%d] not equal %x != %x", i, gotValidator, expectValidator)
		}
	}
	return nil
}

// checkCandidateRecordsConsistency checks the consistency for candidateRecords
func (tec *testEpochContext) checkCandidateRecordsConsistency() error {
	for addr := range tec.ec.userRecords {
		err := tec.ec.checkCandidateRecord(tec.stateDB, tec.ctx, addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkCandidateRecordsLastEpochConsistency checks the consistency for candidateRecords
// in last epoch
func (tec *testEpochContext) checkCandidateRecordsLastEpochConsistency() error {
	// for last epoch, only check for exist entries
	for addr := range tec.ec.candidateRecordsLastEpoch {
		err := tec.ec.checkCandidateRecordLastEpoch(tec.stateDB, tec.ctx, addr, tec.genesis, tec.ec.getValidators())
		if err != nil {
			return err
		}
	}
	return nil
}

// checkDelegatorRecordsConsistency checks the consistency for delegatorRecords
func (tec *testEpochContext) checkDelegatorRecordsConsistency() error {
	for addr := range tec.ec.userRecords {
		err := tec.ec.checkDelegatorRecord(tec.stateDB, tec.ctx, addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkDelegatorRecordsLastEpochConsistency check the consistency of delegatorRecords field
// in expectContext
func (tec *testEpochContext) checkDelegatorRecordsLastEpochConsistency() error {
	for addr := range tec.ec.delegatorRecordsLastEpoch {
		err := tec.ec.checkDelegatorLastEpoch(tec.stateDB, tec.ctx, addr, tec.genesis, tec.ec.getValidators())
		if err != nil {
			return err
		}
	}
	return nil
}

// checkThawingConsistency checks the thawing in the next two epoch whether they are consistent
// in state
func (tec *testEpochContext) checkThawingConsistency() error {
	// only check the thawing effected epoch
	curEpoch := CalculateEpochID(tec.curTime)
	thawEpoch := calcThawingEpoch(curEpoch)
	for epoch := curEpoch + 1; epoch <= thawEpoch; epoch++ {
		thawMap := tec.ec.thawing[epoch]
		expect := make(map[common.Address]common.BigInt)
		for addr, thaw := range thawMap {
			expect[addr] = thaw
		}
		err := checkThawingAddressAndValue(tec.stateDB, epoch, expect)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkFrozenAssetsConsistency checks whether the frozen assets is consistent in state
func (tec *testEpochContext) checkFrozenAssetsConsistency() error {
	for addr := range tec.ec.userRecords {
		frozenAssets, exist := tec.ec.frozenAssets[addr]
		if !exist {
			frozenAssets = common.BigInt0
		}
		gotFrozenAssets := GetFrozenAssets(tec.stateDB, addr)
		if gotFrozenAssets.Cmp(frozenAssets) != 0 {
			return fmt.Errorf("address %x frozen assets not expected: expect %v, got %v", addr,
				frozenAssets, gotFrozenAssets)
		}
	}
	return nil
}

// checkBalanceConsistency checks whether the balance is consistent in state
func (tec *testEpochContext) checkBalanceConsistency() error {
	for addr := range tec.ec.userRecords {
		expectedBalance := tec.ec.getBalance(addr)
		gotBalance := GetBalance(tec.stateDB, addr)
		if expectedBalance.Cmp(gotBalance) != 0 {
			return fmt.Errorf("balance not expected. %x: expect %v; got %v", addr, expectedBalance, gotBalance)
		}
	}
	return nil
}

// newExpectedContextWithGenesis create the expectContext that conform with the given genesis
func newExpectedContextWithGenesis(genesis *genesisConfig) *expectContext {
	ec := &expectContext{
		userRecords:               make(userRecords),
		validators:                make([]common.Address, 0, maxNumValidators),
		candidateRecords:          make(candidateRecords),
		candidateRecordsLastEpoch: make(candidateRecords),
		delegatorRecords:          make(delegatorRecords),
		delegatorRecordsLastEpoch: make(delegatorRecords),
		thawing:                   make(map[int64]map[common.Address]common.BigInt),
		frozenAssets:              make(map[common.Address]common.BigInt),
		balance:                   make(map[common.Address]common.BigInt),
	}

	for addr, balance := range genesis.alloc {
		ec.userRecords[addr] = makeUserRecord()
		ec.balance[addr] = balance
	}
	// Set the genesis
	ec.setGenesis(genesis)
	return ec
}

// setGenesis add the genesis data to the expectContext
func (ec *expectContext) setGenesis(genesis *genesisConfig) {
	// Add allocates
	for addr, balance := range genesis.alloc {
		ec.userRecords[addr] = makeUserRecord()
		ec.balance[addr] = balance
	}
	// Add validators
	validators := make([]common.Address, 0, len(genesis.config.Dpos.Validators))
	for _, vc := range genesis.config.Dpos.Validators {
		validators = append(validators, vc.Address)
		ec.candidateRecords[vc.Address] = candidateRecord{
			deposit:     vc.Deposit,
			rewardRatio: vc.RewardRatio,
			votes:       make(map[common.Address]struct{}),
		}
		ec.candidateRecordsLastEpoch[vc.Address] = candidateRecord{
			deposit:     vc.Deposit,
			rewardRatio: vc.RewardRatio,
			votes:       make(map[common.Address]struct{}),
		}
		ec.frozenAssets[vc.Address] = vc.Deposit
	}
	ec.setValidators(validators)
}

// pickUser randomly pick a user from userRecords
func (ec *expectContext) pickUser() userRecord {
	var randomUser userRecord
	for _, randomUser = range ec.userRecords {
		break
	}
	return randomUser
}

// pickPotentialCandidate randomly pick a potential candidate from userRecords
func (ec *expectContext) pickPotentialCandidate() userRecord {
	var randomUser userRecord
	var addr common.Address
	for addr, randomUser = range ec.userRecords {
		if _, exist := ec.candidateRecords[addr]; !exist && randomUser.isCandidate {
			return randomUser
		}
	}
	return userRecord{}
}

// pickPotentialDelegator randomly pick a potential delegator
func (ec *expectContext) pickPotentialDelegator() userRecord {
	var randomUser userRecord
	var addr common.Address
	for addr, randomUser = range ec.userRecords {
		if _, exist := ec.delegatorRecords[addr]; !exist && randomUser.isDelegator {
			break
		}
	}
	return randomUser
}

// pickCandidate randomly pick a candidate
func (ec *expectContext) pickCandidate() (common.Address, candidateRecord) {
	var candidateRecord candidateRecord
	var addr common.Address
	for addr, candidateRecord = range ec.candidateRecords {
		break
	}
	return addr, candidateRecord
}

// pickDelegator randomly pick a delegator
func (ec *expectContext) pickDelegator() (common.Address, delegatorRecord) {
	var addr common.Address
	var record delegatorRecord
	for addr, record = range ec.delegatorRecords {
		break
	}
	return addr, record
}

// kickoutCandidate kicks out a candidate out. It does the same logic as cancelCandidate
func (ec *expectContext) kickOutCandidate(candidate common.Address, curTime int64) {
	ec.cancelCandidate(candidate, curTime)
}

// addCandidate update expectContext as adding a candidate
func (ec *expectContext) addCandidate(candidate common.Address, newDeposit common.BigInt, rewardRatio uint64) {
	prevRecord, exist := ec.candidateRecords[candidate]
	prevDeposit, prevVote := common.BigInt0, make(map[common.Address]struct{})
	if exist {
		prevDeposit = prevRecord.deposit
		prevVote = prevRecord.votes
	}
	ec.candidateRecords[candidate] = candidateRecord{
		deposit:     newDeposit,
		rewardRatio: rewardRatio,
		votes:       prevVote,
	}
	ec.addFrozenAssets(candidate, newDeposit.Sub(prevDeposit))
}

// cancelCandidate update expectContext of canceling a candidate
func (ec *expectContext) cancelCandidate(candidate common.Address, curTime int64) {
	record, exist := ec.candidateRecords[candidate]
	if !exist {
		return
	}
	for c := range record.votes {
		delete(ec.delegatorRecords[c].votes, candidate)
	}
	delete(ec.candidateRecords, candidate)
	ec.addThawing(candidate, record.deposit, curTime)
}

// voteIncreaseDeposit update expectContext of increase the vote deposit of delegator
func (ec *expectContext) voteIncreaseDeposit(delegator common.Address, newDeposit common.BigInt, votes []common.Address) {
	record, exist := ec.delegatorRecords[delegator]
	prevDeposit := common.BigInt0
	if exist {
		prevDeposit = record.deposit
	}
	ec.deleteDelegateVotes(delegator)
	ec.addDelegateVotes(delegator, newDeposit, votes)
	ec.addFrozenAssets(delegator, newDeposit.Sub(prevDeposit))
}

// voteDecreaseDeposit update expectContext of decrease the vote deposit of delegator
func (ec *expectContext) voteDecreaseDeposit(delegator common.Address, newDeposit common.BigInt, votes []common.Address,
	curTime int64) {

	record, exist := ec.delegatorRecords[delegator]
	prevDeposit := common.BigInt0
	if exist {
		prevDeposit = record.deposit
	}
	ec.deleteDelegateVotes(delegator)
	ec.addDelegateVotes(delegator, newDeposit, votes)
	ec.addThawing(delegator, prevDeposit.Sub(newDeposit), curTime)
}

// cancelVote update expectContext of canceling the vote from delegator
func (ec *expectContext) cancelVote(delegator common.Address, curTime int64) {
	prevDeposit := ec.delegatorRecords[delegator].deposit
	ec.deleteDelegateVotes(delegator)
	ec.addThawing(delegator, prevDeposit, curTime)
}

// checkSelfConsistency checks the consistency for the expectContext itself.
func (ec *expectContext) checkSelfConsistency() error {
	// check the current records
	if err := checkRecordsConsistency(ec.candidateRecords, ec.delegatorRecords); err != nil {
		return fmt.Errorf("current epoch self consistency check failed: %v", err)
	}
	if err := checkRecordsConsistency(ec.candidateRecordsLastEpoch, ec.delegatorRecordsLastEpoch); err != nil {
		return fmt.Errorf("last epoch self consistency check failed: %v", err)
	}
	return nil
}

// checkRecordsConsistency is a helper function to check the consistency between cRecords and
// dRecords
func checkRecordsConsistency(cRecords candidateRecords, dRecords delegatorRecords) error {
	// for each vote in candidate records, should find the vote in delegator records
	for c, cRecord := range cRecords {
		votes := cRecord.votes
		for d := range votes {
			dRecord, exist := dRecords[d]
			if !exist {
				return fmt.Errorf("delegator %x voted candidate %x but not a delegator", d, c)
			}
			if _, exist := dRecord.votes[c]; !exist {
				return fmt.Errorf("delegator %x voted candidate %x but has not voted candidate", d, c)
			}
		}
	}
	// for each vote in delegate records, should find all votes in candidate records
	for d, dRecord := range dRecords {
		votes := dRecord.votes
		for c := range votes {
			cRecord, exist := cRecords[c]
			if !exist {
				return fmt.Errorf("delegator %x voted candidate %x but not a candidate", d, c)
			}
			if _, exist := cRecord.votes[d]; !exist {
				return fmt.Errorf("delegator %x voted candidate %x but candidate no vote from delegator", d, c)
			}
		}
	}
	return nil
}

// checkCandidateRecord checks whether a candidate is consistent in state and ctx
func (ec *expectContext) checkCandidateRecord(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	record, exist := ec.candidateRecords[addr]
	if !exist {
		return checkEmptyCandidate(stateDB, ctx, addr)
	}
	return checkCandidate(stateDB, ctx, addr, record)
}

// checkEmptyCandidate checks in stateDB and ctx whether the addr is an empty candidate
func checkEmptyCandidate(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	candidateDeposit := GetCandidateDeposit(stateDB, addr)
	if candidateDeposit.Cmp(common.BigInt0) != 0 {
		return fmt.Errorf("non candidate address %x have non-zero candidate deposit %v", addr, candidateDeposit)
	}
	// check candidate trie
	b, err := ctx.CandidateTrie().TryGet(addr.Bytes())
	if err == nil && b != nil && len(b) != 0 {
		return fmt.Errorf("non candidate address %x exist in candidate trie", addr)
	}
	// check delegateTrie
	var votes []common.Address
	err = forEachDelegatorForCandidate(ctx, addr, func(address common.Address) error {
		votes = append(votes, address)
		return nil
	})
	if err != errNoEntriesInDelegateTrie {
		return fmt.Errorf("non candidate address %x expect no vote. But got votes %v", addr, formatAddressList(votes))
	}
	return nil
}

// checkCandidate checks in stateDB and ctx whether the addr is consistent to record
func checkCandidate(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address,
	record candidateRecord) error {

	candidateDeposit := GetCandidateDeposit(stateDB, addr)
	if candidateDeposit.Cmp(record.deposit) != 0 {
		return fmt.Errorf("candidate %x does not have expected deposit. Got %v, Expect %v", addr,
			candidateDeposit, record.deposit)
	}
	rewardRatio := GetRewardRatioNumerator(stateDB, addr)
	if rewardRatio != record.rewardRatio {
		return fmt.Errorf("candidate %x does not have expected reward ratio. Got %v, Expect %v", addr,
			rewardRatio, record.rewardRatio)
	}
	// check candidateTrie
	b, err := ctx.CandidateTrie().TryGet(addr.Bytes())
	if err != nil || b == nil || len(b) == 0 {
		return fmt.Errorf("non candidate address %x exist in candidate trie", addr)
	}
	var votes []common.Address
	err = forEachDelegatorForCandidate(ctx, addr, func(addr common.Address) error {
		votes = append(votes, addr)
		return nil
	})
	if err != nil {
		// If expectedVotes has length 0 and no entry found, continue to the next candidate
		if err == errNoEntriesInDelegateTrie && len(record.votes) == 0 {
			return nil
		}
		return fmt.Errorf("check candidate address [%x] delegator: %v", addr, err)
	}
	if err = checkAddressListConsistentToMap(votes, record.votes); err != nil {
		return fmt.Errorf("check candidate address [%x] votes: %v", addr, err)
	}
	return nil
}

// checkCandidateRecordLastEpoch checks the candidate last epoch for addr
func (ec *expectContext) checkCandidateRecordLastEpoch(stateDB *state.StateDB, ctx *types.DposContext,
	addr common.Address, genesis *types.Header, validators []common.Address) error {

	// if the candidate is not in validators, skip checking
	var isValidator bool
	for _, v := range validators {
		if addr == v {
			isValidator = true
			break
		}
	}
	if !isValidator {
		return nil
	}
	record := ec.candidateRecordsLastEpoch[addr]
	gotRewardRatio := GetRewardRatioNumeratorLastEpoch(stateDB, addr)
	if gotRewardRatio != record.rewardRatio {
		return fmt.Errorf("candidate %x last epoch reward ratio not expected. Got %v, Expect %v", addr, gotRewardRatio, record.rewardRatio)
	}
	// Calculate and check the total votes
	expectTotalVotes := record.deposit
	for delegator := range record.votes {
		expectTotalVotes = expectTotalVotes.Add(ec.delegatorRecordsLastEpoch[delegator].deposit)
	}
	gotTotalVotes := GetTotalVote(stateDB, addr)
	// special case for genesis. Skip checking if the candidate have no votes
	if len(record.votes) != 0 && expectTotalVotes.Cmp(gotTotalVotes) != 0 {
		return fmt.Errorf("canidate %x last epoch total vote not expected. Got %v, Expect %v", addr, gotTotalVotes, expectTotalVotes)
	}
	// check the delegate trie from the last epoch
	preEpochSnapshotDelegateTrieRoot := getPreEpochSnapshotDelegateTrieRoot(stateDB, genesis)
	delegateTrie, err := getPreEpochSnapshotDelegateTrie(ctx.DB(), preEpochSnapshotDelegateTrieRoot)
	if err != nil {
		return fmt.Errorf("cannot recover delegate trie: %v", err)
	}
	var gotVotes []common.Address
	expectVotes := record.votes
	err = forEachDelegatorForCandidateFromTrie(delegateTrie, addr, func(delegator common.Address) error {
		gotVotes = append(gotVotes, delegator)
		return nil
	})
	if err != nil {
		// If expectedVotes has length 0 and no entry found, continue to the next candidate
		if err == errNoEntriesInDelegateTrie && len(expectVotes) == 0 {
			return nil
		}
		return fmt.Errorf("check candidate address [%x] delegator last epoch: %v", addr, err)
	}
	if err := checkAddressListConsistentToMap(gotVotes, expectVotes); err != nil {
		return fmt.Errorf("check candidate address [%x] last epoch votes: %v", addr, err)
	}
	return nil
}

// checkDelegatorRecord checks whether a delegator is consistent in state and ctx
func (ec *expectContext) checkDelegatorRecord(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	record, exist := ec.delegatorRecords[addr]
	if !exist {
		return checkEmptyDelegator(stateDB, ctx, addr)
	}
	return checkDelegator(stateDB, ctx, addr, record)
}

// checkEmptyDelegator is a helper function that checks whether a delegator is empty in stateDb and ctx
func checkEmptyDelegator(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	// delegator should not in vote trie
	candidatesByte, err := ctx.VoteTrie().TryGet(addr.Bytes())
	if err == nil && candidatesByte != nil && len(candidatesByte) != 0 {
		return fmt.Errorf("empty delegator %x should not be in vote trie", addr)
	}
	// check delegator deposit
	gotDeposit := GetVoteDeposit(stateDB, addr)
	if gotDeposit.Cmp(common.BigInt0) != 0 {
		return fmt.Errorf("empty delegator %x should have deposit 0, but got %v", addr, gotDeposit)
	}
	return nil
}

// checkDelegator is a helper function that checks whether a delegator is consistent to record
// in statedb and ctx
func checkDelegator(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address, record delegatorRecord) error {
	// delegator should be in vote trie and have candidates in record
	candidatesByte, err := ctx.VoteTrie().TryGet(addr.Bytes())
	if err != nil || candidatesByte == nil || len(candidatesByte) == 0 {
		return fmt.Errorf("delegator %x should be in vote trie", addr)
	}
	var candidates []common.Address
	if err = rlp.DecodeBytes(candidatesByte, &candidates); err != nil {
		return fmt.Errorf("delegator %x vote decode error: %v", addr, err)
	}
	if err = checkAddressListConsistentToMap(candidates, record.votes); err != nil {
		return fmt.Errorf("delegator %x vote not expected: %v", addr, err)
	}
	// check the votes in the delegateTrie
	dt := ctx.DelegateTrie()
	for _, c := range candidates {
		key := append(c.Bytes(), addr.Bytes()...)
		b, err := dt.TryGet(key)
		if err != nil || b == nil {
			return fmt.Errorf("delegator %x vote to %x not found in delegate trie", addr, c)
		}
	}
	return nil
}

// checkDelegatorLastEpoch checks whether a delegator in last epoch is consistent to stateDB and ctx
func (ec *expectContext) checkDelegatorLastEpoch(stateDB *state.StateDB, ctx *types.DposContext,
	addr common.Address, genesis *types.Header, validators []common.Address) error {

	// If the delegator has not voted validator, skip the checking
	var votedValidator bool
	for _, validator := range validators {
		if _, exist := ec.delegatorRecordsLastEpoch[addr].votes[validator]; exist {
			votedValidator = true
			break
		}
	}
	if !votedValidator {
		return nil
	}
	record := ec.delegatorRecordsLastEpoch[addr]
	gotDeposit := GetVoteLastEpoch(stateDB, addr)
	if gotDeposit.Cmp(record.deposit) != 0 {
		return fmt.Errorf("delegator %x last epoch deposit: expect %v, got %v", addr, gotDeposit, record.deposit)
	}
	preEpochSnapshotDelegateTrieRoot := getPreEpochSnapshotDelegateTrieRoot(stateDB, genesis)
	dt, err := getPreEpochSnapshotDelegateTrie(ctx.DB(), preEpochSnapshotDelegateTrieRoot)
	if err != nil {
		return fmt.Errorf("cannot recover delegate trie: %v", err)
	}
	candidates := record.votes
	for c := range candidates {
		key := append(c.Bytes(), addr.Bytes()...)
		b, err := dt.TryGet(key)
		if err != nil || b == nil {
			return fmt.Errorf("delegator %x vote to %x in last epoch not found in delegate trie", addr, c)
		}
	}
	return nil
}

// addDelegateVotes add the delegator's vote to candidate record, and add a record to delegateRecords
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

// delegatorCancelVotes delete the delegator's votes from candidate records, and delete the delegate
// from delegateRecords
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

// addThawing add the thawing of diff amount of address addr to the thawing record.
func (ec *expectContext) addThawing(addr common.Address, diff common.BigInt, curTime int64) {
	thawEpoch := calcThawingEpoch(CalculateEpochID(curTime))
	prevThawing, exist := ec.thawing[thawEpoch][addr]
	if !exist {
		prevThawing = common.BigInt0
	}
	if _, exist := ec.thawing[thawEpoch]; !exist {
		ec.thawing[thawEpoch] = make(map[common.Address]common.BigInt)
	}
	ec.thawing[thawEpoch][addr] = prevThawing.Add(diff)
}

// addFrozenAssets add the frozen assets of an address of diff amount
func (ec *expectContext) addFrozenAssets(addr common.Address, diff common.BigInt) {
	prevFrozenAssets, exist := ec.frozenAssets[addr]
	if !exist {
		prevFrozenAssets = common.BigInt0
	}
	newFrozenAssets := prevFrozenAssets.Add(diff)
	ec.frozenAssets[addr] = newFrozenAssets
}

// getBalance get the expected balance from expectContext
func (ec *expectContext) getBalance(addr common.Address) common.BigInt {
	balance, exist := ec.balance[addr]
	if !exist {
		balance = common.BigInt0
	}
	return balance
}

// addBalance add balance to the address of value diff
func (ec *expectContext) addBalance(addr common.Address, diff common.BigInt) {
	prevBalance, exist := ec.balance[addr]
	if !exist {
		prevBalance = common.BigInt0
	}
	ec.balance[addr] = prevBalance.Add(diff)
}

// setValidators set validators to the expected value
func (ec *expectContext) setValidators(validators []common.Address) {
	ec.validators = validators
}

// getValidators returns a copy of the validatos
func (ec *expectContext) getValidators() []common.Address {
	validators := make([]common.Address, 0, len(ec.validators))
	for _, v := range ec.validators {
		validators = append(validators, v)
	}
	return validators
}

// forEachDelegatorForCandidate iterate over the delegator trie from dpos context for the candidate
// and execute the cb callback function
func forEachDelegatorForCandidate(ctx *types.DposContext, candidate common.Address, cb func(delegator common.Address) error) error {
	delegateTrie := ctx.DelegateTrie()
	return forEachDelegatorForCandidateFromTrie(delegateTrie, candidate, cb)
}

// forEachDelegatorForCandidateFromTrie iterate over the delegator votes for the candidate and execute
//// the cb callback function
func forEachDelegatorForCandidateFromTrie(delegateTrie *trie.Trie, candidate common.Address, cb func(delegator common.Address) error) error {
	delegatorIterator := trie.NewIterator(delegateTrie.PrefixIterator(candidate.Bytes()))
	var hasEntry bool
	for delegatorIterator.Next() {
		hasEntry = true
		delegator := common.BytesToAddress(delegatorIterator.Value)
		if err := cb(delegator); err != nil {
			return err
		}
	}
	if !hasEntry {
		return errNoEntriesInDelegateTrie
	}
	return nil
}

// getGenesis return the genesis config for the integration test
func getGenesis() *genesisConfig {
	genesisValidators := make([]params.ValidatorConfig, 0, maxNumValidators)
	for i := 0; i != maxNumValidators; i++ {
		genesisValidators = append(genesisValidators, params.ValidatorConfig{
			Address:     randomAddress(),
			Deposit:     minDeposit.MultInt64(r.Int63n(10) + 1),
			RewardRatio: uint64(r.Int63n(100)),
		})
	}
	dposConfig := &params.DposConfig{
		Validators: genesisValidators,
	}
	chainConfig := &params.ChainConfig{
		ChainID:        big.NewInt(5),
		HomesteadBlock: big.NewInt(0),
		DAOForkBlock:   nil,
		DAOForkSupport: false,
		EIP150Block:    big.NewInt(0),
		EIP150Hash:     common.Hash{},
		EIP155Block:    big.NewInt(0),
		EIP158Block:    big.NewInt(0),
		ByzantiumBlock: big.NewInt(0),
		Dpos:           dposConfig,
	}
	alloc := make(genesisAlloc)
	// Add some extra balance for the validators
	for _, vc := range genesisValidators {
		newBalance := minDeposit.MultInt64(100)
		alloc[vc.Address] = newBalance
	}
	return &genesisConfig{
		config: chainConfig,
		alloc:  alloc,
	}
}

// toBlock convert the genesis to a block
func (g *genesisConfig) toBlock(db ethdb.Database) *types.Block {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	for addr, balance := range g.alloc {
		statedb.CreateAccount(addr)
		statedb.SetBalance(addr, balance.BigIntPtr())
	}
	dc, err := dposContextWithGenesis(statedb, g, db)
	if err != nil {
		panic(err)
	}
	dcProto := dc.ToRoot()
	root, err := statedb.Commit(true)
	if err != nil {
		panic(err)
	}
	if _, err = dc.Commit(); err != nil {
		panic(err)
	}
	err = statedb.Database().TrieDB().Commit(root, false)
	if err != nil {
		panic(err)
	}
	head := &types.Header{
		Number:      new(big.Int).SetUint64(0),
		Nonce:       types.EncodeNonce(0),
		Time:        new(big.Int).SetUint64(0),
		ParentHash:  common.Hash{},
		Extra:       []byte{},
		GasLimit:    0,
		GasUsed:     0,
		Difficulty:  new(big.Int).SetInt64(0),
		MixDigest:   common.Hash{},
		Coinbase:    common.Address{},
		Root:        root,
		DposContext: dcProto,
	}
	block := types.NewBlock(head, nil, nil, nil)
	block.SetDposCtx(dc)
	return block
}

// dposContextWithGenesis parse the genesis to dposContext
func dposContextWithGenesis(statedb *state.StateDB, g *genesisConfig, db ethdb.Database) (*types.DposContext, error) {
	dc, err := types.NewDposContextFromProto(db, &types.DposContextRoot{})
	if err != nil {
		return nil, err
	}
	validators := make([]common.Address, 0, len(g.config.Dpos.Validators))
	for _, vc := range g.config.Dpos.Validators {
		validatorAddr := vc.Address
		validators = append(validators, vc.Address)
		if err = dc.CandidateTrie().TryUpdate(validatorAddr.Bytes(), validatorAddr.Bytes()); err != nil {
			return nil, err
		}
		SetCandidateDeposit(statedb, validatorAddr, vc.Deposit)
		SetFrozenAssets(statedb, validatorAddr, vc.Deposit)
		SetRewardRatioNumerator(statedb, validatorAddr, vc.RewardRatio)
		SetRewardRatioNumeratorLastEpoch(statedb, validatorAddr, vc.RewardRatio)
	}

	if err = dc.SetValidators(validators); err != nil {
		return nil, err
	}
	statedb.SetNonce(KeyValueCommonAddress, 1)
	statedb.SetState(KeyValueCommonAddress, KeyPreEpochSnapshotDelegateTrieRoot, dc.DelegateTrie().Hash())
	return dc, nil
}

// makeUsers makes number of random users which set isDelegator and isCandidate to a random value
func makeUsers(num int) userRecords {
	records := make(userRecords)
	for i := 0; i != num; i++ {
		record := makeUserRecord()
		records[record.addr] = record
	}
	return records
}

// makeUserRecord make a random userRecord
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

// randomOp select a random operation from all available user operations given the current context
func (user *userRecord) randomOp(ec *expectContext) opType {
	ops := user.availOpList(ec)
	return ops[r.Intn(len(ops))]
}

// availOpList return a list of available operation list given the current context
func (user *userRecord) availOpList(ec *expectContext) []opType {
	ops := make([]opType, 0, 6)
	ops = append(ops, opTypeDoNothing)
	if user.isCandidate {
		ops = append(ops, opTypeAddCandidate)
		if _, exist := ec.candidateRecords[user.addr]; exist && len(ec.candidateRecords) > maxNumValidators {
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

// randomPickCandidates randomly pick num from all candidates
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

// formatAddressList is the formatting of addresses for display
func formatAddressList(addresses []common.Address) string {
	s := "[\n"
	for _, addr := range addresses {
		s += fmt.Sprintf("\t%x\n", addr)
	}
	s += "]"
	return s
}

// checkAddressListConsistentToMap checks whether the address list is the same as map
func checkAddressListConsistentToMap(list []common.Address, m map[common.Address]struct{}) error {
	if len(list) != len(m) {
		return fmt.Errorf("list and map size not equal. %v != %v", len(list), len(m))
	}
	for _, addr := range list {
		_, exist := m[addr]
		if !exist {
			return fmt.Errorf("address %x exist in list not in map", addr)
		}
	}
	return nil
}

func (ec *expectContext) dumpUser(s string) {
	fmt.Println("================== " + s + " dumping user" + " ==================")
	for addr, user := range ec.userRecords {
		fmt.Printf("user %x: %+v\n", addr, struct{ isCandidate, isDelegator bool }{user.isCandidate, user.isDelegator})
	}
	fmt.Println("=============================================")
	fmt.Println()
}

func (ec *expectContext) dumpBalance(s string) {
	fmt.Println("================== " + s + " dumping balance " + " ==================")
	for addr, balance := range ec.balance {
		fmt.Printf("balance %x: %v\n", addr, balance)
	}
	fmt.Println("=============================================")
	fmt.Println()
}

func (ec *expectContext) dumpCandidate(s string) {
	fmt.Println("================== " + s + " dumping candidate " + " ==================")
	for addr, record := range ec.candidateRecords {
		fmt.Printf("balance %x: %+v\n", addr, record)
	}
	fmt.Println("=============================================")
	fmt.Println()
}
