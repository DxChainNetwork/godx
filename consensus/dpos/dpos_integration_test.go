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

	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"

	"github.com/DxChainNetwork/godx/rlp"

	"github.com/DxChainNetwork/godx/trie"

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

	errNoEntriesInDelegateTrie = errors.New("no entries in delegate trie")

	maxNumValidators = 21
)

type (
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

func newTestEpochContext(num int) (*testEpochContext, error) {
	curTime := time.Now().Unix() / BlockInterval * BlockInterval
	db := ethdb.NewMemDatabase()
	// Write genesis
	genesisConfig := getGenesis()
	genesisBlock := genesisConfig.ToBlock(db)
	genesisHeader := genesisBlock.Header()
	// get state from genesis
	statedb, err := state.New(genesisBlock.Root(), state.NewDatabase(db))
	if err != nil {
		return nil, fmt.Errorf("cannot create a new stateDB: %v", err)
	}
	ctx := genesisBlock.DposCtx()
	// construct the epoch context
	tec := &testEpochContext{
		genesis: genesisHeader,
		curTime: curTime,
		stateDB: statedb,
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
	// initialize with genesis
	tec.ec.setGenesis(genesisConfig)
	// Add balance for all users
	for addr := range tec.ec.userRecords {
		statedb.CreateAccount(addr)
		statedb.SetBalance(addr, minDeposit.MultInt64(100).BigIntPtr())
		tec.ec.balance[addr] = minDeposit.MultInt64(100)
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
	newDeposit := prevDeposit.Add(GetAvailableBalance(tec.stateDB, addr).DivUint64(10))
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
	newDeposit := prevDeposit.Add(GetAvailableBalance(tec.stateDB, addr).DivUint64(100))
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

// checkConsistent checks whether the expected context is consistent with the state and context.
func (tec *testEpochContext) checkConsistency() error {
	if err := tec.ec.checkSelfConsistency(); err != nil {
		return fmt.Errorf("check expect context self consistency: %v", err)
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

func (tec *testEpochContext) checkCandidateRecordsLastEpochConsistency() error {
	// for last epoch, only check for exist entries
	for addr := range tec.ec.candidateRecordsLastEpoch {
		err := tec.ec.checkCandidateRecordLastEpoch(tec.stateDB, tec.ctx, addr, tec.genesis)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tec *testEpochContext) checkDelegatorRecordsConsistency() error {
	for addr := range tec.ec.userRecords {
		err := tec.ec.checkDelegatorRecord(tec.stateDB, tec.ctx, addr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tec *testEpochContext) checkDelegatorRecordsLastEpochConsistency() error {
	for addr := range tec.ec.delegatorRecordsLastEpoch {
		err := tec.ec.checkDelegatorLastEpoch(tec.stateDB, tec.ctx, addr, tec.genesis)
		if err != nil {
			return err
		}
	}
	return nil
}

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

func (tec *testEpochContext) checkBalanceConsistency() error {
	for addr := range tec.ec.userRecords {
		expectedBalance := tec.ec.getExpectedBalance(addr)
		gotBalance := GetBalance(tec.stateDB, addr)
		if expectedBalance.Cmp(gotBalance) != 0 {
			return fmt.Errorf("balance not expected. %x: expect %v; got %v", addr, expectedBalance, gotBalance)
		}
	}
	return nil
}

// setGenesis add the genesis data to the expectContext
func (ec *expectContext) setGenesis(genesis *core.Genesis) {
	validators := genesis.Config.Dpos.Validators
	for v, vConfig := range validators {
		ec.userRecords[v] = userRecord{
			addr:        v,
			isCandidate: true,
			isDelegator: false,
		}
		ec.candidateRecords[v] = candidateRecord{
			deposit:     vConfig.Deposit,
			rewardRatio: vConfig.RewardRatio,
			votes:       make(map[common.Address]struct{}),
		}
		ec.addFrozenAssets(v, vConfig.Deposit)
		ec.addCandidate(v, vConfig.Deposit, vConfig.RewardRatio, make(map[common.Address]struct{}))
		ec.addBalance(v, vConfig.Deposit)
		ec.candidateRecordsLastEpoch[v].rewardRatio = vConfig.RewardRatio
	}
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

func (ec *expectContext) getExpectedBalance(addr common.Address) common.BigInt {
	balance, exist := ec.balance[addr]
	if !exist {
		balance = common.BigInt0
	}
	return balance
}

func (ec *expectContext) checkCandidateRecord(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	record, exist := ec.candidateRecords[addr]
	if !exist {
		return checkEmptyCandidate(stateDB, ctx, addr)
	}
	return checkCandidate(stateDB, ctx, addr, record)
}

func (ec *expectContext) addBalance(addr common.Address, diff common.BigInt) {
	prevBalance, exist := ec.balance[addr]
	if !exist {
		prevBalance = common.BigInt0
	}
	ec.balance[addr] = prevBalance.Add(diff)
}

// checkEmptyCandidate checks in stateDB and ctx whether the addr is an empty candidate
func checkEmptyCandidate(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	candidateDeposit := GetCandidateDeposit(stateDB, addr)
	if candidateDeposit.Cmp(common.BigInt0) != 0 {
		return fmt.Errorf("non candidate address %x have non-zero candidate deposit %v", addr, candidateDeposit)
	}
	rewardRatio := GetRewardRatioNumerator(stateDB, addr)
	if rewardRatio != 0 {
		return fmt.Errorf("non candidate address %x have non-zero reward ratio %v", addr, rewardRatio)
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

func (ec *expectContext) checkCandidateRecordLastEpoch(stateDB *state.StateDB, ctx *types.DposContext,
	addr common.Address, genesis *types.Header) error {

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
	if expectTotalVotes.Cmp(gotTotalVotes) != 0 {
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

func (ec *expectContext) checkDelegatorRecord(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address) error {
	record, exist := ec.delegatorRecords[addr]
	if !exist {
		return checkEmptyDelegator(stateDB, ctx, addr)
	}
	return checkDelegator(stateDB, ctx, addr, record)
}

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

func (ec *expectContext) checkDelegatorLastEpoch(stateDB *state.StateDB, ctx *types.DposContext,
	addr common.Address, genesis *types.Header) error {

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

// forEachDelegatorForCandidate iterate over the delegator votes for the candidate and execute
// the cb callback function
func forEachDelegatorForCandidate(ctx *types.DposContext, candidate common.Address, cb func(delegator common.Address) error) error {
	delegateTrie := ctx.DelegateTrie()
	return forEachDelegatorForCandidateFromTrie(delegateTrie, candidate, cb)
}

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

func formatAddressList(addresses []common.Address) string {
	s := "[\n"
	for _, addr := range addresses {
		s += fmt.Sprintf("\t%x\n", addr)
	}
	s += "]"
	return s
}

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

func getGenesis() *core.Genesis {
	genesisValidators := make(map[common.Address]params.ValidatorConfig)
	for i := 0; i != maxNumValidators; i++ {
		genesisValidators[randomAddress()] = params.ValidatorConfig{
			Deposit:     minDeposit.MultInt64(r.Int63n(10) + 1),
			RewardRatio: uint64(r.Int63n(100)),
		}
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
		Dpos: &params.DposConfig{
			Validators: genesisValidators,
		},
	}
	return &core.Genesis{
		Config:     chainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x4478436861696e204e6574776f726b20546573746e657420302e372e32"),
		GasLimit:   3141592,
		Difficulty: big.NewInt(1048576),
		Alloc: map[common.Address]core.GenesisAccount{
			common.HexToAddress("0x71562b71999873DB5b286dF957af199Ec94617F7"): {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 192), big.NewInt(9))},
		},
	}
}
