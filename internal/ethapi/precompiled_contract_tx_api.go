// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/rpc"
)

// PrivateStorageContractTxAPI exposes the storage contract tx methods for the RPC interface
type PrivateStorageContractTxAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPrivateStorageContractTxAPI creates a private RPC service with methods specific for storage contract tx.
func NewPrivateStorageContractTxAPI(b Backend, nonceLock *AddrLocker) *PrivateStorageContractTxAPI {
	return &PrivateStorageContractTxAPI{b, nonceLock}
}

// SendHostAnnounceTX submit a host announce tx to txpool, only for outer request, need to open cmd and RPC API
func (psc *PrivateStorageContractTxAPI) SendHostAnnounceTX(from common.Address) (common.Hash, error) {
	hostEnodeURL := psc.b.GetHostEnodeURL()
	hostAnnouncement := types.HostAnnouncement{
		NetAddress: hostEnodeURL,
	}

	hash := hostAnnouncement.RLPHash()
	sign, err := psc.b.SignByNode(hash.Bytes())
	if err != nil {
		return common.Hash{}, err
	}
	hostAnnouncement.Signature = sign

	payload, err := rlp.EncodeToBytes(hostAnnouncement)
	if err != nil {
		return common.Hash{}, err
	}

	to := common.Address{}
	to.SetBytes([]byte{9})

	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, payload, nil, StorageContractTxGas)
	txHash, err := sendPrecompiledContractTx(ctx, psc.b, psc.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendContractCreateTX submit a storage contract creation tx, generally triggered in ContractCreate, not for outer request
func (psc *PrivateStorageContractTxAPI) SendContractCreateTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{10})
	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, input, nil, StorageContractTxGas)
	txHash, err := sendPrecompiledContractTx(ctx, psc.b, psc.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendContractRevisionTX submit a storage contract revision tx, only triggered when host received consensus change, not for outer request
func (psc *PrivateStorageContractTxAPI) SendContractRevisionTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{11})
	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, input, nil, StorageContractTxGas)
	txHash, err := sendPrecompiledContractTx(ctx, psc.b, psc.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendStorageProofTX submit a storage proof tx, only triggered when host received consensus change, not for outer request
func (psc *PrivateStorageContractTxAPI) SendStorageProofTX(from common.Address, input []byte) (common.Hash, error) {
	to := common.Address{}
	to.SetBytes([]byte{12})
	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, input, nil, StorageContractTxGas)
	txHash, err := sendPrecompiledContractTx(ctx, psc.b, psc.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// PublicDposTxAPI exposes the dpos tx methods for the RPC interface
type PublicDposTxAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicDposTxAPI construct a PublicDposTxAPI object
func NewPublicDposTxAPI(b Backend, nonceLock *AddrLocker) *PublicDposTxAPI {
	return &PublicDposTxAPI{b, nonceLock}
}

// SendApplyCandidateTx submit a apply candidate tx.
// the parameter ratio is the award distribution ratio that candidate state.
func (pd *PublicDposTxAPI) SendApplyCandidateTx(fields map[string]string) (common.Hash, error) {
	to := vm.ApplyCandidateContractAddress
	ctx := context.Background()

	stateDB, _, err := pd.b.StateAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return common.Hash{}, err
	}

	// parse precompile contract tx args
	args, err := ParseAndValidateCandidateApplyTxArgs(to, DposTxGas, fields, stateDB, pd.b.AccountManager())
	if err != nil {
		return common.Hash{}, err
	}

	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendCancelCandidateTx submit a cancel candidate tx
func (pd *PublicDposTxAPI) SendCancelCandidateTx(from common.Address) (common.Hash, error) {
	to := vm.CancelCandidateContractAddress
	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, nil, nil, DposTxGas)

	// get the latest block header
	header, err := pd.b.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if header == nil || err != nil {
		return common.Hash{}, err
	}

	// check if the address is the candidate address
	if !dpos.IsCandidate(args.From, header, pd.b.ChainDb()) {
		return common.Hash{}, ErrNotCandidate
	}

	// send contract transaction
	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendVoteTx submit a vote tx
func (pd *PublicDposTxAPI) SendVoteTx(fields map[string]string) (common.Hash, error) {
	to := vm.VoteContractAddress
	ctx := context.Background()

	stateDB, _, err := pd.b.StateAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return common.Hash{}, err
	}

	// parse precompile contract tx args
	args, err := ParseAndValidateVoteTxArgs(to, DposTxGas, fields, stateDB, pd.b.AccountManager())
	if err != nil {
		return common.Hash{}, err
	}

	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// SendCancelVoteTx submit a cancel vote tx
func (pd *PublicDposTxAPI) SendCancelVoteTx(from common.Address) (common.Hash, error) {
	to := vm.CancelVoteContractAddress
	ctx := context.Background()

	// construct args
	args := NewPrecompiledContractTxArgs(from, to, nil, nil, DposTxGas)

	// get the latest block header
	header, err := pd.b.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if header == nil || err != nil {
		return common.Hash{}, err
	}

	// check if the delegator has voted before
	if !dpos.HasVoted(args.From, header, pd.b.ChainDb()) {
		return common.Hash{}, fmt.Errorf("failed to send cancel vote transaction, %s has not voted before", args.From.String())
	}

	// send the contract transaction
	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

// sendPrecompiledContractTx send precompiled contract tx，mostly need from、to、value、input（rlp encoded）
//
// NOTE: this is general func, you can construct different args to send detailed tx, like host announce、form contract、contract revision、storage proof.
// Actually, it need to set different PrecompiledContractTxArgs, like from、to、value、input
func sendPrecompiledContractTx(ctx context.Context, b Backend, nonceLock *AddrLocker, args *PrecompiledContractTxArgs) (common.Hash, error) {

	// find the account of the address from
	account := accounts.Account{Address: args.From}
	wallet, err := b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	nonceLock.LockAddr(args.From)
	defer nonceLock.UnlockAddr(args.From)

	// construct tx
	tx, err := args.NewPrecompiledContractTx(ctx, b)
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

// PrecompiledContractTxArgs represents the arguments to submit a precompiled contract tx into the transaction pool.
type PrecompiledContractTxArgs struct {
	From     common.Address  `json:"from"`
	To       common.Address  `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	Value    *hexutil.Big    `json:"value"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	Input    *hexutil.Bytes  `json:"input"`
}

// NewPrecompiledContractTx construct precompiled contract tx with args
func (args *PrecompiledContractTxArgs) NewPrecompiledContractTx(ctx context.Context, b Backend) (*types.Transaction, error) {
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

	if args.To == (common.Address{}) {
		return nil, errors.New(`precompile contract tx without to`)
	}

	return types.NewTransaction(uint64(*args.Nonce), args.To, (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), *args.Input), nil
}

// NewPrecompiledContractTxArgs construct precompiled contract tx args
func NewPrecompiledContractTxArgs(from, to common.Address, input []byte, value *big.Int, gas uint64) *PrecompiledContractTxArgs {
	args := &PrecompiledContractTxArgs{
		From: from,
		To:   to,
	}

	if input != nil {
		args.Input = (*hexutil.Bytes)(&input)
	} else {
		args.Input = new(hexutil.Bytes)
	}

	args.Gas = new(hexutil.Uint64)
	*(*uint64)(args.Gas) = gas

	if value != nil {
		args.Value = (*hexutil.Big)(value)
	} else {
		args.Value = new(hexutil.Big)
	}

	return args
}
