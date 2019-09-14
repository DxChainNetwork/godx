// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package eth

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus/dpos"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/trie"
)

type PublicDposAPI struct {
	e *Ethereum
}

type CandidateInfo struct {
	Candidate          common.Address
	Deposit            common.BigInt
	Votes              common.BigInt
	RewardDistribution common.BigInt
}

type ValidatorInfo struct {
	Validator          common.Address
	Votes              common.BigInt
	MinedBlocks        int64
	RewardDistribution common.BigInt
}

func NewPublicDposAPI(e *Ethereum) *PublicDposAPI {
	return &PublicDposAPI{
		e: e,
	}
}

func (d *PublicDposAPI) Validators(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	// re-create the tree based on the chain database
	triedb := trie.NewDatabase(d.e.ChainDb())
	epochTrie, err := types.NewEpochTrie(header.DposContext.EpochRoot, triedb)
	if err != nil {
		return nil, fmt.Errorf("failed to recover the epochTrie based on the root: %s", err.Error())
	}

	// based on the epochTrie, get the validators
	dposContext := types.DposContext{}
	dposContext.SetEpoch(epochTrie)
	return dposContext.GetValidators()
}

func (d *PublicDposAPI) Validator(validatorAddress common.Address) (ValidatorInfo, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return ValidatorInfo{}, err
	}

	// get the validator votes
	validatorVotes := statedb.GetState(validatorAddress, dpos.KeyTotalVoteWeight).Big()

	// get the validator reward distribution
	rewardDistribution := statedb.GetState(validatorAddress, dpos.KeyRewardRatioNumerator)

	// get the validator minedBlocks
	triedb := trie.NewDatabase(d.e.ChainDb())
	minedCntTrie, err := types.NewMinedCntTrie(header.DposContext.MinedCntRoot, triedb)
	if err != nil {
		return ValidatorInfo{}, fmt.Errorf("failed to recover the minedCntTrie based on the root: %s", err.Error())
	}
	epochID := dpos.CalculateEpochID(header.Time.Int64())
	minedCount := getMinedCnt(epochID, validatorAddress, minedCntTrie)

	return ValidatorInfo{
		Validator:          validatorAddress,
		Votes:              common.PtrBigInt(validatorVotes),
		RewardDistribution: hashToRewardRatioNumerator(rewardDistribution),
		MinedBlocks:        minedCount,
	}, nil
}

func (d *PublicDposAPI) Candidates(blockNr *rpc.BlockNumber) ([]common.Address, error) {
	// get the block header information based on the block number
	header, err := getHeaderBasedOnNumber(blockNr, d.e)
	if err != nil {
		return nil, err
	}

	// re-create the tree based on the chain database
	triedb := trie.NewDatabase(d.e.ChainDb())
	candidateTrie, err := types.NewCandidateTrie(header.DposContext.CandidateRoot, triedb)
	if err != nil {
		return nil, fmt.Errorf("failed to recover the candidateTrie based on the root: %s", err.Error())
	}

	// iterate through the candidateTrie, get each candidate
	var candidates []common.Address
	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	for iterCandidate.Next() {
		candidateAddr := common.BytesToAddress(iterCandidate.Value)
		candidates = append(candidates, candidateAddr)
	}

	// return candidates
	return candidates, nil
}

func (d *PublicDposAPI) Candidate(candidateAddress common.Address) (CandidateInfo, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return CandidateInfo{}, err
	}

	// get the candidate deposit
	candidateDeposit := statedb.GetState(candidateAddress, dpos.KeyCandidateDeposit).Big()

	// get the number of votes that candidate get
	candidateVotes := statedb.GetState(candidateAddress, dpos.KeyTotalVoteWeight).Big()

	// get the reward distribution plan
	rewardDistribution := statedb.GetState(candidateAddress, dpos.KeyRewardRatioNumerator)

	return CandidateInfo{
		Candidate:          candidateAddress,
		Deposit:            common.PtrBigInt(candidateDeposit),
		Votes:              common.PtrBigInt(candidateVotes),
		RewardDistribution: hashToRewardRatioNumerator(rewardDistribution),
	}, nil

}

// CandidateDeposit is used to check how much deposit a candidate has put in
func (d *PublicDposAPI) CandidateDeposit(candidateAddress common.Address) (*big.Int, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	// get the candidate deposit from the stateDB
	candidateDepositHash := statedb.GetState(candidateAddress, dpos.KeyCandidateDeposit)
	return candidateDepositHash.Big(), nil
}

func (d *PublicDposAPI) VoteDeposit(voteAddress common.Address) (*big.Int, error) {
	// based on the block header root, get the statedb
	header := d.e.BlockChain().CurrentHeader()
	statedb, err := d.e.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	// get the vote deposit from the stateDB
	voteDepositHash := statedb.GetState(voteAddress, dpos.KeyVoteDeposit)
	return voteDepositHash.Big(), nil
}

func getHeaderBasedOnNumber(blockNr *rpc.BlockNumber, e *Ethereum) (*types.Header, error) {
	// based on the block number, get the block header
	var header *types.Header
	if blockNr == nil {
		header = e.BlockChain().CurrentHeader()
	} else {
		header = e.BlockChain().GetHeaderByNumber(uint64(blockNr.Int64()))
	}

	// sanity check
	if header == nil {
		return nil, fmt.Errorf("unknown block")
	}

	// return
	return header, nil
}

// hashToRewardRatioNumerator return the customized block reward ratio numerator
func hashToRewardRatioNumerator(h common.Hash) common.BigInt {
	v := h.Bytes()
	return common.NewBigIntUint64(uint64(v[len(v)-1]))
}

func getMinedCnt(epochID int64, validator common.Address, minedCntTrie *trie.Trie) int64 {
	// form the key
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := minedCntTrie.Get(append(key, validator.Bytes()...))
	if cntBytes == nil {
		return 0
	} else {
		return int64(binary.BigEndian.Uint64(cntBytes))
	}
}
