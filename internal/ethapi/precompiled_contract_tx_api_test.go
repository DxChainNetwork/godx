// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/ethdb"
)

var (
	addrs = []common.Address{
		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"),
		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"),
		common.HexToAddress("0x6de5af2854ad0f9d5b7d0b749fffa3f7a57d7b9d"),
	}

	tests = []struct {
		name    string
		fn      func(stateDB *state.StateDB) error
		wantErr error
	}{
		{
			name: "balance not enough candidate threshold",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{13})
				value := new(big.Int).SetInt64(1e18)
				args := NewPrecompiledContractTxArgs(addrs[0], to, nil, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrBalanceNotEnoughCandidateThreshold,
		},
		{
			name: "deposit value not suitable",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{13})
				value := new(big.Int).SetInt64(1e18)
				voteDeposit := new(big.Int).SetInt64(1.5e18)
				stateDB.SetState(addrs[1], dpos.KeyVoteDeposit, common.BigToHash(voteDeposit))
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrDepositValueNotSuitable,
		},
		{
			name: "deposit value too low to apply candidate",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{13})
				value := new(big.Int).SetInt64(1e17)
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrCandidateDepositTooLow,
		},
		{
			name: "not become candidate yet",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{14})
				value := new(big.Int).SetInt64(1e18)
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrNotCandidate,
		},
		{
			name: "has not voted before",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{16})
				args := NewPrecompiledContractTxArgs(addrs[0], to, nil, nil, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrHasNotVote,
		},
		{
			name: "invalid precompiled contract address",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{17})
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, nil, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: ErrUnknownPrecompileContractAddress,
		},
		{
			name: "success to check apply candidate tx",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{13})
				value := new(big.Int).SetInt64(1e18)
				args := NewPrecompiledContractTxArgs(addrs[2], to, nil, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: nil,
		},
		{
			name: "success to check cancel candidate tx",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{14})
				canDeposit := new(big.Int).SetInt64(1.5e18)
				stateDB.SetState(addrs[1], dpos.KeyCandidateDeposit, common.BigToHash(canDeposit))
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, nil, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: nil,
		},
		{
			name: "success to check vote tx",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{15})
				data := []byte{0x23, 0x45}
				value := new(big.Int).SetInt64(1e18)
				args := NewPrecompiledContractTxArgs(addrs[2], to, data, value, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: nil,
		},
		{
			name: "success to check cancel vote tx",
			fn: func(stateDB *state.StateDB) error {
				to := common.BytesToAddress([]byte{16})
				voteDeposit := new(big.Int).SetInt64(1.5e18)
				stateDB.SetState(addrs[1], dpos.KeyVoteDeposit, common.BigToHash(voteDeposit))
				args := NewPrecompiledContractTxArgs(addrs[1], to, nil, nil, DposTxGas)
				return CheckDposOperationTx(stateDB, args)
			},
			wantErr: nil,
		},
	}
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

func TestCheckDposOperationTx(t *testing.T) {
	stateDB, err := mockState(addrs)
	if err != nil {
		t.Fatalf("failed to mock evm,error: %v", err)
	}

	for _, test := range tests {
		err := test.fn(stateDB)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("%s: returned error %v, want %v", test.name, err, test.wantErr)
		}
	}
}
