// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"crypto/rand"
	"testing"

	"github.com/DxChainNetwork/godx/core/types"

	"github.com/DxChainNetwork/godx/ethdb"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
)

func TestMakeThawingAssetsKey(t *testing.T) {
	tests := []int64{
		1, 2, 3, 4, 5, 100,
	}
	res := make(map[common.Hash]struct{})
	for _, epoch := range tests {
		h := makeThawingAssetsKey(epoch)
		if _, exist := res[h]; exist {
			t.Fatal("key collision")
		}
		res[h] = struct{}{}
	}
}

func newStateAndDposContext() (*state.StateDB, *types.DposContext, error) {
	db := ethdb.NewMemDatabase()
	stateDB, err := newStateDB(db)
	if err != nil {
		return nil, nil, err
	}
	ctx, err := types.NewDposContext(types.NewDposDb(db))
	if err != nil {
		return nil, nil, err
	}
	return stateDB, ctx, nil
}

// newStateDBWithAccounts create a new state db with a number of created accounts. The accounts
// are addresses 0x000001 ~ 0x0000${NUM}
func newStateDBWithAccounts(db ethdb.Database, num int) (*state.StateDB, []common.Address, error) {
	stateDB, err := newStateDB(db)
	if err != nil {
		return nil, []common.Address{}, err
	}
	var addresses []common.Address
	for i := 0; i != num; i++ {
		address := randomAddress()
		stateDB.CreateAccount(address)
		stateDB.SetNonce(address, 1)
		addresses = append(addresses, address)
	}
	return stateDB, addresses, nil
}

// addAccountInState add an account in stateDB balance and frozenAssets
func addAccountInState(state *state.StateDB, addr common.Address, balance common.BigInt, frozenAssets common.BigInt) {
	state.CreateAccount(addr)
	state.SetBalance(addr, balance.BigIntPtr())
	SetFrozenAssets(state, addr, frozenAssets)
}

func newStateDB(db ethdb.Database) (*state.StateDB, error) {
	return state.New(common.Hash{}, state.NewDatabase(db))
}

func randomAddress() common.Address {
	var addr common.Address
	_, err := rand.Read(addr[:])
	if err != nil {
		panic("cannot randomize an address")
	}
	return addr
}
