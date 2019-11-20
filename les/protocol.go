// Copyright 2016 The go-ethereum Authors
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

// Package les implements the Light Ethereum Subprotocol.
package les

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/rawdb"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/light"
	"github.com/DxChainNetwork/godx/p2p"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
)

// Constants to match up protocol versions and messages
const (
	lpv1 = 1
	lpv2 = 2
)

// Supported versions of the les protocol (first is primary)
var (
	ClientProtocolVersions    = []uint{lpv2, lpv1}
	ServerProtocolVersions    = []uint{lpv2, lpv1}
	AdvertiseProtocolVersions = []uint{lpv2} // clients are searching for the first advertised protocol in the list
)

// Number of implemented message corresponding to different protocol versions.
var ProtocolLengths = map[uint]uint64{lpv1: 15, lpv2: 22}

const (
	NetworkId          = 1
	ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message
)

// les protocol message codes
const (
	// Protocol messages belonging to LPV1
	StatusMsg          = 0x00
	AnnounceMsg        = 0x01
	GetBlockHeadersMsg = 0x02
	BlockHeadersMsg    = 0x03
	GetBlockBodiesMsg  = 0x04
	BlockBodiesMsg     = 0x05
	GetReceiptsMsg     = 0x06
	ReceiptsMsg        = 0x07
	GetProofsV1Msg     = 0x08
	ProofsV1Msg        = 0x09
	GetCodeMsg         = 0x0a
	CodeMsg            = 0x0b
	SendTxMsg          = 0x0c
	GetHeaderProofsMsg = 0x0d
	HeaderProofsMsg    = 0x0e
	// Protocol messages belonging to LPV2
	GetProofsV2Msg         = 0x0f
	ProofsV2Msg            = 0x10
	GetHelperTrieProofsMsg = 0x11
	HelperTrieProofsMsg    = 0x12
	SendTxV2Msg            = 0x13
	GetTxStatusMsg         = 0x14
	TxStatusMsg            = 0x15
	// Protocol for Dpos
	GetDposProofMsg                = 0x16
	DposProofMsg                   = 0x17
	GetBlockHeaderAndValidatorsMsg = 0x18
	BlockHeaderAndValidatorsMsg    = 0x19
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
	ErrUselessPeer
	ErrRequestRejected
	ErrUnexpectedResponse
	ErrInvalidResponse
	ErrTooManyTimeouts
	ErrMissingKey
	ErrUnknownValidators
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
	ErrRequestRejected:         "Request rejected",
	ErrUnexpectedResponse:      "Unexpected response",
	ErrInvalidResponse:         "Invalid response",
	ErrTooManyTimeouts:         "Too many request timeouts",
	ErrMissingKey:              "Key missing from list",
	ErrUnknownValidators:       "Cannot open epoch trie",
}

type announceBlock struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
	Td     *big.Int    // Total difficulty of one particular block being announced
}

// announceData is the network packet for the block announcements.
type announceData struct {
	Hash       common.Hash // Hash of one particular block being announced
	Number     uint64      // Number of one particular block being announced
	Td         *big.Int    // Total difficulty of one particular block being announced
	ReorgDepth uint64
	Update     keyValueList
}

// sign adds a signature to the block announcement by the given privKey
func (a *announceData) sign(privKey *ecdsa.PrivateKey) {
	rlp, _ := rlp.EncodeToBytes(announceBlock{a.Hash, a.Number, a.Td})
	sig, _ := crypto.Sign(crypto.Keccak256(rlp), privKey)
	a.Update = a.Update.add("sign", sig)
}

// checkSignature verifies if the block announcement has a valid signature by the given pubKey
func (a *announceData) checkSignature(id enode.ID) error {
	var sig []byte
	if err := a.Update.decode().get("sign", &sig); err != nil {
		return err
	}
	rlp, _ := rlp.EncodeToBytes(announceBlock{a.Hash, a.Number, a.Td})
	recPubkey, err := crypto.SigToPub(crypto.Keccak256(rlp), sig)
	if err != nil {
		return err
	}
	if id == enode.PubkeyToIDV4(recPubkey) {
		return nil
	}
	return errors.New("wrong signature")
}

type blockInfo struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
	Td     *big.Int    // Total difficulty of one particular block being announced
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
	return hon.Hash != common.Hash{}
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

// CodeData is the network response packet for a node data retrieval.
type CodeData []struct {
	Value []byte
}

type proofsData [][]rlp.RawValue

type txStatus struct {
	Status core.TxStatus
	Lookup *rawdb.TxLookupEntry `rlp:"nil"`
	Error  string
}

type getDposProofRequestPacket struct {
	ReqID uint64
	Reqs  []DposProofReq
}

func decodeGetDposProofMsg(msg p2p.Msg) (getDposProofRequestPacket, error) {
	var req getDposProofRequestPacket
	if err := msg.Decode(&req); err != nil {
		return getDposProofRequestPacket{}, err
	}
	return req, nil
}

type dposProofRequestPacket struct {
	ReqID, BV uint64
	Data      light.NodeList
}

func decodeDposProofRequestMsg(msg p2p.Msg) (dposProofRequestPacket, error) {
	var req dposProofRequestPacket
	if err := msg.Decode(&req); err != nil {
		return dposProofRequestPacket{}, err
	}
	return req, nil
}

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
