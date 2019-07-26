// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package maintenance

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
)

var (
	contractOriginbal      = new(big.Int).SetInt64(10000000)
	clientAndHostOriginBal = new(big.Int).SetInt64(1000000000000000000)
	clientMpo              = new(big.Int).SetInt64(2000000)
	hostMpo                = new(big.Int).SetInt64(1000000)
)

// AccountInfo is an account in the state
type AccountInfo struct {
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance *big.Int                    `json:"balance" gencodec:"required"`
	Nonce   uint64                      `json:"nonce,omitempty"`
}

type AccountAlloc map[common.Address]AccountInfo

type PrivkeyAddress struct {
	Privkey *ecdsa.PrivateKey
	Address common.Address
}

func TestMaintenanceMissedProof(t *testing.T) {

	// mock evm, state, client and host address ...
	prvAndAddresses, err := mockClientAndHostAddress()
	if err != nil {
		t.Error(err)
	}
	clientAddress := prvAndAddresses[0].Address
	hostAddress := prvAndAddresses[1].Address

	// mock evm
	accounts := mockAccountAlloc([]common.Address{clientAddress, hostAddress})
	stateDB := mockState(ethdb.NewMemDatabase(), accounts)

	// mock write missed storage proof
	contractAddr := mockMissedStorageProof(1000, stateDB, prvAndAddresses)

	MaintenanceMissedProof(1000, stateDB)

	// check balance
	afterContractBal := stateDB.GetBalance(contractAddr)
	if afterContractBal.Int64() != contractOriginbal.Int64()-clientMpo.Int64()-hostMpo.Int64() {
		t.Errorf("failed to effect status account, wanted %d, getted %d", contractOriginbal.Int64()-clientMpo.Int64()-hostMpo.Int64(), afterContractBal.Int64())
	}

	afterClientBal := stateDB.GetBalance(clientAddress)
	if afterClientBal.Int64() != clientAndHostOriginBal.Int64()+clientMpo.Int64() {
		t.Errorf("failed to effect client missed proof, wanted %d, getted %d", clientAndHostOriginBal.Int64()+clientMpo.Int64(), afterClientBal.Int64())
	}

	afterHostBal := stateDB.GetBalance(hostAddress)
	if afterHostBal.Int64() != clientAndHostOriginBal.Int64()+hostMpo.Int64() {
		t.Errorf("failed to effect host missed proof, wanted %d, getted %d", clientAndHostOriginBal.Int64()+hostMpo.Int64(), afterHostBal.Int64())
	}
}

// mock that have a missed proof at the given height
func mockMissedStorageProof(height uint64, state *state.StateDB, prvAndAddresses []PrivkeyAddress) common.Address {
	windowEndStr := strconv.FormatUint(height, 10)
	statusAddr := common.BytesToAddress([]byte(StrPrefixExpSC + windowEndStr))
	state.CreateAccount(statusAddr)
	state.SetNonce(statusAddr, 1)

	contractID := common.HexToHash("0x5e109495581395e5d86c377efb05c2aef6ab6f2046f1bd7336e1ab1bfd96b6ed")
	contractAddr := common.BytesToAddress(contractID[12:])
	state.CreateAccount(contractAddr)
	state.AddBalance(contractAddr, contractOriginbal)
	state.SetNonce(contractAddr, 1)

	notProofedStatus := append(NotProofedStatus, contractAddr[:]...)
	state.SetState(statusAddr, contractID, common.BytesToHash(notProofedStatus))
	state.SetState(contractAddr, KeyClientAddress, common.BytesToHash(prvAndAddresses[0].Address.Bytes()))
	state.SetState(contractAddr, KeyHostAddress, common.BytesToHash(prvAndAddresses[1].Address.Bytes()))
	state.SetState(contractAddr, KeyClientMissedProofOutput, common.BytesToHash(clientMpo.Bytes()))
	state.SetState(contractAddr, KeyHostMissedProofOutput, common.BytesToHash(hostMpo.Bytes()))
	state.Commit(true)

	return contractAddr
}

func mockClientAndHostAddress() ([]PrivkeyAddress, error) {
	// mock host address
	prvKeyHost, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate public/private key pairs for storage host: %v", err)
	}

	// mock client address
	prvKeyClient, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate public/private key pairs for storage client: %v", err)
	}

	hostAddress := crypto.PubkeyToAddress(prvKeyHost.PublicKey)
	clientAddress := crypto.PubkeyToAddress(prvKeyClient.PublicKey)
	return []PrivkeyAddress{{prvKeyClient, clientAddress}, {prvKeyHost, hostAddress}}, nil
}

func mockAccountAlloc(addrs []common.Address) AccountAlloc {
	accounts := make(AccountAlloc)
	for _, addr := range addrs {
		accounts[addr] = AccountInfo{
			Nonce:   1,
			Balance: clientAndHostOriginBal,
			Storage: make(map[common.Hash]common.Hash),
		}
	}
	return accounts
}

func mockState(db ethdb.Database, accounts AccountAlloc) *state.StateDB {
	sdb := state.NewDatabase(db)
	stateDB, _ := state.New(common.Hash{}, sdb)
	for addr, a := range accounts {
		stateDB.SetNonce(addr, a.Nonce)
		stateDB.SetBalance(addr, a.Balance)
		for k, v := range a.Storage {
			stateDB.SetState(addr, k, v)
		}
	}
	root, _ := stateDB.Commit(true)
	stateDB, _ = state.New(root, sdb)
	return stateDB
}
