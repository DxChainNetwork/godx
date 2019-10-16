// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package fakeproto

import "github.com/DxChainNetwork/godx/accounts"

type FakeAccountManager struct{}

func (fa *FakeAccountManager) Find(accounts.Account) (accounts.Wallet, error) {
	return &FakeWallet{}, nil
}
