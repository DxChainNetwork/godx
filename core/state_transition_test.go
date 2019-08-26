package core

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/params"
)

func TestCheckDposOperationTx(t *testing.T) {
	prvkey, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("failed to generate public/private key pairs,error: %v", err)
	}

	addr := crypto.PubkeyToAddress(prvkey.PublicKey)
	signer := types.NewEIP155Signer(new(big.Int).SetInt64(1))
	evm, err := mockEvmAndState(addr)
	if err != nil {
		t.Errorf("failed to mock evm,error: %v", err)
	}

	// check candidate tx
	candidateMsg, err := mockMessage(common.BytesToAddress([]byte{13}), signer, prvkey, nil)
	if err != nil {
		t.Errorf("failed to mock candidate tx message,error: %v", err)
	}

	err = CheckDposOperationTx(evm, candidateMsg)
	if err != nil {
		t.Errorf("failed to check candiadte tx,error: %v", err)
	}

	// check cancel candidate tx
	evm.StateDB.SetState(addr, common.BytesToHash([]byte("candidate-deposit")), common.BytesToHash([]byte{0x34}))
	candidateCancelMsg, err := mockMessage(common.BytesToAddress([]byte{14}), signer, prvkey, nil)
	if err != nil {
		t.Errorf("failed to mock candidate cancel tx message,error: %v", err)
	}

	err = CheckDposOperationTx(evm, candidateCancelMsg)
	if err != nil {
		t.Errorf("failed to check candiadte cancel tx,error: %v", err)
	}

	// check vote tx
	voteMsg, err := mockMessage(common.BytesToAddress([]byte{15}), signer, prvkey, []byte{0x56})
	if err != nil {
		t.Errorf("failed to mock vote tx message,error: %v", err)
	}

	err = CheckDposOperationTx(evm, voteMsg)
	if err != nil {
		t.Errorf("failed to check vote tx,error: %v", err)
	}

	// check cancel vote tx
	evm.StateDB.SetState(addr, common.BytesToHash([]byte("vote-deposit")), common.BytesToHash([]byte{0x78}))
	voteCancelMsg, err := mockMessage(common.BytesToAddress([]byte{16}), signer, prvkey, []byte{0x56})
	if err != nil {
		t.Errorf("failed to mock cancel vote tx message,error: %v", err)
	}

	err = CheckDposOperationTx(evm, voteCancelMsg)
	if err != nil {
		t.Errorf("failed to check cancel vote tx,error: %v", err)
	}
}

func mockMessage(to common.Address, signer types.Signer, prvkey *ecdsa.PrivateKey, data []byte) (*types.Message, error) {
	amount := new(big.Int).SetInt64(1e10)
	gasLimit := uint64(1e9)
	gas := new(big.Int).SetInt64(1e6)
	tx := types.NewTransaction(1, to, amount, gasLimit, gas, data)
	tx, err := types.SignTx(tx, signer, prvkey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign candidate tx,error: %v", err)
	}

	msg, err := tx.AsMessage(signer)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tx to message,error: %v", err)
	}

	return &msg, nil
}

// AccountInfo is an account in the state
type AccountInfo struct {
	Storage map[common.Hash]common.Hash
	Balance *big.Int
	Nonce   uint64
}

func mockEvmAndState(addr common.Address) (*vm.EVM, error) {

	// mock account
	account := AccountInfo{
		Nonce:   1,
		Balance: new(big.Int).SetInt64(2e18),
		Storage: make(map[common.Hash]common.Hash),
	}

	// mock state
	sdb := state.NewDatabase(ethdb.NewMemDatabase())
	stateDB, _ := state.New(common.Hash{}, sdb)
	stateDB.SetNonce(addr, account.Nonce)
	stateDB.SetBalance(addr, account.Balance)
	root, _ := stateDB.Commit(false)
	stateDB, _ = state.New(root, sdb)

	// mock evm
	evm := vm.NewEVM(vm.Context{}, stateDB, params.MainnetChainConfig, vm.Config{})
	return evm, nil
}
