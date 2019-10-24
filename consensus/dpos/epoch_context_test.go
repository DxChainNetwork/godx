// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
)

func TestRewardSubstituteCandidates(t *testing.T) {

	// mock state DB
	db := ethdb.NewMemDatabase()
	state, err := newStateDB(db)
	if err != nil {
		t.Fatalf("failed to create stateDB,error: %v", err)
	}

	// mock 60 randomSelectorEntries, and already sorted as deposit by descending
	substituteCandidates := make(randomSelectorEntries, 0)
	for i := 0; i < 60; i++ {
		addrStr := strconv.FormatUint(uint64(i+1), 10)
		addr := common.HexToAddress("0x" + addrStr)
		entry := &randomSelectorEntry{
			addr: addr,
			vote: common.NewBigInt(1e18).MultUint64(uint64(60 - i)),
		}
		substituteCandidates = append(substituteCandidates, entry)
		SetCandidateDeposit(state, addr, common.NewBigInt(1e18).MultUint64(uint64(i+1)))
	}

	// reward substitute candidates
	rewardSubstituteCandidates(state, substituteCandidates, int64(10), nil)

	// check the balance whether is right
	for i := 0; i < 60; i++ {
		addrStr := strconv.FormatUint(uint64(i+1), 10)
		addr := common.HexToAddress("0x" + addrStr)
		gotBal := state.GetBalance(addr)
		wantedBal := common.NewBigInt(0)
		if i < 50 {
			deposit := GetCandidateDeposit(state, addr)
			switch {
			case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e3)) == -1:
				wantedBal = minCandidateReward
				break
			case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e6)) == -1:
				wantedBal = minCandidateReward.MultInt64(10)
				break
			case deposit.Cmp(common.NewBigInt(1e18).MultInt64(1e9)) == -1:
				wantedBal = minCandidateReward.MultInt64(100)
				break
			default:
				wantedBal = minCandidateReward.MultInt64(1000)
			}
		}

		if gotBal.Cmp(wantedBal.BigIntPtr()) != 0 {
			t.Errorf("balance is not right,got %v,wanted %v", gotBal, wantedBal)
		}
	}
}

func TestDeductPenaltyForValidator(t *testing.T) {

	// mock some const parameters
	epochID := int64(1000)
	initBalance := common.NewBigInt(1e18).MultInt64(1e9)
	initFrozenAsset := common.NewBigInt(1e18).MultInt64(1e6)
	initDeposit := common.NewBigInt(1e18).MultInt64(1e5)
	initThawingAsset := common.NewBigInt(1e18).MultInt64(9e5)
	expectedBlockPerValidator := int64(480)

	tests := []struct {
		name string
		fn   func(state stateDB, validator, penaltyAccount common.Address,
			epochID, expectedBlockPerValidator int64) (common.BigInt, float64)
	}{
		{
			name: "countLostBlocks < expectedBlockPerValidator * 1/16",
			fn: func(state stateDB, validator, penaltyAccount common.Address,
				epochID, expectedBlockPerValidator int64) (common.BigInt, float64) {
				deductPenaltyForValidator(state, validator, penaltyAccount, epochID, 25, expectedBlockPerValidator)
				penaltyRatio := ValidatorPenaltyRatio * 1 / 16
				penalty := initFrozenAsset.MultFloat64(penaltyRatio).DivUint64(PercentageDenominator)
				return penalty, penaltyRatio
			},
		},
		{
			name: "countLostBlocks < expectedBlockPerValidator * 1/8",
			fn: func(state stateDB, validator, penaltyAccount common.Address,
				epochID, expectedBlockPerValidator int64) (common.BigInt, float64) {
				deductPenaltyForValidator(state, validator, penaltyAccount, epochID, 55, expectedBlockPerValidator)
				penaltyRatio := ValidatorPenaltyRatio * 1 / 8
				penalty := initFrozenAsset.MultFloat64(penaltyRatio).DivUint64(PercentageDenominator)
				return penalty, penaltyRatio
			},
		},
		{
			name: "countLostBlocks < expectedBlockPerValidator * 1/4",
			fn: func(state stateDB, validator, penaltyAccount common.Address,
				epochID, expectedBlockPerValidator int64) (common.BigInt, float64) {
				deductPenaltyForValidator(state, validator, penaltyAccount, epochID, 110, expectedBlockPerValidator)
				penaltyRatio := ValidatorPenaltyRatio * 1 / 4
				penalty := initFrozenAsset.MultFloat64(penaltyRatio).DivUint64(PercentageDenominator)
				return penalty, penaltyRatio
			},
		},
		{
			name: "countLostBlocks <= expectedBlockPerValidator * 1/2",
			fn: func(state stateDB, validator, penaltyAccount common.Address,
				epochID, expectedBlockPerValidator int64) (common.BigInt, float64) {
				deductPenaltyForValidator(state, validator, penaltyAccount, epochID, 240, expectedBlockPerValidator)
				penaltyRatio := ValidatorPenaltyRatio * 1 / 2
				penalty := initFrozenAsset.MultFloat64(penaltyRatio).DivUint64(PercentageDenominator)
				return penalty, penaltyRatio
			},
		},
		{
			name: "countLostBlocks > expectedBlockPerValidator * 1/2",
			fn: func(state stateDB, validator, penaltyAccount common.Address,
				epochID, expectedBlockPerValidator int64) (common.BigInt, float64) {
				deductPenaltyForValidator(state, validator, penaltyAccount, epochID, 300, expectedBlockPerValidator)
				penaltyRatio := ValidatorPenaltyRatio
				penalty := initFrozenAsset.MultFloat64(penaltyRatio).DivUint64(PercentageDenominator)
				return penalty, penaltyRatio
			},
		},
	}

	for _, test := range tests {

		// mock state db
		db := ethdb.NewMemDatabase()
		sdb := state.NewDatabase(db)
		state, err := state.New(common.Hash{}, sdb)
		if err != nil {
			t.Fatalf("failed to create state db,error: %v", err)
		}

		// mock validator and delegator account
		validator := common.HexToAddress("0x1")
		penaltyAccount := common.HexToAddress("0xccc")
		state.CreateAccount(validator)
		state.AddBalance(validator, initBalance.BigIntPtr())

		// mock frozen asset

		SetFrozenAssets(state, validator, initFrozenAsset)
		SetCandidateDeposit(state, validator, initDeposit)
		SetThawingAssets(state, validator, epochID, initThawingAsset)

		validatorPenalty, penaltyRatio := test.fn(state, validator, penaltyAccount, epochID, expectedBlockPerValidator)
		validatorBal := state.GetBalance(validator)
		penaltyAccountBal := state.GetBalance(penaltyAccount)
		validatorFrozenAsset := GetFrozenAssets(state, validator)
		validatorDeposit := GetCandidateDeposit(state, validator)
		validatorThawingAsset := GetThawingAssets(state, validator, epochID)

		// check validator\delegator\penaltyAccount balance
		if validatorBal.Cmp(initBalance.Sub(validatorPenalty).BigIntPtr()) != 0 {
			t.Errorf("the case [%s] wanted validator balance %v,got %v", test.name, initBalance.Sub(validatorPenalty), validatorBal)
		}

		if penaltyAccountBal.Cmp(validatorPenalty.BigIntPtr()) != 0 {
			t.Errorf("the case [%s] wanted penalty account balance %v,got %v", test.name, validatorPenalty, penaltyAccountBal)
		}

		wantedDeposit := initDeposit.MultFloat64(float64(PercentageDenominator) - penaltyRatio).DivUint64(PercentageDenominator)
		if wantedDeposit.Cmp(validatorDeposit) != 0 {
			t.Errorf("the case [%s] wanted validator deposit %v,got %v", test.name, wantedDeposit, validatorDeposit)
		}

		wantedThawingAsset := initThawingAsset.MultFloat64(float64(PercentageDenominator) - penaltyRatio).DivUint64(PercentageDenominator)
		if wantedThawingAsset.Cmp(validatorThawingAsset) != 0 {
			t.Errorf("the case [%s] wanted validator thawing asset %v,got %v", test.name, wantedThawingAsset, validatorThawingAsset)
		}

		wantedFrozenAsset := initFrozenAsset.MultFloat64(float64(PercentageDenominator) - penaltyRatio).DivUint64(PercentageDenominator)
		if wantedFrozenAsset.Cmp(validatorFrozenAsset) != 0 {
			t.Errorf("the case [%s] wanted validator frozen asset %v,got %v", test.name, wantedFrozenAsset, validatorFrozenAsset)
		}

		if validatorDeposit.Add(validatorThawingAsset).Cmp(validatorFrozenAsset) != 0 {
			t.Errorf("the case [%s] forzenAsset[%v] != deposit[%v] + thawingAsset[%v]", test.name, validatorFrozenAsset, validatorDeposit, validatorThawingAsset)
		}
	}
}

func TestDeductPenaltyForDelegator(t *testing.T) {

	// mock some const parameters
	epochID := int64(1000)

	// mock state db
	db := ethdb.NewMemDatabase()
	sdb := state.NewDatabase(db)
	state, err := state.New(common.Hash{}, sdb)
	if err != nil {
		t.Fatalf("failed to create state db,error: %v", err)
	}

	// mock validator and delegator account
	validator := common.HexToAddress("0x1")
	penaltyAccount := common.HexToAddress("0xccc")

	delegateTrie, err := types.NewDelegateTrie(common.Hash{}, trie.NewDatabase(db))
	if err != nil {
		t.Fatalf("failed to create delegate trie,error: %v", err)
	}

	// mock some delegate records
	for i := int64(0); i < 3; i++ {
		str := strconv.FormatInt(i+1, 10)
		delegatorAddr := common.HexToAddress("0x" + str)
		err = delegateTrie.TryUpdate(append(validator.Bytes(), delegatorAddr.Bytes()...), delegatorAddr.Bytes())
		if err != nil {
			t.Fatalf("failed to update delegate trie,error: %v", err)
		}

		initBalance := common.NewBigInt(1e18).MultInt64((i + 1) * 1e9)
		initFrozenAsset := common.NewBigInt(1e18).MultInt64((i + 1) * 1e6)
		initDeposit := common.NewBigInt(1e18).MultInt64((i + 1) * 1e5)
		initThawingAsset := common.NewBigInt(1e18).MultInt64((i + 1) * 9e5)
		state.AddBalance(delegatorAddr, initBalance.BigIntPtr())
		SetFrozenAssets(state, delegatorAddr, initFrozenAsset)
		SetVoteDeposit(state, delegatorAddr, initDeposit)
		SetThawingAssets(state, delegatorAddr, epochID, initThawingAsset)
	}

	deductPenaltyForDelegator(state, delegateTrie, penaltyAccount, validator, epochID)

	// check delegator and penalty account balance
	for i := int64(0); i < 3; i++ {
		str := strconv.FormatInt(i+1, 10)
		delegatorAddr := common.HexToAddress("0x" + str)
		penalty := common.NewBigInt(1e18).MultInt64((i + 1) * 1e6).MultUint64(DelegatorPenaltyRatio).DivUint64(PercentageDenominator)
		wantedBalance := common.NewBigInt(1e18).MultInt64((i + 1) * 1e9).Sub(penalty)
		wantedFrozenAsset := common.NewBigInt(1e18).MultInt64((i + 1) * 1e6).MultUint64(PercentageDenominator - DelegatorPenaltyRatio).DivUint64(PercentageDenominator)
		wantedDeposit := common.NewBigInt(1e18).MultInt64((i + 1) * 1e5).MultUint64(PercentageDenominator - DelegatorPenaltyRatio).DivUint64(PercentageDenominator)
		wantedThawingAsset := common.NewBigInt(1e18).MultInt64((i + 1) * 9e5).MultUint64(PercentageDenominator - DelegatorPenaltyRatio).DivUint64(PercentageDenominator)
		delBal := state.GetBalance(delegatorAddr)
		delFrozenAsset := GetFrozenAssets(state, delegatorAddr)
		delDeposit := GetVoteDeposit(state, delegatorAddr)
		delThawingAsset := GetThawingAssets(state, delegatorAddr, epochID)

		if delBal.Cmp(wantedBalance.BigIntPtr()) != 0 {
			t.Errorf("wanted balance %v,got %v", wantedBalance, delBal)
		}

		if delFrozenAsset.Cmp(wantedFrozenAsset) != 0 {
			t.Errorf("wanted frozen asset %v,got %v", wantedFrozenAsset, delFrozenAsset)
		}

		if delDeposit.Cmp(wantedDeposit) != 0 {
			t.Errorf("wanted deposit %v,got %v", wantedDeposit, delDeposit)
		}

		if delThawingAsset.Cmp(wantedThawingAsset) != 0 {
			t.Errorf("wanted thawing asset %v,got %v", wantedThawingAsset, delThawingAsset)
		}
	}
}

func TestLookupValidator(t *testing.T) {
	db := ethdb.NewMemDatabase()
	dposCtx, _ := types.NewDposContext(db)
	mockEpochContext := &EpochContext{
		DposContext: dposCtx,
	}

	validators := []common.Address{
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
		common.HexToAddress("0x3"),
	}

	err := mockEpochContext.DposContext.SetValidators(validators)
	if err != nil {
		t.Fatalf("Failed to set valdiators,error: %v", err)
	}

	for i, expected := range validators {
		got, _ := mockEpochContext.lookupValidator(int64(i) * BlockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.String(), got.String())
		}
	}

	_, err = mockEpochContext.lookupValidator(BlockInterval - 1)
	if err != errInvalidMinedBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", errInvalidMinedBlockTime, err)
	}
}

func Test_CountVotes(t *testing.T) {

	// mock addresses
	addresses := []common.Address{
		common.HexToAddress("0x58a366c3c1a735bf3d09f2a48a014a8ebc64457c"),
		common.HexToAddress("0x60c8947134be7c0604a866a0462542eb0dcf71f9"),
		common.HexToAddress("0x801ee9587ea0d52fe477755a3e91d7244e6556a3"),
		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"),
		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"),
	}

	// mock state
	db := ethdb.NewMemDatabase()
	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	for i := 0; i < len(addresses); i++ {
		stateDB.SetNonce(addresses[i], 1)
		bal := int64(1e10 * (i + 1))
		stateDB.SetBalance(addresses[i], new(big.Int).SetInt64(bal))
	}
	root, _ := stateDB.Commit(false)
	stateDB, _ = state.New(root, sdb)

	// create epoch context
	dposCtx, _ := types.NewDposContext(db)
	epochContext := &EpochContext{
		DposContext: dposCtx,
		stateDB:     stateDB,
	}

	// mock some vote records
	for i, addr := range addresses {
		addrBytes := addr.Bytes()
		err := epochContext.DposContext.CandidateTrie().TryUpdate(addrBytes, addrBytes)
		if err != nil {
			t.Fatalf("Failed to update candidates,error: %v", err)
		}

		for j := 0; j < len(addresses); j++ {
			key := append(addresses[j].Bytes(), addrBytes...)
			err = epochContext.DposContext.DelegateTrie().TryUpdate(key, addrBytes)
			if err != nil {
				t.Fatalf("Failed to update vote records,error: %v", err)
			}
		}

		_, err = epochContext.DposContext.Commit()
		if err != nil {
			t.Fatalf("Failed to commit mock dpos context,error: %v", err)
		}

		// set candidates deposit
		candidateDeposit := new(big.Int).SetInt64(int64(1e6 * (i + 1)))
		stateDB.SetState(addr, KeyCandidateDeposit, common.BytesToHash(candidateDeposit.Bytes()))

		// set vote deposit
		voteDeposit := new(big.Int).SetInt64(int64(1e6 * (i + 1)))
		stateDB.SetState(addr, KeyVoteDeposit, common.BytesToHash(voteDeposit.Bytes()))

		_, err = stateDB.Commit(false)
		if err != nil {
			t.Fatalf("Failed to commit state,error: %v", err)
		}
	}

	// count votes
	votes, err := epochContext.countVotes()
	if err != nil {
		t.Errorf("Failed to count votes,error: %v", err)
	}

	// check vote weight without attenuation
	expectedVoteWeightWithoutAttenuation := int64(15e6)
	for _, entry := range votes {
		addr, weight := entry.addr, entry.vote
		candidateDeposit := stateDB.GetState(addr, KeyCandidateDeposit).Big()
		wantTotalVoteWeight := expectedVoteWeightWithoutAttenuation + candidateDeposit.Int64()
		if weight.Cmp(common.NewBigInt(wantTotalVoteWeight)) != 0 {
			t.Errorf("%s wanted vote weight: %d,got %v", addr.String(), wantTotalVoteWeight, weight)
		}
	}
}

func Test_KickoutValidators(t *testing.T) {
	now := time.Now().Unix()
	var (
		delegator = common.HexToAddress("0xaaa")
	)

	db := ethdb.NewMemDatabase()
	dposContext, candidates, err := mockDposContext(db, now, delegator)
	if err != nil {
		t.Fatalf("failed to new dpos context,error: %v", err)
	}

	timeOfFirstBlock = 100000

	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	epochContext := &EpochContext{
		DposContext: dposContext,
		TimeStamp:   now,
		stateDB:     stateDB,
	}

	epochID := CalculateEpochID(now)
	err = epochContext.kickoutValidators(epochID)
	if err != nil {
		t.Errorf("something wrong to kick out validators,error: %v", err)
	}

	validatorsFromTrie, err := epochContext.DposContext.GetValidators()
	if err != nil {
		t.Fatalf("failed to retrieve validators,error: %v", err)
	}

	if len(validatorsFromTrie) != MaxValidatorSize {
		t.Errorf("after kick out,wanted validator length: %v,got: %v", MaxValidatorSize, len(validatorsFromTrie))
	}

	for i := 0; i < MaxValidatorSize/3; i++ {
		canFromTrie := epochContext.DposContext.CandidateTrie().Get(candidates[i].Bytes())
		if canFromTrie != nil {
			t.Errorf("failed to delete the kick out one from candidates trie: %s", candidates[i].String())
		}

		delegatorFromTrie := epochContext.DposContext.DelegateTrie().Get(append(candidates[i].Bytes(), delegator.Bytes()...))
		if delegatorFromTrie != nil {
			t.Errorf("failed to delete the kick out one from delegate trie: %s", candidates[i].String())
		}
	}

	votedFromTrie := epochContext.DposContext.VoteTrie().Get(delegator.Bytes())
	if votedFromTrie == nil {
		t.Fatalf("failed to retrieve voted candidates")
	}

	var votedCan []common.Address
	err = rlp.DecodeBytes(votedFromTrie, &votedCan)
	if err != nil {
		t.Fatalf("failed to rlp decode voted candidates")
	}

	if len(votedCan) != MaxValidatorSize+5-MaxValidatorSize/3 {
		t.Errorf("failed to delete the kick out one from vote trie,wanted length: %d,got: %d", MaxValidatorSize+5-MaxValidatorSize/3, len(votedCan))
	}
}

// TestAllDelegatorForValidators test the function allDelegatorForValidators
func TestAllDelegatorForValidators(t *testing.T) {
	stateDB, ctx, candidates, err := newStateAndDposContextWithCandidate(1000)
	if err != nil {
		t.Fatal(err)
	}
	// randomly pick 21 validators from the 1000 candidates
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	validators, validatorMap := randomSelectWithListAndMapFromAddress(candidates, 21, r)
	// selectedDelegators are a map of delegators who select the validators in candidates
	selectedDelegators := make(map[common.Address]struct{})
	votedCandidates := make(map[common.Address]struct{})
	numDelegators, deposit, curTime := 10000, dx.MultInt64(100), time.Now().Unix()
	// Vote numDelegators delegator
	for i := 0; i != numDelegators; i++ {
		// at 1/100 ratio, the vote come from an existing candidates
		addr := randomAddress()
		if r.Intn(100) == 0 {
			c := candidates[r.Intn(len(candidates))]
			if _, exist := votedCandidates[c]; !exist {
				votedCandidates[c] = struct{}{}
				addr = c
			}
		}
		selected, err := randomProcessVote(stateDB, ctx, addr, deposit, candidates, validatorMap, curTime, r)
		if err != nil {
			t.Fatal(err)
		}
		if selected {
			selectedDelegators[addr] = struct{}{}
		}
	}
	// Execute the function
	res := allDelegatorForValidators(ctx, validators)
	// Check the results. The result should be exactly the same as selectedDelegators
	if err = checkSetsEqual(selectedDelegators, res); err != nil {
		t.Fatal(err)
	}
}

// randomProcessVote use the input arguments as vote parameters, randomly select 30 voting candidates from candidates
// and vote. If one or more of the selected candidates exist in validators, return true and error. Else return false.
func randomProcessVote(stateDB *state.StateDB, ctx *types.DposContext, addr common.Address, deposit common.BigInt, candidates []common.Address,
	validators map[common.Address]struct{}, time int64, r *rand.Rand) (bool, error) {

	selected := false
	stateDB.AddBalance(addr, deposit.BigIntPtr())
	votedCandidates := randomSelectFromAddress(candidates, 30, r)
	for _, c := range votedCandidates {
		if _, exist := validators[c]; exist {
			selected = true
			break
		}
	}

	voteData := &types.VoteTxData{
		Deposit:    deposit,
		Candidates: votedCandidates,
		Duration:   uint64(EpochInterval),
	}
	if _, err := ProcessVote(stateDB, ctx, addr, voteData, time, time); err != nil {
		return false, err
	}
	return selected, nil
}

func randomSelectWithListAndMapFromAddress(source []common.Address, num int, r *rand.Rand) ([]common.Address, map[common.Address]struct{}) {
	validators := randomSelectFromAddress(source, num, r)
	vm := make(map[common.Address]struct{})
	for _, v := range validators {
		vm[v] = struct{}{}
	}
	return validators, vm
}

func randomSelectFromAddress(rawList []common.Address, num int, r *rand.Rand) []common.Address {
	length := len(rawList)
	if length <= num {
		return rawList
	}
	res := make([]common.Address, 0, num)
	list := make([]common.Address, length)
	copy(list, rawList)
	// Randomly select num of addresses from the list
	for i := 0; i != num; i++ {
		index := r.Intn(length)
		res = append(res, list[index])
		list[index] = list[length-1]
		length--
	}
	return res
}

func checkSetsEqual(m1, m2 map[common.Address]struct{}) error {
	// Copy m2
	m2Copy := make(map[common.Address]struct{})
	for k, v := range m2 {
		m2Copy[k] = v
	}
	for k1 := range m1 {
		if _, exist := m2Copy[k1]; !exist {
			return fmt.Errorf("key %v exist in m1 not in m2", k1)
		}
		delete(m2Copy, k1)
	}
	if len(m2Copy) != 0 {
		return fmt.Errorf("m2 contains more key than m1: %v", m2Copy)
	}
	return nil
}
