// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/consensus/misc"
	"github.com/DxChainNetwork/godx/core/state"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/trie"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

// Mode is dpos consensus engine work mode
type Mode uint

const (
	// ModeNormal is the default work mode
	ModeNormal Mode = iota

	// ModeFake is fake mode skipping verify(Header/Uncle/DposState) logic
	ModeFake
)

var (
	// PrefixThawingAddr is the prefix thawing string of frozen account
	PrefixThawingAddr = "thawing_"

	confirmedBlockHead = []byte("confirmed-block-head")
)

var (
	uncleHash = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.
)

// Dpos consensus engine
type Dpos struct {
	config *params.DposConfig // Consensus engine configuration parameters
	db     ethdb.Database     // Database to store and retrieve snapshot checkpoints

	signer     common.Address
	signFn     SignerFn
	signatures *lru.ARCCache // Signatures of recent blocks to speed up mining

	mu   sync.RWMutex
	stop chan bool

	Mode Mode
}

// SignerFn is the function for signature
type SignerFn func(accounts.Account, []byte) ([]byte, error)

// NOTE: sigHash was copy from clique
// sigHash returns the hash which is used as input for the proof-of-authority
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Validator,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-65], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
		header.DposContext.Root(),
	})
	hasher.Sum(hash[:0])
	return hash
}

// New creates a dpos consensus engine
func New(config *params.DposConfig, db ethdb.Database) *Dpos {
	signatures, _ := lru.NewARC(inmemorySignatures)
	return &Dpos{
		config:     config,
		db:         db,
		signatures: signatures,
	}
}

// NewDposFaker create fake dpos for test
func NewDposFaker(db ethdb.Database) *Dpos {
	return &Dpos{
		Mode: ModeFake,
		db:   db,
	}
}

// Author return the address who produced the block
func (d *Dpos) Author(header *types.Header) (common.Address, error) {
	return header.Validator, nil
}

// Coinbase return the address who should receive the award
func (d *Dpos) Coinbase(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader check the given header whether it's fit for dpos engine
func (d *Dpos) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return d.verifyHeader(chain, header, nil, nil)
}

func (d *Dpos) verifyHeader(chain consensus.ChainReader, header *types.Header, validators []common.Address,
	parents []*types.Header) error {

	if d.Mode == ModeFake {
		var parent *types.Header
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		} else {
			parent = chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
		}
		if parent == nil || parent.Number.Uint64() != header.Number.Uint64()-1 || parent.Hash() != header.ParentHash {
			return consensus.ErrUnknownAncestor
		}
		return nil
	}

	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()
	// Unnecessary to verify the block from feature
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}
	// Check that the extra-data contains both the vanity and signature
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
	// Difficulty always 1
	if header.Difficulty.Uint64() != 1 {
		return errInvalidDifficulty
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in DPoS
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
		return err
	}

	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	if parent.Time.Uint64()+uint64(BlockInterval) > header.Time.Uint64() {
		return ErrInvalidTimestamp
	}
	var vGetter validatorHelper
	if len(validators) != 0 {
		vGetter = newVHelper(validators, header)
	} else {
		vGetter = newDBVHelper(d.db, parent, header)
	}
	return d.verifySeal(chain, header, vGetter)
}

// VerifyHeaders verify a batch of headers. The input validatorsSet is the new validators
// for each header.
func (d *Dpos) VerifyHeaders(chain consensus.ChainReader, batch types.HeaderInsertDataBatch, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(batch))
	headers, validatorsSet := batch.Split()
	go func() {
		for i, header := range headers {
			parents := headers[:i]
			err := validateHeaderWithValidators(header, validatorsSet[i])
			if err != nil {
				var validators []common.Address
				if i != 0 {
					validators = validatorsSet[i-1]
				}
				err = d.verifyHeader(chain, header, validators, parents)
			}
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

//  validateHeaderWithValidators checks whether the validators is consistent with the header
func validateHeaderWithValidators(header *types.Header, validators []common.Address) error {
	expectRoot := header.DposContext.EpochRoot
	dc, err := types.NewDposContext(types.NewFullDposDatabase(nil))
	if err != nil {
		return fmt.Errorf("cannot create the dpos context: %v", err)
	}
	if err := dc.SetValidators(validators); err != nil {
		return fmt.Errorf("cannot set validators: %v", err)
	}
	gotRoot := dc.EpochTrie().Hash()
	if expectRoot != gotRoot {
		return fmt.Errorf("validators not consist with header")
	}
	return nil
}

// VerifyUncles implements consensus.Engine, returning an error if the block has uncles,
// because dpos engine doesn't support uncles.
func (d *Dpos) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if d.Mode == ModeFake {
		return nil
	}

	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// VerifySeal implements consensus.Engine, checking whether the signature contained
// in the header satisfies the consensus protocol requirements.
func (d *Dpos) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	validatorGetter, err := newDBVHelperNoParent(chain, d.db, header)
	if err != nil {
		return err
	}
	return d.verifySeal(chain, header, validatorGetter)
}

func (d *Dpos) verifySeal(chain consensus.ChainReader, header *types.Header, vHelper validatorHelper) error {

	if d.Mode == ModeFake {
		seal := header.Extra[len(header.Extra)-extraSeal:]
		targetValidator, err := vHelper.getValidator()
		if err != nil {
			return fmt.Errorf("cannot get validator: %v", err)
		}
		if !bytes.Equal(seal[:common.AddressLength], targetValidator[:]) {
			return fmt.Errorf("fake verify seal failed. got %x, expect %x", seal[:common.AddressLength], targetValidator[:])
		}
		return nil
	}
	validator, err := vHelper.getValidator()
	if err != nil {
		return fmt.Errorf("cannot get validator: %v", err)
	}
	if err := d.verifyBlockSigner(validator, header); err != nil {
		return err
	}
	return d.updateConfirmedBlockHeader(chain)
}

func (d *Dpos) verifyBlockSigner(validator common.Address, header *types.Header) error {
	signer, err := ecrecover(header, d.signatures)
	if err != nil {
		return err
	}
	if bytes.Compare(signer.Bytes(), validator.Bytes()) != 0 {
		return ErrInvalidBlockValidator
	}
	if bytes.Compare(signer.Bytes(), header.Validator.Bytes()) != 0 {
		return ErrMismatchSignerAndValidator
	}
	return nil
}

// load the latest confirmed block from the database
func (d *Dpos) loadConfirmedBlockHeader(chain consensus.ChainReader) (*types.Header, error) {
	key, err := d.db.Get(confirmedBlockHead)
	if err != nil {
		return nil, err
	}
	header := chain.GetHeaderByHash(common.BytesToHash(key))
	if header == nil {
		return nil, ErrNilBlockHeader
	}
	return header, nil
}

// Prepare implements consensus.Engine, assembly some basic fields into header
func (d *Dpos) Prepare(chain consensus.ChainReader, header *types.Header) error {
	header.Nonce = types.BlockNonce{}
	number := header.Number.Uint64()
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = d.CalcDifficulty(chain, header.Time.Uint64(), parent)
	header.Validator = d.signer
	return nil
}

// accumulateRewards add the block award to Coinbase of validator
func accumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, db ethdb.Database, genesis *types.Header) {
	// Select the correct block reward based on chain progression
	blockReward := frontierBlockReward
	if config.IsByzantium(header.Number) {
		blockReward = byzantiumBlockReward
	}
	if config.IsConstantinople(header.Number) {
		blockReward = constantinopleBlockReward
	}
	// retrieve the total vote weight of header's validator
	voteCount := GetTotalVote(state, header.Validator)
	if voteCount.Cmp(common.BigInt0) <= 0 {
		state.AddBalance(header.Coinbase, blockReward.BigIntPtr())
		return
	}
	// get ratio of reward between validator and its delegator
	rewardRatioNumerator := GetRewardRatioNumeratorLastEpoch(state, header.Validator)
	sharedReward := blockReward.MultUint64(rewardRatioNumerator).DivUint64(RewardRatioDenominator)
	assignedReward := common.BigInt0

	// Loop over the delegators to add delegator rewards
	preEpochSnapshotDelegateTrieRoot := getPreEpochSnapshotDelegateTrieRoot(state, genesis)
	delegateTrie, err := getPreEpochSnapshotDelegateTrie(types.NewFullDposDatabase(db), preEpochSnapshotDelegateTrieRoot)
	if err != nil {
		log.Error("couldn't get snapshot delegate trie, error:", err)
		return
	}

	delegatorIterator := trie.NewIterator(delegateTrie.PrefixIterator(header.Validator.Bytes()))
	for delegatorIterator.Next() {
		delegator := common.BytesToAddress(delegatorIterator.Value)
		// get the votes of delegator to vote for delegate
		delegatorVote := GetVoteLastEpoch(state, delegator)
		// calculate reward of each delegator due to it's vote(stake) percent
		delegatorReward := delegatorVote.Mult(sharedReward).Div(voteCount)
		state.AddBalance(delegator, delegatorReward.BigIntPtr())
		assignedReward = assignedReward.Add(delegatorReward)
	}
	// accumulate the rest rewards for the validator
	validatorReward := blockReward.Sub(assignedReward)
	state.AddBalance(header.Coinbase, validatorReward.BigIntPtr())
}

// Finalize implements consensus.Engine, commit state、calculate block award and update some context
func (d *Dpos) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt, dposContext *types.DposContext) (*types.Block, error) {

	// Accumulate block rewards and commit the final state root
	genesis := chain.GetHeaderByNumber(0)
	accumulateRewards(chain.Config(), state, header, d.db, genesis)

	//if d.Mode == ModeFake {
	//	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	//	return types.NewBlock(header, txs, uncles, receipts), nil
	//}

	parent := chain.GetHeaderByHash(header.ParentHash)
	epochContext := &EpochContext{
		stateDB:     state,
		DposContext: dposContext,
		TimeStamp:   header.Time.Int64(),
	}
	// update the value of timeOfFirstBlock if the value is 0
	updateTimeOfFirstBlockIfNecessary(chain)

	//update mined count trie
	err := updateMinedCnt(parent.Time.Int64(), header.Validator, dposContext)
	if err != nil {
		return nil, err
	}
	// try to elect, if current block is the first one in a new epoch, then elect new epoch
	err = epochContext.tryElect(genesis, parent)
	if err != nil {
		return nil, fmt.Errorf("got error when elect next epoch, err: %s", err)
	}

	header.DposContext = dposContext.ToRoot()
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	block := types.NewBlock(header, txs, uncles, receipts)
	block.SetDposCtx(dposContext)
	return block, nil
}

// checkDeadline check the given block whether is fit to produced at now
func (d *Dpos) checkDeadline(lastBlock *types.Block, now int64) error {
	prevSlot := PrevSlot(now)
	nextSlot := NextSlot(now)
	if lastBlock.Time().Int64() >= nextSlot {
		return ErrMinedFutureBlock
	}
	// last block was arrived, or time's up
	if lastBlock.Time().Int64() == prevSlot || nextSlot-now <= 0 {
		return nil
	}
	return ErrWaitForPrevBlock
}

// CheckValidator check the given block whether has a right validator to produce
func (d *Dpos) CheckValidator(lastBlock *types.Block, now int64) error {
	if d.Mode == ModeFake {
		return nil
	}

	if err := d.checkDeadline(lastBlock, now); err != nil {
		return err
	}
	dposContext, err := types.NewDposContextFromRoot(types.NewFullDposDatabase(d.db), lastBlock.Header().DposContext)
	if err != nil {
		return err
	}
	epochContext := &EpochContext{DposContext: dposContext}
	validator, err := epochContext.lookupValidator(now)
	if err != nil {
		return err
	}

	if (validator == common.Address{}) || bytes.Compare(validator.Bytes(), d.signer.Bytes()) != 0 {
		return ErrInvalidBlockValidator
	}

	return nil
}

// Seal implements consensus.Engine, sign the given block and return it
func (d *Dpos) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	if d.Mode == ModeFake {
		header := block.Header()
		header.Nonce, header.MixDigest = types.BlockNonce{}, common.Hash{}
		header.Extra = make([]byte, extraSeal)
		validators, err := block.DposCtx().GetValidators()
		if err != nil {
			return fmt.Errorf("fake mode get validators error: %v", err)
		}
		validator, err := lookupValidator(block.Time().Int64(), validators[:])
		if err != nil {
			return fmt.Errorf("fake mode look up validator error: %v", err)
		}
		copy(header.Extra, validator[:])
		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "mode", "fake")
		}
		return nil
	}

	header := block.Header()
	number := header.Number.Uint64()
	// Sealing the genesis block is not supported
	if number == 0 {
		return errUnknownBlock
	}
	//now := time.Now().Unix()
	//delay := NextSlot(now) - now
	//if delay > 0 {
	//	select {
	//	case <-stop:
	//		return nil
	//	case <-time.After(time.Duration(delay) * time.Second):
	//	}
	//}
	//block.Header().Time.SetInt64(time.Now().Unix())

	// time's up, sign the block
	sighash, err := d.signFn(accounts.Account{Address: d.signer}, sigHash(header).Bytes())
	if err != nil {
		return err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	results <- block.WithSeal(header)
	return nil
}

// CalcDifficulty return a constant value for dpos consensus engine
func (d *Dpos) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

// Authorize register the miner address and signature func when node start
func (d *Dpos) Authorize(signer common.Address, signFn SignerFn) {
	d.mu.Lock()
	d.signer = signer
	d.signFn = signFn
	d.mu.Unlock()
}

// APIs implemented Engine interface which includes DPOS API
func (d *Dpos) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "dpos",
		Version:   "1.0",
		Service:   &API{chain: chain, dpos: d},
		Public:    true,
	}}
}

// SealHash implements consensus.Engine, returns the hash of a block prior to it being sealed.
func (d *Dpos) SealHash(header *types.Header) common.Hash {
	return sigHash(header)
}

// Close implements consensus.Engine, It's a noop for dpos as there are no background threads.
func (d *Dpos) Close() error {
	return nil
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]
	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(sigHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	sigcache.Add(hash, signer)
	return signer, nil
}

// PrevSlot calculate the last block time
func PrevSlot(now int64) int64 {
	return int64((now-1)/BlockInterval) * BlockInterval
}

// NextSlot calculate the next block time
func NextSlot(now int64) int64 {
	return int64((now+BlockInterval-1)/BlockInterval) * BlockInterval
}

// updateMinedCnt update counts in minedCntTrie for the miner of newBlock
func updateMinedCnt(parentBlockTime int64, validator common.Address, dposContext *types.DposContext) error {
	mct := dposContext.MinedCntTrie()
	// The updated mined count belong to the parent epoch
	epoch := CalculateEpochID(parentBlockTime)
	cnt, err := getMinedCnt(mct, epoch, validator)
	if err != nil {
		return err
	}
	cnt++
	return setMinedCnt(mct, epoch, validator, cnt)
}

// getPreEpochSnapshotDelegateTrie get the snapshot delegate trie of pre epoch
func getPreEpochSnapshotDelegateTrie(db types.DposDatabase, root common.Hash) (types.DposTrie, error) {
	return db.OpenLastDelegateTrie(root)
}

func setMinedCnt(minedCntTrie types.DposTrie, epoch int64, validator common.Address, value uint64) error {
	keyBytes := makeMinedCntKey(epoch, validator)
	valueBytes := common.Uint64ToByte(value)
	return minedCntTrie.TryUpdate(keyBytes, valueBytes)
}

// getMinedCnt get the mined count of a specified validator in a certain epoch from the minedCntTrie
func getMinedCnt(minedCntTrie types.DposTrie, epoch int64, validator common.Address) (uint64, error) {
	key := makeMinedCntKey(epoch, validator)
	cntBytes, err := minedCntTrie.TryGet(key)
	if err != nil {
		return 0, err
	}
	var cnt uint64
	if cntBytes != nil && len(cntBytes) != 0 {
		cnt = common.BytesToUint64(cntBytes)
	}
	return cnt, nil
}

func makeMinedCntKey(epoch int64, addr common.Address) []byte {
	epochBytes := common.Uint64ToByte(uint64(epoch))
	return append(epochBytes, addr.Bytes()...)
}

// validatorHelper is the interface for getting the validator for a header,
// and save the validators to db
type validatorHelper interface {
	getValidator() (common.Address, error)
}

// vHelper is the validatorHelper which gets info from validators
type vHelper struct {
	validators []common.Address
	header     *types.Header
}

func newVHelper(validators []common.Address, header *types.Header) *vHelper {
	return &vHelper{
		validators: validators,
		header:     header,
	}
}

func (g *vHelper) getValidator() (common.Address, error) {
	return lookupValidator(g.header.Time.Int64(), g.validators)
}

// dbVHelper get the validator for the header from database
type dbVHelper struct {
	db     ethdb.Database
	parent *types.Header
	header *types.Header

	validators []common.Address
}

func newDBVHelper(db ethdb.Database, parent, header *types.Header) *dbVHelper {
	return &dbVHelper{
		db:     db,
		parent: parent,
		header: header,
	}
}

func newDBVHelperNoParent(chain consensus.ChainReader, db ethdb.Database, header *types.Header) (*dbVHelper, error) {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return nil, fmt.Errorf("header does not exist on chain: %x", header.ParentHash)
	}
	return &dbVHelper{
		db:     db,
		parent: parent,
		header: header,
	}, nil
}

func (g *dbVHelper) getValidator() (common.Address, error) {
	validators, err := g.getValidators()
	if err != nil {
		return common.Address{}, err
	}
	g.validators = validators
	return lookupValidator(g.header.Time.Int64(), validators)
}

func (g *dbVHelper) getValidators() ([]common.Address, error) {
	if g.validators != nil {
		return g.validators, nil
	}
	dposDb := types.NewFullDposDatabase(g.db)
	epochTrie, err := dposDb.OpenEpochTrie(g.parent.DposContext.EpochRoot)
	if err != nil {
		return []common.Address{}, err
	}
	validators, err := types.GetValidatorsFromDposTrie(epochTrie)
	if err != nil {
		return []common.Address{}, err
	}
	g.validators = validators
	return validators, nil
}
