// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package simulation

import (
	"math/big"

	godx "github.com/DxChainNetwork/godx"
	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/core/types"
)

type FakeWallet struct{}

// SignHash is the only method used in the negotiation protocol
func (fw *FakeWallet) SignHash(account accounts.Account, hash []byte) ([]byte, error) {
	return []byte("signature by some client or some host"), nil
}

func (fw *FakeWallet) URL() accounts.URL {
	return accounts.URL{}
}

func (fw *FakeWallet) Status() (string, error) {
	return "", nil
}

func (fw *FakeWallet) Open(passphrase string) error {
	return nil
}

func (fw *FakeWallet) Close() error {
	return nil
}

func (fw *FakeWallet) Accounts() []accounts.Account {
	return nil
}

func (fw *FakeWallet) Contains(account accounts.Account) bool {
	return true
}

func (fw *FakeWallet) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
	return accounts.Account{}, nil
}

func (fw *FakeWallet) SelfDerive(base accounts.DerivationPath, chain godx.ChainStateReader) {}

func (fw *FakeWallet) SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (fw *FakeWallet) SignHashWithPassphrase(account accounts.Account, passphrase string, hash []byte) ([]byte, error) {
	return nil, nil
}

func (fw *FakeWallet) SignTxWithPassphrase(account accounts.Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return nil, nil
}
