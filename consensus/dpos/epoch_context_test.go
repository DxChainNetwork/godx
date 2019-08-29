// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

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
		got, _ := mockEpochContext.lookupValidator(int64(i) * blockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.String(), got.String())
		}
	}

	_, err = mockEpochContext.lookupValidator(blockInterval - 1)
	if err != ErrInvalidMintBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", ErrInvalidMintBlockTime, err)
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
			t.Fatalf("Failed to update candidate,error: %v", err)
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

		// set vote deposit
		deposit := new(big.Int).SetInt64(int64(1e6 * (i + 1)))
		stateDB.SetState(addr, common.BytesToHash([]byte("vote-deposit")), common.BytesToHash(deposit.Bytes()))

		ratio := float64(1.0)
		bits := math.Float64bits(ratio)
		ratioBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(ratioBytes, bits)
		stateDB.SetState(addr, common.BytesToHash([]byte("real-vote-weight-ratio")), common.BytesToHash(ratioBytes))
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
	for addr, weight := range votes {
		if weight.Int64() != expectedVoteWeightWithoutAttenuation {
			t.Errorf("%s wanted vote weight: %d,got %d", addr.String(), expectedVoteWeightWithoutAttenuation, weight.Int64())
		}
	}

	// set vote attenuation ratio
	for i, addr := range addresses {
		ratio := float64(i+1) / 10
		bits := math.Float64bits(ratio)
		ratioBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(ratioBytes, bits)
		stateDB.SetState(addr, common.BytesToHash([]byte("real-vote-weight-ratio")), common.BytesToHash(ratioBytes))
	}
	votes, err = epochContext.countVotes()
	if err != nil {
		t.Errorf("Failed to count votes,error: %v", err)
	}

	// check vote weight with attenuation
	expectedVoteWeightWithAttenuation := int64(5.5e6)
	for addr, weight := range votes {
		if weight.Int64() != expectedVoteWeightWithAttenuation {
			t.Errorf("%s wanted vote weight: %d,got %d", addr.String(), expectedVoteWeightWithAttenuation, weight.Int64())
		}
	}
}
