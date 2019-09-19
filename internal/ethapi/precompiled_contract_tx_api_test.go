// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/ethdb"
)

// mock state
func mockState(addrs []common.Address) (*state.StateDB, error) {
	sdb := state.NewDatabase(ethdb.NewMemDatabase())
	stateDB, err := state.New(common.Hash{}, sdb)
	if err != nil {
		return nil, err
	}

	stateDB.SetNonce(addrs[0], 1)
	stateDB.SetBalance(addrs[0], new(big.Int).SetInt64(2e17))

	stateDB.SetNonce(addrs[1], 1)
	stateDB.SetBalance(addrs[1], new(big.Int).SetInt64(2e18))

	stateDB.SetNonce(addrs[2], 1)
	stateDB.SetBalance(addrs[2], new(big.Int).SetInt64(2e18))
	root, err := stateDB.Commit(false)
	if err != nil {
		return nil, err
	}

	stateDB, err = state.New(root, sdb)
	if err != nil {
		return nil, err
	}

	return stateDB, nil
}
