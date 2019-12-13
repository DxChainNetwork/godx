// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package types contains data types related to Ethereum consensus.
package types

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/hexutil"
	"github.com/DxChainNetwork/godx/rlp"
	"golang.org/x/crypto/sha3"
)

var (
	EmptyRootHash  = DeriveSha(Transactions{})
	EmptyUncleHash = CalcUncleHash(nil)
	EmptyHash      = common.Hash{}
)

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	ParentHash  common.Hash      `json:"parentHash"       gencodec:"required"` // Hash pointer to the previous block
	UncleHash   common.Hash      `json:"sha3Uncles"       gencodec:"required"` // Hash pointer to the uncle
	Validator   common.Address   `json:"validator"        gencodec:"required"`
	Coinbase    common.Address   `json:"coinbase"         gencodec:"required"` // Address of the coinbase
	Root        common.Hash      `json:"stateRoot"        gencodec:"required"` // stateRoot
	TxHash      common.Hash      `json:"transactionsRoot" gencodec:"required"` // txRoot
	ReceiptHash common.Hash      `json:"receiptsRoot"     gencodec:"required"` // Receipt root
	DposContext *DposContextRoot `json:"dposContext"      gencodec:"required"`
	Bloom       Bloom            `json:"logsBloom"        gencodec:"required"` //
	Difficulty  *big.Int         `json:"difficulty"       gencodec:"required"` // Difficulty of the current block
	Number      *big.Int         `json:"number"           gencodec:"required"` // Block height
	GasLimit    uint64           `json:"gasLimit"         gencodec:"required"` // Total gases could be spent
	GasUsed     uint64           `json:"gasUsed"          gencodec:"required"` // Gas spent in transactions from this block
	Time        *big.Int         `json:"timestamp"        gencodec:"required"` // timestamp
	Extra       []byte           `json:"extraData"        gencodec:"required"` // Extra info
	MixDigest   common.Hash      `json:"mixHash"`                              // Signature?
	Nonce       BlockNonce       `json:"nonce"`                                // Number used for PoW
}

// field type overrides for gencodec
type headerMarshaling struct {
	Difficulty *hexutil.Big
	Number     *hexutil.Big
	GasLimit   hexutil.Uint64
	GasUsed    hexutil.Uint64
	Time       *hexutil.Big
	Extra      hexutil.Bytes
	Hash       common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	return rlpHash(h)
}

// size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}

var _ = (*headerMarshaling)(nil)

// MarshalJSON marshals as JSON.
func (h Header) MarshalJSON() ([]byte, error) {
	type Header struct {
		ParentHash  common.Hash      `json:"parentHash"       gencodec:"required"`
		UncleHash   common.Hash      `json:"sha3Uncles"       gencodec:"required"`
		Validator   common.Address   `json:"validator"        gencodec:"required"`
		Coinbase    common.Address   `json:"coinbase"         gencodec:"required"`
		Root        common.Hash      `json:"stateRoot"        gencodec:"required"`
		TxHash      common.Hash      `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash common.Hash      `json:"receiptsRoot"     gencodec:"required"`
		DposContext *DposContextRoot `json:"dposContext"      gencodec:"required"`
		Bloom       Bloom            `json:"logsBloom"        gencodec:"required"`
		Difficulty  *hexutil.Big     `json:"difficulty"       gencodec:"required"`
		Number      *hexutil.Big     `json:"number"           gencodec:"required"`
		GasLimit    hexutil.Uint64   `json:"gasLimit"         gencodec:"required"`
		GasUsed     hexutil.Uint64   `json:"gasUsed"          gencodec:"required"`
		Time        *hexutil.Big     `json:"timestamp"        gencodec:"required"`
		Extra       hexutil.Bytes    `json:"extraData"        gencodec:"required"`
		MixDigest   common.Hash      `json:"mixHash"`
		Nonce       BlockNonce       `json:"nonce"`
		Hash        common.Hash      `json:"hash"`
	}
	var enc Header
	enc.ParentHash = h.ParentHash
	enc.UncleHash = h.UncleHash
	enc.Validator = h.Validator
	enc.Coinbase = h.Coinbase
	enc.Root = h.Root
	enc.TxHash = h.TxHash
	enc.ReceiptHash = h.ReceiptHash
	enc.DposContext = h.DposContext
	enc.Bloom = h.Bloom
	enc.Difficulty = (*hexutil.Big)(h.Difficulty)
	enc.Number = (*hexutil.Big)(h.Number)
	enc.GasLimit = hexutil.Uint64(h.GasLimit)
	enc.GasUsed = hexutil.Uint64(h.GasUsed)
	enc.Time = (*hexutil.Big)(h.Time)
	enc.Extra = h.Extra
	enc.MixDigest = h.MixDigest
	enc.Nonce = h.Nonce
	enc.Hash = h.Hash()
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (h *Header) UnmarshalJSON(input []byte) error {
	type Header struct {
		ParentHash  *common.Hash     `json:"parentHash"       gencodec:"required"`
		UncleHash   *common.Hash     `json:"sha3Uncles"       gencodec:"required"`
		Validator   *common.Address  `json:"validator"        gencodec:"required"`
		Coinbase    *common.Address  `json:"coinbase"         gencodec:"required"`
		Root        *common.Hash     `json:"stateRoot"        gencodec:"required"`
		TxHash      *common.Hash     `json:"transactionsRoot" gencodec:"required"`
		ReceiptHash *common.Hash     `json:"receiptsRoot"     gencodec:"required"`
		DposContext *DposContextRoot `json:"dposContext"      gencodec:"required"`
		Bloom       *Bloom           `json:"logsBloom"        gencodec:"required"`
		Difficulty  *hexutil.Big     `json:"difficulty"       gencodec:"required"`
		Number      *hexutil.Big     `json:"number"           gencodec:"required"`
		GasLimit    *hexutil.Uint64  `json:"gasLimit"         gencodec:"required"`
		GasUsed     *hexutil.Uint64  `json:"gasUsed"          gencodec:"required"`
		Time        *hexutil.Big     `json:"timestamp"        gencodec:"required"`
		Extra       *hexutil.Bytes   `json:"extraData"        gencodec:"required"`
		MixDigest   *common.Hash     `json:"mixHash"`
		Nonce       *BlockNonce      `json:"nonce"`
	}

	var dec Header
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	if dec.ParentHash == nil {
		return errors.New("missing required field 'parentHash' for Header")
	}
	h.ParentHash = *dec.ParentHash

	if dec.UncleHash == nil {
		return errors.New("missing required field 'sha3Uncles' for Header")
	}
	h.UncleHash = *dec.UncleHash

	if dec.Validator == nil {
		return errors.New("missing required filed 'validator' for Header")
	}
	h.Validator = *dec.Validator

	if dec.Coinbase == nil {
		return errors.New("missing required field 'coinbase' for Header")
	}
	h.Coinbase = *dec.Coinbase

	if dec.Root == nil {
		return errors.New("missing required field 'stateRoot' for Header")
	}
	h.Root = *dec.Root

	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionsRoot' for Header")
	}
	h.TxHash = *dec.TxHash

	if dec.ReceiptHash == nil {
		return errors.New("missing required field 'receiptsRoot' for Header")
	}
	h.ReceiptHash = *dec.ReceiptHash

	if dec.Bloom == nil {
		return errors.New("missing required field 'logsBloom' for Header")
	}
	h.Bloom = *dec.Bloom

	if dec.DposContext == nil {
		return errors.New("missing required filed 'dposContext' for Header")
	}
	h.DposContext = dec.DposContext

	if dec.Difficulty == nil {
		return errors.New("missing required field 'difficulty' for Header")
	}
	h.Difficulty = (*big.Int)(dec.Difficulty)

	if dec.Number == nil {
		return errors.New("missing required field 'number' for Header")
	}
	h.Number = (*big.Int)(dec.Number)

	if dec.GasLimit == nil {
		return errors.New("missing required field 'gasLimit' for Header")
	}
	h.GasLimit = uint64(*dec.GasLimit)

	if dec.GasUsed == nil {
		return errors.New("missing required field 'gasUsed' for Header")
	}
	h.GasUsed = uint64(*dec.GasUsed)

	if dec.Time == nil {
		return errors.New("missing required field 'timestamp' for Header")
	}
	h.Time = (*big.Int)(dec.Time)

	if dec.Extra == nil {
		return errors.New("missing required field 'extraData' for Header")
	}
	h.Extra = *dec.Extra

	if dec.MixDigest != nil {
		h.MixDigest = *dec.MixDigest
	}

	if dec.Nonce != nil {
		h.Nonce = *dec.Nonce
	}
	return nil
}

var shaPool = sync.Pool{
	New: func() interface{} {
		return sha3.NewLegacyKeccak256()
	},
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := shaPool.Get().(hash.Hash)
	defer func() {
		hw.Reset()
		shaPool.Put(hw)
	}()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions []*Transaction
	Uncles       []*Header
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions

	// dpos consensus context
	dposContext *DposContext

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
// Deprecated: relic method to get difficulty. do not use
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}

// [deprecated by eth/63]
// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
type StorageBlock Block

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
}

// [deprecated by eth/63]
// "storage" block encoding. used for database.
type storageblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
	TD     *big.Int
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewBlock(header *Header, txs []*Transaction, uncles []*Header, receipts []*Receipt) *Block {
	b := &Block{header: CopyHeader(header), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	if len(uncles) == 0 {
		b.header.UncleHash = EmptyUncleHash
	} else {
		b.header.UncleHash = CalcUncleHash(uncles)
		b.uncles = make([]*Header, len(uncles))
		for i := range uncles {
			b.uncles[i] = CopyHeader(uncles[i])
		}
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}

	// add dposContextRoot to header
	cpy.DposContext = &DposContextRoot{}
	if h.DposContext != nil {
		cpy.DposContext = h.DposContext
	}

	return &cpy
}

// DecodeRLP decodes the Ethereum RLP block format to original block
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions = eb.Header, eb.Uncles, eb.Txs
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header: b.header,
		Txs:    b.transactions,
		Uncles: b.uncles,
	})
}

// [deprecated by eth/63]
func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.td = sb.Header, sb.Uncles, sb.Txs, sb.TD
	return nil
}

// TODO: copies

func (b *Block) Uncles() []*Header          { return b.uncles }
func (b *Block) Transactions() Transactions { return b.transactions }

func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *Block) Number() *big.Int     { return new(big.Int).Set(b.header.Number) }
func (b *Block) GasLimit() uint64     { return b.header.GasLimit }
func (b *Block) GasUsed() uint64      { return b.header.GasUsed }
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *Block) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }

func (b *Block) NumberU64() uint64         { return b.header.Number.Uint64() }
func (b *Block) MixDigest() common.Hash    { return b.header.MixDigest }
func (b *Block) Nonce() uint64             { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *Block) Bloom() Bloom              { return b.header.Bloom }
func (b *Block) Coinbase() common.Address  { return b.header.Coinbase }
func (b *Block) Root() common.Hash         { return b.header.Root }
func (b *Block) ParentHash() common.Hash   { return b.header.ParentHash }
func (b *Block) TxHash() common.Hash       { return b.header.TxHash }
func (b *Block) ReceiptHash() common.Hash  { return b.header.ReceiptHash }
func (b *Block) UncleHash() common.Hash    { return b.header.UncleHash }
func (b *Block) Extra() []byte             { return common.CopyBytes(b.header.Extra) }
func (b *Block) Validator() common.Address { return b.header.Validator }
func (b *Block) DposCtx() *DposContext     { return b.dposContext }

func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body { return &Body{b.transactions, b.uncles} }

// size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func CalcUncleHash(uncles []*Header) common.Hash {
	return rlpHash(uncles)
}

func (b *Block) SetDposCtx(dposCtx *DposContext) {
	b.dposContext = dposCtx
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header

	return &Block{
		header:       &cpy,
		transactions: b.transactions,
		uncles:       b.uncles,
		dposContext:  b.dposContext,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, uncles []*Header) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
		uncles:       make([]*Header, len(uncles)),
	}
	copy(block.transactions, transactions)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

type Blocks []*Block

// BlockBy is the function to be used for block sorting.
type BlockBy func(b1, b2 *Block) bool

// Sort is the public function to be used for sorting Blocks
func (b BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     b,
	}
	sort.Sort(bs)
}

// BlockSorter is the implementation for Sorter interface.
// The comparator algorithm is defined by 'bs.by'
type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

// Number is the default BlockBy function, which compares b.header.Number
// and return blocks in increasing order
func Number(b1, b2 *Block) bool { return b1.header.Number.Cmp(b2.header.Number) < 0 }
