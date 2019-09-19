// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
)

func TestMarkThawingAddressAndValue(t *testing.T) {
	num := 100
	db := ethdb.NewMemDatabase()
	state, addresses, err := newStateDBWithAccounts(db, num)
	if err != nil {
		t.Fatal(err)
	}
	// Mark 100 addresses as thaw in 2 epochs
	epoch1, epoch2 := int64(100), int64(101)
	m1 := randomMarkThawAddresses(state, addresses, epoch1)
	m2 := randomMarkThawAddresses(state, addresses, epoch2)
	// check the two periods
	if err := checkThawingAddressAndValue(state, calcThawingEpoch(epoch1), m1); err != nil {
		t.Error("period1: ", err)
	}
	if err := checkThawingAddressAndValue(state, calcThawingEpoch(epoch2), m2); err != nil {
		t.Error("period2: ", err)
	}
}

// TestThawAllFrozenAssetsInEpoch test the functionality of thawAllFrozenAssetsInEpoch
func TestThawAllFrozenAssetsInEpoch(t *testing.T) {
	num := 100
	db := ethdb.NewMemDatabase()
	state, addresses, err := newStateDBWithAccounts(db, num)
	if err != nil {
		t.Fatal(err)
	}
	// Mark thawing for the addresses
	epoch1, epoch2 := int64(100), int64(101)
	randomMarkThawAddresses(state, addresses, epoch1)
	randomMarkThawAddresses(state, addresses, epoch2)
	epoch1, epoch2 = calcThawingEpoch(epoch1), calcThawingEpoch(epoch2)
	// thaw the assets
	if err := thawAllFrozenAssetsInEpoch(state, epoch1); err != nil {
		t.Fatal(err)
	}
	if err := thawAllFrozenAssetsInEpoch(state, epoch2); err != nil {
		t.Fatal(err)
	}
	state.IntermediateRoot(true)
	// After thawing all addresses should have frozen assets of value 0
	for _, addr := range addresses {
		frozenAssets := GetFrozenAssets(state, addr)
		if frozenAssets.Cmp(common.BigInt0) != 0 {
			t.Errorf("Address %v still have frozen assets %v", addr, frozenAssets)
		}
	}
	thawingAddress1, thawingAddress2 := getThawingAddress(epoch1), getThawingAddress(epoch2)
	if state.Exist(thawingAddress1) {
		t.Errorf("after thawing, thawing address 1 %v not removed", thawingAddress1)
	}
	if state.Exist(thawingAddress2) {
		t.Errorf("after thawing, thawing address 2 %v not removed", thawingAddress2)
	}
}

// TestThawAllFrozenAssetsInEpochError test the insufficient frozen assets error for
// function thawAllFrozenAssetsInEpoch
func TestThawAllFrozenAssetsInEpochError(t *testing.T) {
	num := 1
	db := ethdb.NewMemDatabase()
	state, addresses, err := newStateDBWithAccounts(db, num)
	if err != nil {
		t.Fatal(err)
	}
	addr := addresses[0]
	// Mark thawing for the addresses
	epoch := int64(100)
	randomMarkThawAddresses(state, addresses, epoch)
	// Hard code to set the frozen assets to 0, which should incur error
	setFrozenAssets(state, addr, common.BigInt0)
	// thaw the asset, which should trigger errInsufficientFrozenAssets error
	epoch = calcThawingEpoch(epoch)
	err = thawAllFrozenAssetsInEpoch(state, epoch)
	if err != errInsufficientFrozenAssets {
		t.Errorf("error expect [%v], got [%v]", errInsufficientFrozenAssets, err)
	}
}

// checkThawingAddressAndValue checks the result of markThawingAddressAndValue
func checkThawingAddressAndValue(state stateDB, epoch int64, expect map[common.Address]common.BigInt) error {
	thawingAddress := getThawingAddress(epoch)
	var err error
	forEachEntryInThawingAddress(state, thawingAddress, func(addr common.Address) {
		expectTa, exist := expect[addr]
		if !exist {
			err = fmt.Errorf("address %v not in map", addr)
			return
		}
		gotTa := getThawingAssets(state, addr, epoch)
		if gotTa.Cmp(expectTa) != 0 {
			err = fmt.Errorf("address %v thawing assets not expected", addr)
			return
		}
		delete(expect, addr)
	})
	if err != nil {
		return err
	}
	if len(expect) != 0 {
		return fmt.Errorf("thawing size not expected")
	}
	return nil
}

// randomMarkThawAddresses randomly mark the thawing address with a random value value,
// It also add the frozen assets and then commit to statedb.
// Return the thawing address to value field.
func randomMarkThawAddresses(stateDB stateDB, addresses []common.Address, epoch int64) map[common.Address]common.BigInt {
	m := make(map[common.Address]common.BigInt)
	for _, addr := range addresses {
		ta := common.RandomBigInt()
		markThawingAddressAndValue(stateDB, addr, epoch, ta)
		addFrozenAssets(stateDB, addr, ta)
		m[addr] = ta
	}
	stateDB.IntermediateRoot(true)
	return m
}
