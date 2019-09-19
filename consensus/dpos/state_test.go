// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dpos

import (
	"testing"

	"github.com/DxChainNetwork/godx/ethdb"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
)

type testOperation interface {
	execute()
	check() error
}

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

// newStateDBWithAccounts create a new state db with a number of created accounts. The accounts
// are addresses 0x000001 ~ 0x0000${NUM}
func newStateDBWithAccounts(db ethdb.Database, num int) (*state.StateDB, error) {
	stateDB, err := newStateDB(db)
	if err != nil {
		return nil, err
	}
	for i := 0; i != num; i++ {
		address := common.BytesToAddress([]byte{byte(i + 1)})
		stateDB.CreateAccount(address)
		stateDB.SetNonce(address, 1)
	}
	return stateDB, nil
}

func newStateDB(db ethdb.Database) (*state.StateDB, error) {
	return state.New(common.Hash{}, state.NewDatabase(db))
}
