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

package eth

import (
	"fmt"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/p2p"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/rlp"
)

// Constants to match up protocol versions and messages
const (
	eth62 = 62
	eth63 = 63
	eth64 = 64
)

// ProtocolVersions are the supported versions of the eth protocol (first is primary).
var ProtocolVersions = []uint{eth64, eth63, eth62}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{100, 17, 8}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// eth protocol message codes
const (
	// Protocol messages belonging to eth/62
	StatusMsg          = 0x00
	NewBlockHashesMsg  = 0x01
	TxMsg              = 0x02
	GetBlockHeadersMsg = 0x03
	BlockHeadersMsg    = 0x04
	GetBlockBodiesMsg  = 0x05
	BlockBodiesMsg     = 0x06
	NewBlockMsg        = 0x07

	// Protocol messages belonging to eth/63
	GetNodeDataMsg = 0x0d
	NodeDataMsg    = 0x0e
	GetReceiptsMsg = 0x0f
	ReceiptsMsg    = 0x10

	// Protocol messages for dpos
	GetBlockHeaderAndValidatorsMsg = 0x11
	BlockHeaderAndValidatorsMsg    = 0x12
)

type errCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
	ErrUnexpectedResponse
)

func (e errCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
	ErrUnexpectedResponse:      "Unexpected response",
}

type txPool interface {
	// AddRemotes should add the given transactions to the pool.
	AddRemotes([]*types.Transaction) []error

	// Pending should return pending transactions.
	// The slice should be modifiable by the caller.
	Pending() (map[common.Address]types.Transactions, error)

	// SubscribeNewTxsEvent should return an event subscription of
	// NewTxsEvent and send events to the given channel.
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
}

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	TD              *big.Int
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
}

// newBlockHashesData is the network packet for the block announcements.
type newBlockHashesData []struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
}

// getBlockHeadersData represents a block header query.
type getBlockHeadersData struct {
	Origin  hashOrNumber // Block from which to retrieve headers
	Amount  uint64       // Maximum number of headers to retrieve
	Skip    uint64       // Blocks to skip between consecutive headers
	Reverse bool         // Query direction (false = rising towards latest, true = falling towards genesis)
}

// hashOrNumber is a combined field for specifying an origin block.
type hashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

func isHashMode(hon hashOrNumber) bool {
	return hon.Hash == common.Hash{}
}

// EncodeRLP is a specialized encoder for hashOrNumber to encode only one of the
// two contained union fields.
func (hn *hashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for hashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *hashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	origin, err := s.Raw()
	if err == nil {
		switch {
		case size == 32:
			err = rlp.DecodeBytes(origin, &hn.Hash)
		case size <= 8:
			err = rlp.DecodeBytes(origin, &hn.Number)
		default:
			err = fmt.Errorf("invalid input size %d for origin", size)
		}
	}
	return err
}

// newBlockData is the network packet for the block propagation message.
type newBlockData struct {
	Block *types.Block
	TD    *big.Int
}

// blockBody represents the data content of a single block.
type blockBody struct {
	Transactions []*types.Transaction // Transactions contained within a block
	Uncles       []*types.Header      // Uncles contained within a block
}

// blockBodiesData is the network packet for block content distribution.
type blockBodiesData []*blockBody

type getBlockHeaderAndValidatorsRequestWithID struct {
	ReqID uint64
	Query getBlockHeaderAndValidatorsRequest
}

func decodeGetBlockHeaderAndValidatorsRequests(msg p2p.Msg) (getBlockHeaderAndValidatorsRequestWithID, error) {
	var req getBlockHeaderAndValidatorsRequestWithID
	if err := msg.Decode(&req); err != nil {
		return getBlockHeaderAndValidatorsRequestWithID{}, err
	}
	return req, nil
}

type getBlockHeaderAndValidatorsRequest struct {
	Origin  hashOrNumber
	Amount  uint64
	Skip    uint64
	Reverse bool
}

// HeaderAndValidatorData is the data structure for BlockHeaderAndValidatorsMsg
type HeaderAndValidatorData struct {
	data types.HeaderInsertDataBatch
}

// EncodeRLP defines the rlp encoding rule for HeaderAndValidatorData.
func (data *HeaderAndValidatorData) EncodeRLP(w io.Writer) error {
	compressed := compressHeaderInsertDataBatch(data.data)
	return rlp.Encode(w, compressed)
}

// DecodeRLP defines the rlp decoding rule for HeaderAndValidatorData.
func (data *HeaderAndValidatorData) DecodeRLP(s *rlp.Stream) error {
	var compressed types.HeaderInsertDataBatch
	if err := s.Decode(&compressed); err != nil {
		return fmt.Errorf("cannot decode HeaderAndValidatorData: %v", err)
	}
	rawData, err := decompressHeaderInsertDataBatch(compressed)
	if err != nil {
		return fmt.Errorf("cannot decode HeaderAndValidatorData: %v", err)
	}
	data.data = rawData
	return nil
}

// compressHeaderInsertDataBatch compress the types.HeaderInsertDataBatch, validators will be omitted for the latter one of the two
// consecutive data. Thus the same validators will not be send through network again and again.
func compressHeaderInsertDataBatch(raw types.HeaderInsertDataBatch) types.HeaderInsertDataBatch {
	compressed := make(types.HeaderInsertDataBatch, 0, len(raw))
	var lastValidators []common.Address
	for _, entry := range raw {
		if isValidatorsEqual(entry.Validators, lastValidators) {
			compressed = append(compressed, types.HeaderInsertData{
				Header: entry.Header,
			})
		} else {
			compressed = append(compressed, types.HeaderInsertData{
				Header:     entry.Header,
				Validators: entry.Validators,
			})
			lastValidators = entry.Validators
		}
	}
	return compressed
}

func isValidatorsEqual(validators1, validators2 []common.Address) bool {
	if len(validators1) != len(validators2) {
		return false
	}
	for i := range validators1 {
		val1, val2 := validators1[i], validators2[i]
		if val1 != val2 {
			return false
		}
	}
	return true
}

// decompressHeaderInsertDataBatch decompress the compressed data batch.
func decompressHeaderInsertDataBatch(compressed types.HeaderInsertDataBatch) (types.HeaderInsertDataBatch, error) {
	rawData := make(types.HeaderInsertDataBatch, 0, len(compressed))
	if len(compressed[0].Validators) == 0 {
		return nil, fmt.Errorf("first entry does not have validators")
	}
	var lastValidators []common.Address
	for _, entry := range compressed {
		rawEntry := types.HeaderInsertData{Header: entry.Header}
		if entry.Validators == nil {
			rawEntry.Validators = make([]common.Address, len(lastValidators))
			copy(rawEntry.Validators, lastValidators)
		} else {
			rawEntry.Validators = make([]common.Address, len(entry.Validators))
			copy(rawEntry.Validators, entry.Validators)
			lastValidators = entry.Validators
		}
		rawData = append(rawData, rawEntry)
	}
	return rawData, nil
}
