// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storage

import (
	"context"
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/internal/ethapi"
)

// PublicTransactionPoolAPI exposes methods for the RPC interface
type ContractTxAPI struct {
	b         ClientBackend
	nonceLock *ethapi.AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewContractTxAPI(b ClientBackend) *ContractTxAPI {
	nonceLock := new(ethapi.AddrLocker)
	return &ContractTxAPI{b, nonceLock}
}

//func NewStorageContractTxAPI(b ClientBackend) *ContractTxAPI {
//	nonceLock := new(ethapi.AddrLocker)
//	return &ContractTxAPI{b, nonceLock}
//}

// send storage contract tx，only need from、to、input（rlp encoded）
//
// NOTE: this is general func, you can construct different args to send 4 type txs, like host announce、form contract、contract revision、storage proof.
// Actually, it need to set different SendStorageContractTxArgs, like from、to、input
func (sc *ContractTxAPI) sendStorageContractTX(ctx context.Context, from, to common.Address, input []byte) (common.Hash, error) {

	// construct args
	args := SendStorageContractTxArgs{
		From: from,
		To:   to,
	}
	args.Input = (*hexutil.Bytes)(&input)

	// find the account of the address from
	account := accounts.Account{Address: args.From}
	wallet, err := sc.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	sc.nonceLock.LockAddr(args.From)
	defer sc.nonceLock.UnlockAddr(args.From)

	// construct tx
	tx, err := args.setDefaultsTX(ctx, sc.b)
	if err != nil {
		return common.Hash{}, err
	}

	// get chain ID
	var chainID *big.Int
	if config := sc.b.ChainConfig(); config.IsEIP155(sc.b.CurrentBlock().Number()) {
		chainID = config.ChainID
	}

	// sign the tx by using from's wallet
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}

	// send tx to txpool
	if err := sc.b.SendTx(ctx, tx); err != nil {
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
func (args *SendStorageContractTxArgs) setDefaultsTX(ctx context.Context, b ClientBackend) (*types.Transaction, error) {
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

// send form contract tx, generally triggered in ContractCreate, not for outer request
func SendFormContractTX(b ClientBackend, from common.Address, input []byte) (common.Hash, error) {
	scTxAPI := NewContractTxAPI(b)
	to := common.Address{}
	to.SetBytes([]byte{10})
	ctx := context.Background()
	txHash, err := scTxAPI.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send host announce tx, only for outer request, need to open cmd and RPC API
func (sc *ContractTxAPI) SendHostAnnounceTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{9})
	ctx := context.Background()
	txHash, err := sc.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send contract revision tx, only triggered when host received consensus change, not for outer request
func SendContractRevisionTX(b ClientBackend, from common.Address, input []byte) (common.Hash, error) {
	scTxAPI := NewContractTxAPI(b)
	to := common.Address{}
	to.SetBytes([]byte{11})
	ctx := context.Background()
	txHash, err := scTxAPI.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send storage proof tx, only triggered when host received consensus change, not for outer request
func SendStorageProofTX(b ClientBackend, from common.Address, input []byte) (common.Hash, error) {
	scTxAPI := NewContractTxAPI(b)
	to := common.Address{}
	to.SetBytes([]byte{12})
	ctx := context.Background()
	txHash, err := scTxAPI.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type ContractHostTxAPI struct {
	eth       EthBackend
	nonceLock *ethapi.AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewContractHostTxAPI(eth EthBackend) *ContractHostTxAPI {
	nonceLock := new(ethapi.AddrLocker)
	return &ContractHostTxAPI{eth, nonceLock}
}

// send contract revision tx, only triggered when host received consensus change, not for outer request
func SendContractHostRevisionTX(eth EthBackend, from common.Address, input []byte) (common.Hash, error) {
	scTxAPI := NewContractHostTxAPI(eth)
	to := common.Address{}
	to.SetBytes([]byte{11})
	ctx := context.Background()
	txHash, err := scTxAPI.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// send storage proof tx, only triggered when host received consensus change, not for outer request
func SendStorageHostProofTX(eth EthBackend, from common.Address, input []byte) (common.Hash, error) {
	scTxAPI := NewContractHostTxAPI(eth)
	to := common.Address{}
	to.SetBytes([]byte{12})
	ctx := context.Background()
	txHash, err := scTxAPI.sendStorageContractTX(ctx, from, to, input)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// construct tx with args
func (args *SendStorageContractTxArgs) setHostDefaultsTX(ctx context.Context, eth EthBackend) (*types.Transaction, error) {
	args.Gas = new(hexutil.Uint64)
	*(*uint64)(args.Gas) = 90000

	price, err := eth.SuggestPrice(ctx)
	if err != nil {
		return nil, err
	}
	args.GasPrice = (*hexutil.Big)(price)

	nonce, err := eth.GetPoolNonce(ctx, args.From)
	if err != nil {
		return nil, err
	}
	args.Nonce = (*hexutil.Uint64)(&nonce)

	if args.To == (common.Address{}) || args.Input == nil {
		return nil, errors.New(`storage contract tx without to or input`)
	}

	return types.NewTransaction(uint64(*args.Nonce), args.To, nil, uint64(*args.Gas), (*big.Int)(args.GasPrice), *args.Input), nil
}

func (sc *ContractHostTxAPI) sendStorageContractTX(ctx context.Context, from, to common.Address, input []byte) (common.Hash, error) {

	// construct args
	args := SendStorageContractTxArgs{
		From: from,
		To:   to,
	}
	args.Input = (*hexutil.Bytes)(&input)

	// find the account of the address from
	account := accounts.Account{Address: args.From}
	wallet, err := sc.eth.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	sc.nonceLock.LockAddr(args.From)
	defer sc.nonceLock.UnlockAddr(args.From)

	// construct tx
	tx, err := args.setHostDefaultsTX(ctx, sc.eth)
	if err != nil {
		return common.Hash{}, err
	}

	// get chain ID
	var chainID *big.Int
	if config := sc.eth.ChainConfig(); config.IsEIP155(sc.eth.CurrentBlock().Number()) {
		chainID = config.ChainID
	}

	// sign the tx by using from's wallet
	signed, err := wallet.SignTx(account, tx, chainID)
	if err != nil {
		return common.Hash{}, err
	}

	// send tx to txpool
	if err := sc.eth.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}

	return signed.Hash(), nil
}
