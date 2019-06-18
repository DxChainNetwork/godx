// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"context"
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core/types"
)

// PublicStorageContractTxAPI exposes the SendHostAnnounceTx methods for the RPC interface
type PublicStorageContractTxAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicStorageContractTxAPI creates a public RPC service with methods specific for storage contract tx.
func NewPublicStorageContractTxAPI(b Backend, nonceLock *AddrLocker) *PublicStorageContractTxAPI {
	return &PublicStorageContractTxAPI{b, nonceLock}
}

// send host announce tx, only for outer request, need to open cmd and RPC API
func (psc *PublicStorageContractTxAPI) SendHostAnnounceTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{9})
	ctx := context.Background()
	txHash, err := sendStorageContractTX(ctx, psc.b, psc.nonceLock, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// PrivateStorageContractTxAPI exposes the SendHostAnnounceTx methods for the RPC interface
type PrivateStorageContractTxAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPrivateStorageContractTxAPI creates a private RPC service with methods specific for storage contract tx.
func NewPrivateStorageContractTxAPI(b Backend, nonceLock *AddrLocker) *PrivateStorageContractTxAPI {
	return &PrivateStorageContractTxAPI{b, nonceLock}
}

// send form contract tx, generally triggered in ContractCreate, not for outer request
func (psc *PrivateStorageContractTxAPI) SendContractCreateTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{10})
	ctx := context.Background()
	txHash, err := sendStorageContractTX(ctx, psc.b, psc.nonceLock, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send contract revision tx, only triggered when host received consensus change, not for outer request
func (psc *PrivateStorageContractTxAPI) SendContractRevisionTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{11})
	ctx := context.Background()
	txHash, err := sendStorageContractTX(ctx, psc.b, psc.nonceLock, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send storage proof tx, only triggered when host received consensus change, not for outer request
func (psc *PrivateStorageContractTxAPI) SendStorageProofTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{12})
	ctx := context.Background()
	txHash, err := sendStorageContractTX(ctx, psc.b, psc.nonceLock, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send storage contract tx，only need from、to、input（rlp encoded）
//
// NOTE: this is general func, you can construct different args to send 4 type txs, like host announce、form contract、contract revision、storage proof.
// Actually, it need to set different SendStorageContractTxArgs, like from、to、input
func sendStorageContractTX(ctx context.Context, b Backend, nonceLock *AddrLocker, from, to common.Address, input []byte) (common.Hash, error) {

	// construct args
	args := SendStorageContractTxArgs{
		From: from,
		To:   to,
	}
	args.Input = (*hexutil.Bytes)(&input)

	// find the account of the address from
	account := accounts.Account{Address: args.From}
	wallet, err := b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	nonceLock.LockAddr(args.From)
	defer nonceLock.UnlockAddr(args.From)

	// construct tx
	tx, err := args.setDefaultsTX(ctx, b)
	if err != nil {
		return common.Hash{}, err
	}

	// get chain ID
	var chainID *big.Int
	if config := b.ChainConfig(); config.IsEIP155(b.CurrentBlock().Number()) {
		chainID = config.ChainID
	}

	// sign the tx by using from's wallet
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}

	// send signed tx to txpool
	if err := b.SendTx(ctx, signed); err != nil {
		return common.Hash{}, err
	}

	return signed.Hash(), nil
}

// SendTxArgs represents the arguments to submit a new transaction into the transaction pool.
type SendStorageContractTxArgs struct {
	From     common.Address  `json:"from"`
	To       common.Address  `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Input    *hexutil.Bytes  `json:"input"`
}

// construct tx with args
func (args *SendStorageContractTxArgs) setDefaultsTX(ctx context.Context, b Backend) (*types.Transaction, error) {
	args.Gas = new(hexutil.Uint64)
	*(*uint64)(args.Gas) = 90000

	price, err := b.SuggestPrice(ctx)
	if err != nil {
		return nil, err
	}
	args.GasPrice = (*hexutil.Big)(price)

	nonce, err := b.GetPoolNonce(ctx, args.From)
	if err != nil {
		return nil, err
	}
	args.Nonce = (*hexutil.Uint64)(&nonce)

	if args.To == (common.Address{}) || args.Input == nil {
		return nil, errors.New(`storage contract tx without to or input`)
	}

	return types.NewTransaction(uint64(*args.Nonce), args.To, nil, uint64(*args.Gas), (*big.Int)(args.GasPrice), *args.Input), nil
}
