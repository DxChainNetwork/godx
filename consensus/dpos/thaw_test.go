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
	state, err := newStateDBWithAccounts(db, num)
	if err != nil {
		t.Fatal(err)
	}
	// Mark 100 addresses as thaw in 2 epochs
	epoch1, epoch2 := int64(100), int64(101)
	m1, m2 := make(map[common.Address]common.BigInt), make(map[common.Address]common.BigInt)
	for i := 1; i <= num; i++ {
		addr := common.BytesToAddress([]byte{byte(i)})
		ta1, ta2 := common.RandomBigInt(), common.RandomBigInt()
		m1[addr], m2[addr] = ta1, ta2
		markThawingAddressAndValue(state, addr, epoch1, ta1)
		markThawingAddressAndValue(state, addr, epoch2, ta2)
	}

	state.IntermediateRoot(true)

	// check the two periods
	if err := checkThawingAddressAndValue(state, calcThawingEpoch(epoch1), m1); err != nil {
		t.Error("period1: ", err)
	}
	if err := checkThawingAddressAndValue(state, calcThawingEpoch(epoch2), m2); err != nil {
		t.Error("period2: ", err)
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
