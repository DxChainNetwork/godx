// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/rpc"
)

// PublicDposTxAPI exposes the dpos tx methods for the RPC interface
type PublicDposTxAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// CandidateInfo stores detailed candidate information
type CandidateInfo struct {
	Candidate   common.Address `json:"candidate"`
	Deposit     common.BigInt  `json:"deposit"`
	Votes       common.BigInt  `json:"votes"`
	RewardRatio uint64         `json:"reward_distribution"`
}

// ValidatorInfo stores detailed validator information
type ValidatorInfo struct {
	Validator   common.Address `json:"validator"`
	Votes       common.BigInt  `json:"votes"`
	EpochID     int64          `json:"current_epoch"`
	MinedBlocks int64          `json:"epoch_mined_blocks"`
	RewardRatio uint64         `json:"reward_distribution"`
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

	if err := pd.validateCancelCandidateRequest(ctx, from); err != nil {
		return common.Hash{}, err
	}
	// construct args
	args := NewPrecompiledContractTxArgs(from, to, nil, nil, DposTxGas)

	// send contract transaction
	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

func (pd *PublicDposTxAPI) validateCancelCandidateRequest(ctx context.Context, addr common.Address) error {
	_, dposCtx, _, err := pd.b.StateDposCtxAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return err
	}
	if dposCtx == nil {
		return errors.New("unknown block number")
	}
	isCand, err := dpos.IsCandidate(dposCtx, addr)
	if err != nil {
		return err
	}
	if !isCand {
		return ErrNotCandidate
	}
	return nil
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

	if err := pd.validateCancelVoteRequest(ctx, from); err != nil {
		return common.Hash{}, err
	}
	// construct args
	args := NewPrecompiledContractTxArgs(from, to, nil, nil, DposTxGas)

	// send the contract transaction
	txHash, err := sendPrecompiledContractTx(ctx, pd.b, pd.nonceLock, args)
	if err != nil {
		return common.Hash{}, err
	}
	return txHash, nil
}

func (pd *PublicDposTxAPI) validateCancelVoteRequest(ctx context.Context, addr common.Address) error {
	_, dposCtx, _, err := pd.b.StateDposCtxAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return err
	}
	if dposCtx == nil {
		return ErrUnknownBlockNumber
	}
	hasVoted, err := dpos.HasVoted(dposCtx, addr)
	if err != nil {
		return err
	}
	if !hasVoted {
		return fmt.Errorf("failed to send cancel vote transaction, %x has not voted before", addr)
	}
	return nil
}

// Validators will return a list of validators based on the blockNumber provided
func (pd *PublicDposTxAPI) Validators(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	num := blockNumberConverter(blockNr)
	ctx := context.Background()
	_, dposCtx, _, err := pd.b.StateDposCtxAndHeaderByNumber(ctx, num)
	if err != nil {
		return []common.Address{}, err
	}
	if dposCtx == nil {
		return []common.Address{}, ErrUnknownBlockNumber
	}
	return dposCtx.GetValidators()
}

// Validator return a detailed info about the validator address provided.
func (pd *PublicDposTxAPI) Validator(address common.Address, blockNr *rpc.BlockNumber) (ValidatorInfo, error) {
	num := blockNumberConverter(blockNr)
	ctx := context.Background()
	_, dposCtx, statedb, err := pd.b.StateDposCtxAndHeaderByNumber(ctx, num)
	if err != nil {
		return ValidatorInfo{}, err
	}
	if dposCtx == nil || statedb == nil {
		return ValidatorInfo{}, ErrUnknownBlockNumber
	}

}

func blockNumberConverter(blockNumber *rpc.BlockNumber) rpc.BlockNumber {
	if blockNumber == nil {
		return rpc.LatestBlockNumber
	}
	return *blockNumber
}
