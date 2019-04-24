package state

import (
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
)

//  ______ _______ _    _ ______ _____    _______ ______  _____ _______ _____
// |  ____|__   __| |  | |  ____|  __ \  |__   __|  ____|/ ____|__   __/ ____|
// | |__     | |  | |__| | |__  | |__) |    | |  | |__  | (___    | | | (___
// |  __|    | |  |  __  |  __| |  _  /     | |  |  __|  \___ \   | |  \___ \
// | |____   | |  | |  | | |____| | \ \     | |  | |____ ____) |  | |  ____) |
// |______|  |_|  |_|  |_|______|_|  \_\    |_|  |______|_____/   |_| |_____/

var addr = common.BytesToAddress([]byte("test"))

func create() (*ManagedState, *account) {
	statedb, _ := New(common.Hash{}, NewDatabase(ethdb.NewMemDatabase()))
	ms := ManageState(statedb)
	ms.StateDB.SetNonce(addr, 100)
	ms.accounts[addr] = newAccount(ms.StateDB.getStateObject(addr))
	return ms, ms.accounts[addr]
}

func TestNewNonce(t *testing.T) {
	ms, _ := create()

	nonce := ms.NewNonce(addr)
	if nonce != 100 {
		t.Error("expected nonce 100. got", nonce)
	}

	nonce = ms.NewNonce(addr)
	if nonce != 101 {
		t.Error("expected nonce 101. got", nonce)
	}
}

func TestRemove(t *testing.T) {
	ms, account := create()

	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)

	i := uint64(5)
	ms.RemoveNonce(addr, account.nstart+i)
	if len(account.nonces) != 5 {
		t.Error("expected", i, "'th index to be false")
	}
}

func TestReuse(t *testing.T) {
	ms, account := create()

	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)

	i := uint64(5)
	ms.RemoveNonce(addr, account.nstart+i)
	nonce := ms.NewNonce(addr)
	if nonce != 105 {
		t.Error("expected nonce to be 105. got", nonce)
	}
}

func TestRemoteNonceChange(t *testing.T) {
	ms, account := create()
	nn := make([]bool, 10)
	for i := range nn {
		nn[i] = true
	}
	account.nonces = append(account.nonces, nn...)
	ms.NewNonce(addr)

	ms.StateDB.stateObjects[addr].data.Nonce = 200
	nonce := ms.NewNonce(addr)
	if nonce != 200 {
		t.Error("expected nonce after remote update to be", 200, "got", nonce)
	}
	ms.NewNonce(addr)
	ms.NewNonce(addr)
	ms.NewNonce(addr)
	ms.StateDB.stateObjects[addr].data.Nonce = 200
	nonce = ms.NewNonce(addr)
	if nonce != 204 {
		t.Error("expected nonce after remote update to be", 204, "got", nonce)
	}
}

func TestSetNonce(t *testing.T) {
	ms, _ := create()

	var addr common.Address
	ms.SetNonce(addr, 10)

	if ms.GetNonce(addr) != 10 {
		t.Error("Expected nonce of 10, got", ms.GetNonce(addr))
	}

	addr[0] = 1
	ms.StateDB.SetNonce(addr, 1)

	if ms.GetNonce(addr) != 1 {
		t.Error("Expected nonce of 1, got", ms.GetNonce(addr))
	}
}
