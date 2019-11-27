// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package types

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/trie"
	"golang.org/x/crypto/sha3"
)

// DposContext wraps 5 tries to store data needed in dpos consensus
type DposContext struct {
	epochTrie     DposTrie
	delegateTrie  DposTrie
	voteTrie      DposTrie
	candidateTrie DposTrie
	minedCntTrie  DposTrie

	db DposDatabase
}

var (
	keyValidator = []byte("validator")
)

// DposDatabase is the underlying database under the dposCtx.
// Implemented by both FullDposDatabase and light.dposOdrDatabase
type DposDatabase interface {
	OpenEpochTrie(root common.Hash) (DposTrie, error)
	OpenDelegateTrie(root common.Hash) (DposTrie, error)
	OpenLastDelegateTrie(root common.Hash) (DposTrie, error)
	OpenVoteTrie(root common.Hash) (DposTrie, error)
	OpenCandidateTrie(root common.Hash) (DposTrie, error)
	OpenMinedCntTrie(root common.Hash) (DposTrie, error)
	CopyTrie(DposTrie) DposTrie
}

// DposTrie is the adapter of trie data structure to be used in dposContext.
// DposTrie is implemented by
type DposTrie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit(onLeaf trie.LeafCallback) (common.Hash, error)
	Hash() common.Hash
	NodeIterator(startKey []byte) trie.NodeIterator
	PrefixIterator(prefix []byte) trie.NodeIterator
	Prove(key []byte, fromLevel uint, proofDb ethdb.Putter) error
}

// FullDposDatabase is the database for full node's dpos contents
type FullDposDatabase struct {
	db *trie.Database
}

// NewFullDposDatabase creates a new FullDposDatabase based on a diskdb
func NewFullDposDatabase(diskdb ethdb.Database) *FullDposDatabase {
	tdb := trie.NewDatabase(diskdb)
	return &FullDposDatabase{tdb}
}

// OpenEpochTrie opens the epochTrie
func (db *FullDposDatabase) OpenEpochTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

// OpenDelegateTrie opens the delegateTrie
func (db *FullDposDatabase) OpenDelegateTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

// OpenLastDelegateTrie opens the epochTrie in the last epoch
func (db *FullDposDatabase) OpenLastDelegateTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

// OpenVoteTrie opens the voteTrie
func (db *FullDposDatabase) OpenVoteTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

// OpenCandidateTrie opens the CandidateTrie
func (db *FullDposDatabase) OpenCandidateTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

// OpenMinedCntTrie opens the MinedCntTrie
func (db *FullDposDatabase) OpenMinedCntTrie(root common.Hash) (DposTrie, error) {
	return db.openTrie(root)
}

func (db *FullDposDatabase) openTrie(root common.Hash) (DposTrie, error) {
	return trie.New(root, db.db)
}

// CopyTrie copies a dpos trie
func (db *FullDposDatabase) CopyTrie(t DposTrie) DposTrie {
	switch t := t.(type) {
	case *trie.Trie:
		copy := *t
		return &copy
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// NewDposContext creates DposContext with the given database
func NewDposContext(db DposDatabase) (*DposContext, error) {
	epochTrie, err := db.OpenEpochTrie(common.Hash{})
	if err != nil {
		return nil, err
	}

	delegateTrie, err := db.OpenDelegateTrie(common.Hash{})
	if err != nil {
		return nil, err
	}

	voteTrie, err := db.OpenVoteTrie(common.Hash{})
	if err != nil {
		return nil, err
	}

	candidateTrie, err := db.OpenCandidateTrie(common.Hash{})
	if err != nil {
		return nil, err
	}

	minedCntTrie, err := db.OpenMinedCntTrie(common.Hash{})
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		minedCntTrie:  minedCntTrie,
		db:            db,
	}, nil
}

// NewDposContextFromRoot creates DposContext with database and trie root
func NewDposContextFromRoot(db DposDatabase, ctxProto *DposContextRoot) (*DposContext, error) {
	epochTrie, err := db.OpenEpochTrie(ctxProto.EpochRoot)
	if err != nil {
		return nil, err
	}

	delegateTrie, err := db.OpenDelegateTrie(ctxProto.DelegateRoot)
	if err != nil {
		return nil, err
	}

	voteTrie, err := db.OpenVoteTrie(ctxProto.VoteRoot)
	if err != nil {
		return nil, err
	}

	candidateTrie, err := db.OpenCandidateTrie(ctxProto.CandidateRoot)
	if err != nil {
		return nil, err
	}

	minedCntTrie, err := db.OpenMinedCntTrie(ctxProto.MinedCntRoot)
	if err != nil {
		return nil, err
	}

	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		minedCntTrie:  minedCntTrie,
		db:            db,
	}, nil
}

// Copy creates a new DposContext which has the same content with old one
func (dc *DposContext) Copy() *DposContext {
	return &DposContext{
		epochTrie:     dc.db.CopyTrie(dc.epochTrie),
		delegateTrie:  dc.db.CopyTrie(dc.delegateTrie),
		voteTrie:      dc.db.CopyTrie(dc.voteTrie),
		candidateTrie: dc.db.CopyTrie(dc.candidateTrie),
		minedCntTrie:  dc.db.CopyTrie(dc.minedCntTrie),
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dc *DposContext) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dc.epochTrie.Hash())
	rlp.Encode(hw, dc.delegateTrie.Hash())
	rlp.Encode(hw, dc.candidateTrie.Hash())
	rlp.Encode(hw, dc.voteTrie.Hash())
	rlp.Encode(hw, dc.minedCntTrie.Hash())
	hw.Sum(h[:0])
	return h
}

// Snapshot works as same with Copy
func (dc *DposContext) Snapshot() *DposContext {
	return dc.Copy()
}

// RevertToSnapShot revert current DposContext to a previous one
func (dc *DposContext) RevertToSnapShot(snapshot *DposContext) {
	dc.epochTrie = snapshot.epochTrie
	dc.delegateTrie = snapshot.delegateTrie
	dc.candidateTrie = snapshot.candidateTrie
	dc.voteTrie = snapshot.voteTrie
	dc.minedCntTrie = snapshot.minedCntTrie
}

// DposContextRoot wrap 5 trie root hash
type DposContextRoot struct {
	EpochRoot     common.Hash `json:"epochRoot"        gencodec:"required"`
	DelegateRoot  common.Hash `json:"delegateRoot"     gencodec:"required"`
	CandidateRoot common.Hash `json:"candidateRoot"    gencodec:"required"`
	VoteRoot      common.Hash `json:"voteRoot"         gencodec:"required"`
	MinedCntRoot  common.Hash `json:"minedCntRoot"     gencodec:"required"`
}

// ToRoot convert DposContext to DposContextRoot
func (dc *DposContext) ToRoot() *DposContextRoot {
	return &DposContextRoot{
		EpochRoot:     dc.epochTrie.Hash(),
		DelegateRoot:  dc.delegateTrie.Hash(),
		CandidateRoot: dc.candidateTrie.Hash(),
		VoteRoot:      dc.voteTrie.Hash(),
		MinedCntRoot:  dc.minedCntTrie.Hash(),
	}
}

// Root calculates the root hash of 5 tries in DposContext
func (dcr *DposContextRoot) Root() (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, dcr.EpochRoot)
	rlp.Encode(hw, dcr.DelegateRoot)
	rlp.Encode(hw, dcr.CandidateRoot)
	rlp.Encode(hw, dcr.VoteRoot)
	rlp.Encode(hw, dcr.MinedCntRoot)
	hw.Sum(h[:0])
	return h
}

// Copy makes a copy of the DposContextRoot
func (dcr *DposContextRoot) Copy() *DposContextRoot {
	return &DposContextRoot{
		EpochRoot:     dcr.EpochRoot,
		DelegateRoot:  dcr.DelegateRoot,
		CandidateRoot: dcr.CandidateRoot,
		VoteRoot:      dcr.VoteRoot,
		MinedCntRoot:  dcr.MinedCntRoot,
	}
}

// KickoutCandidate will kick out the given candidate
func (dc *DposContext) KickoutCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	err := dc.candidateTrie.TryDelete(candidate)
	if err != nil {

		// if got a MissingNodeError, that means cannot find the key of candidate in trie, it's normal case.
		// if not a MissingNodeError, that means something wrong with the db of trie
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return err
		}
	}

	iter := trie.NewIterator(dc.delegateTrie.PrefixIterator(candidate))
	for iter.Next() {
		delegator := iter.Value
		key := append(candidate, delegator...)
		err = dc.delegateTrie.TryDelete(key)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return err
			}
		}

		votedCandidateBytes, err := dc.voteTrie.TryGet(delegator)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return err
			}
		}

		if votedCandidateBytes == nil {
			return fmt.Errorf("no any voted candiate for %s", common.BytesToAddress(delegator).String())
		}

		votedCandidates := make([]common.Address, 0)
		err = rlp.DecodeBytes(votedCandidateBytes, &votedCandidates)
		if err != nil {
			return fmt.Errorf("failed to rlp decode candidate bytes for voteTrie,error: %v", err)
		}

		// remove the given candidate from the vote records of delegator
		index := 0
		for i, votedCandidate := range votedCandidates {
			if votedCandidate == candidateAddr {
				index = i
				break
			}
		}
		pre := votedCandidates[:index]
		suf := make([]common.Address, 0)
		if index != len(votedCandidates)-1 {
			suf = votedCandidates[index+1:]
		}
		candidatesAfterRemove := make([]common.Address, 0)
		candidatesAfterRemove = append(candidatesAfterRemove, pre...)
		candidatesAfterRemove = append(candidatesAfterRemove, suf...)

		value, err := rlp.EncodeToBytes(candidatesAfterRemove)
		if err != nil {
			return fmt.Errorf("failed to rlp encode candidate bytes for voteTrie,error: %v", err)
		}
		err = dc.voteTrie.TryUpdate(delegator, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// BecomeCandidate will store the given candidate into candidateTrie
func (dc *DposContext) BecomeCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	return dc.candidateTrie.TryUpdate(candidate, candidate)
}

// Vote will store the vote record
func (dc *DposContext) Vote(delegatorAddr common.Address, candidateList []common.Address) (int, error) {
	delegator := delegatorAddr.Bytes()
	successVoted := make([]common.Address, 0)

	oldCandidateBytes, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return 0, fmt.Errorf("failed to retrieve from voteTrie,err: %v", err)
		}
	}

	// if voted before, then delete old vote record
	if oldCandidateBytes != nil {
		err = dc.voteTrie.TryDelete(delegator)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); !ok {
				return 0, fmt.Errorf("failed to delete old votes from voteTrie,err: %v", err)
			}
		}

		oldCandidateList := make([]common.Address, 0)
		err = rlp.DecodeBytes(oldCandidateBytes, &oldCandidateList)
		if err != nil {
			return 0, fmt.Errorf("failed to rlp decode old candidate bytes,err: %v", err)
		}

		for _, oldCandidate := range oldCandidateList {
			err = dc.delegateTrie.TryDelete(append(oldCandidate.Bytes(), delegator...))
			if err != nil {
				if _, ok := err.(*trie.MissingNodeError); !ok {
					return 0, fmt.Errorf("failed to delete old votes from delegateTrie,err: %v", err)
				}
			}
		}
	}

	for _, candidateAddr := range candidateList {
		candidate := candidateAddr.Bytes()

		// the given candidate must have been candidate
		candidateInTrie, err := dc.candidateTrie.TryGet(candidate)
		if err != nil {
			if _, ok := err.(*trie.MissingNodeError); ok {
				log.Error("No this candidate on chain", "candidate", candidateAddr.String())
				continue
			} else {
				return 0, fmt.Errorf("failed to retrieve from candidateTrie,err: %v", err)
			}
		}

		if candidateInTrie == nil {
			log.Error("Voted to a invalid candidate", "candidate", candidateAddr.String())
			continue
		}

		err = dc.delegateTrie.TryUpdate(append(candidate, delegator...), delegator)
		if err != nil {
			log.Error("Failed to update a new vote to delegateTrie", "error", err)
			continue
		}

		// successfully vote a candidate, then add it
		successVoted = append(successVoted, candidateAddr)
	}

	// store all successful voted candidate
	if len(successVoted) != 0 {
		votedCandidateBytes, err := rlp.EncodeToBytes(successVoted)
		if err != nil {
			return 0, fmt.Errorf("failed to rlp encode voted candidates,err: %v", err)
		}

		err = dc.voteTrie.TryUpdate(delegator, votedCandidateBytes)
		if err != nil {
			return 0, fmt.Errorf("failed to update new vote to voteTrie,err: %v", err)
		}
		return len(successVoted), nil
	}

	return 0, errors.New("failed to vote all candidates")
}

// CancelVote will remove all vote records
func (dc *DposContext) CancelVote(delegatorAddr common.Address) error {
	delegator := delegatorAddr.Bytes()

	oldCandidateBytes, err := dc.voteTrie.TryGet(delegator)
	if err != nil {
		if _, ok := err.(*trie.MissingNodeError); !ok {
			return fmt.Errorf("failed to retrieve from voteTrie,err: %v", err)
		}
	}

	if oldCandidateBytes == nil {
		return fmt.Errorf("no vote records on chain for %s", delegatorAddr.String())
	}

	oldCandidateList := make([]common.Address, 0)
	err = rlp.DecodeBytes(oldCandidateBytes, &oldCandidateList)
	if err != nil {
		return fmt.Errorf("failed to rlp decode old candidate bytes,err: %v", err)
	}

	// delete all vote records from delegateTrie
	for _, oldCandidate := range oldCandidateList {
		err = dc.delegateTrie.TryDelete(append(oldCandidate.Bytes(), delegator...))
		if err != nil {
			return fmt.Errorf("failed to delete old votes from delegateTrie,err: %v", err)
		}
	}
	// delete vote records from voteTrie
	err = dc.voteTrie.TryDelete(delegator)
	if err != nil {
		return fmt.Errorf("failed to delete old votes from voteTrie,err: %v", err)
	}

	return nil
}

// Commit writes the data in 5 tries to db
func (dc *DposContext) Commit() (*DposContextRoot, error) {
	var tdb *trie.Database
	if db, ok := dc.db.(*FullDposDatabase); ok {
		tdb = db.db
	} else {
		return nil, errors.New("commit method not implemented for light node")
	}

	// commit dpos context into memory
	epochRoot, err := dc.epochTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	delegateRoot, err := dc.delegateTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	voteRoot, err := dc.voteTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	candidateRoot, err := dc.candidateTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	minedCntRoot, err := dc.minedCntTrie.Commit(nil)
	if err != nil {
		return nil, err
	}

	// commit dpos context into disk, and this is the finally commit
	err = tdb.Commit(epochRoot, false)
	if err != nil {
		return nil, err
	}

	err = tdb.Commit(candidateRoot, false)
	if err != nil {
		return nil, err
	}

	err = tdb.Commit(delegateRoot, false)
	if err != nil {
		return nil, err
	}

	err = tdb.Commit(minedCntRoot, false)
	if err != nil {
		return nil, err
	}

	err = tdb.Commit(voteRoot, false)
	if err != nil {
		return nil, err
	}

	return &DposContextRoot{
		EpochRoot:     epochRoot,
		DelegateRoot:  delegateRoot,
		VoteRoot:      voteRoot,
		CandidateRoot: candidateRoot,
		MinedCntRoot:  minedCntRoot,
	}, nil
}

// CandidateTrie return the candidate trie of the DposContext
func (dc *DposContext) CandidateTrie() DposTrie { return dc.candidateTrie }

// DelegateTrie return the delegate trie of the DposContext
func (dc *DposContext) DelegateTrie() DposTrie { return dc.delegateTrie }

// VoteTrie return the vote trie of the DposContext
func (dc *DposContext) VoteTrie() DposTrie { return dc.voteTrie }

// EpochTrie return the epoch trie of the DposContext
func (dc *DposContext) EpochTrie() DposTrie { return dc.epochTrie }

// MinedCntTrie return the mined count trie of the DposContext
func (dc *DposContext) MinedCntTrie() DposTrie { return dc.minedCntTrie }

// DB return the underlying database in the DposContext
func (dc *DposContext) DB() DposDatabase { return dc.db }

// SetEpoch set the epoch trie of the DposContext
func (dc *DposContext) SetEpoch(epoch DposTrie) { dc.epochTrie = epoch }

// SetDelegate set the delegate trie of the DposContext
func (dc *DposContext) SetDelegate(delegate DposTrie) { dc.delegateTrie = delegate }

// SetVote set the vote trie of the DposContext
func (dc *DposContext) SetVote(vote DposTrie) { dc.voteTrie = vote }

// SetCandidate set the candidate trie of the DposContext
func (dc *DposContext) SetCandidate(candidate DposTrie) { dc.candidateTrie = candidate }

// SetMinedCnt set the mined count trie of the DposContext
func (dc *DposContext) SetMinedCnt(minedCnt DposTrie) { dc.minedCntTrie = minedCnt }

// GetValidators retrieves validator list in current epoch
func (dc *DposContext) GetValidators() ([]common.Address, error) {
	return GetValidatorsFromDposTrie(dc.epochTrie)
}

// GetValidatorsFromDposTrie get the list of validators from the trie
func GetValidatorsFromDposTrie(t DposTrie) ([]common.Address, error) {
	var validators []common.Address
	validatorsRLP, err := t.TryGet(keyValidator)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator: %v", err)
	}
	if len(validatorsRLP) == 0 {
		return nil, fmt.Errorf("empty value for validators, make sure opening the epoch trie")
	}
	if err := rlp.DecodeBytes(validatorsRLP, &validators); err != nil {
		return nil, fmt.Errorf("failed to decode validators: %s", err)
	}
	return validators, nil
}

// SetValidators update validators into epochTrie
func (dc *DposContext) SetValidators(validators []common.Address) error {
	validatorsRLP, err := rlp.EncodeToBytes(validators)
	if err != nil {
		return fmt.Errorf("failed to encode validators to rlp bytes: %s", err)
	}

	err = dc.epochTrie.TryUpdate(keyValidator, validatorsRLP)
	if err != nil {
		return fmt.Errorf("failed to set validator: %v", err)
	}
	return nil
}

// SaveValidators save the validators to db as a trie
func SaveValidators(validators []common.Address, db ethdb.Database) error {
	dposCtx, err := NewDposContext(NewFullDposDatabase(db))
	if err != nil {
		return err
	}
	if err = dposCtx.SetValidators(validators); err != nil {
		return err
	}
	_, err = dposCtx.Commit()
	return err
}

// GetVotedCandidatesByAddress retrieve all voted candidates of given delegator
func (dc *DposContext) GetVotedCandidatesByAddress(delegator common.Address) ([]common.Address, error) {
	key := delegator.Bytes()
	candidatesRLP, err := dc.voteTrie.TryGet(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get the vote for candidate [%x]: %v", delegator, err)
	}

	var result []common.Address
	err = rlp.DecodeBytes(candidatesRLP, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vaoted candidates: %s", err)
	}

	return result, nil
}

// GetMinedCnt get mined block count in the minedCntTrie
func (dc *DposContext) GetMinedCnt(epoch int64, addr common.Address) (int64, error) {
	key := makeMinedCntKey(epoch, addr)
	cntBytes, err := dc.minedCntTrie.TryGet(key)
	if err != nil {
		return 0, err
	}
	if cntBytes == nil || len(cntBytes) < 8 {
		return 0, nil
	}
	cnt := int64(binary.BigEndian.Uint64(cntBytes))
	return cnt, nil
}

// GetCandidates will iterate through the candidateTrie and get all candidates
func (dc *DposContext) GetCandidates() []common.Address {
	var candidates []common.Address
	iterCandidate := trie.NewIterator(dc.candidateTrie.NodeIterator(nil))
	for iterCandidate.Next() {
		candidateAddr := common.BytesToAddress(iterCandidate.Value)
		candidates = append(candidates, candidateAddr)
	}

	return candidates
}

// makeMinedCntKey is the private function to make the key for the specified addr and
// epoch
func makeMinedCntKey(epoch int64, validatorAddr common.Address) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epoch))
	key = append(key, validatorAddr.Bytes()...)
	return key
}

// DPOS related transaction data.
type (
	// AddCandidateTxData is the data field for AddCandidateTx
	AddCandidateTxData struct {
		Deposit     common.BigInt
		RewardRatio uint64
	}

	// addCandidateTxRLPData is the rlp data structure used for rlp encoding/decoding for
	// AddCandidateTx
	addCandidateTxRLPData struct {
		Deposit     *big.Int
		RewardRatio uint64
	}

	// VoteTxData is the data field for VoteTx
	VoteTxData struct {
		Deposit    common.BigInt
		Candidates []common.Address
	}

	// voteTxRLPData is the rlp data structure used for rlp encoding/decoding for
	// VoteTxData
	voteTxRLPData struct {
		Deposit    *big.Int
		Candidates []common.Address
	}
)

// EncodeRLP defines the rlp encoding rule for AddCandidateTxData
func (data *AddCandidateTxData) EncodeRLP(w io.Writer) error {
	rlpData := addCandidateTxRLPData{
		Deposit:     data.Deposit.BigIntPtr(),
		RewardRatio: data.RewardRatio,
	}
	return rlp.Encode(w, rlpData)
}

// DecodeRLP defines the rlp decoding rule for AddCandidateTxData
func (data *AddCandidateTxData) DecodeRLP(s *rlp.Stream) error {
	var rlpData addCandidateTxRLPData
	if err := s.Decode(&rlpData); err != nil {
		return err
	}
	data.RewardRatio, data.Deposit = rlpData.RewardRatio, common.PtrBigInt(rlpData.Deposit)
	return nil
}

// EncodeRLP defines the rlp encoding rule for VoteTxData
func (data *VoteTxData) EncodeRLP(w io.Writer) error {
	rlpData := voteTxRLPData{
		Deposit:    data.Deposit.BigIntPtr(),
		Candidates: data.Candidates,
	}
	return rlp.Encode(w, rlpData)
}

// DecodeRLP defines the rlp decoding rule for VoteTxData
func (data *VoteTxData) DecodeRLP(s *rlp.Stream) error {
	var rlpData voteTxRLPData
	if err := s.Decode(&rlpData); err != nil {
		return err
	}
	data.Deposit, data.Candidates = common.PtrBigInt(rlpData.Deposit), rlpData.Candidates
	return nil
}

// HeaderInsertData is the data structure necessary for header insertion
type HeaderInsertData struct {
	Header     *Header
	Validators []common.Address
}

// HeaderInsertDataBatch is a batch of HeaderInsertData
type HeaderInsertDataBatch []HeaderInsertData

// NewHeaderInsertDataBatch return the new HeaderInsertDataBatch
func NewHeaderInsertDataBatch(headers []*Header, validators [][]common.Address) HeaderInsertDataBatch {
	if len(validators) == 0 {
		validators = make([][]common.Address, len(headers))
	}
	var data HeaderInsertDataBatch
	for i, header := range headers {
		data = append(data, HeaderInsertData{
			Header:     header,
			Validators: validators[i],
		})
	}
	return data
}

// Split split the HeaderInsertDataBatch to two slices:
// One as a slice of Header, another a slice of validators
func (hidb HeaderInsertDataBatch) Split() ([]*Header, [][]common.Address) {
	size := len(hidb)
	headers := make([]*Header, 0, size)
	validatorsSet := make([][]common.Address, 0, size)
	for _, hid := range hidb {
		headers = append(headers, hid.Header)
		validatorsSet = append(validatorsSet, hid.Validators)
	}
	return headers, validatorsSet
}
