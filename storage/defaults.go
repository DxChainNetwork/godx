// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"errors"
	"math/big"
	"time"

	"github.com/DxChainNetwork/godx/common/unit"

	"github.com/DxChainNetwork/godx/common"
)

var (
	// ErrHostBusyHandleReq defines that client sent the contract request too frequently. If this error is occurred
	// the host's evaluation will not be deducted
	ErrHostBusyHandleReq = errors.New("client must wait until the host finish its's previous request")

	// ErrClientNegotiate defines that client occurs error while negotiate
	ErrClientNegotiate = errors.New("client negotiate error")

	// ErrClientCommit defines that client occurs error while commit(finalize)
	ErrClientCommit = errors.New("client commit error")

	// ErrHostNegotiate defines that client occurs error while negotiate
	ErrHostNegotiate = errors.New("host negotiate error")

	// ErrHostCommit defines that host occurs error while commit(finalize)
	ErrHostCommit = errors.New("host commit error")
)

// Negotiation related messages
const (
	// Client Handle Message Set
	HostConfigRespMsg            = 0x20
	ContractCreateHostSign       = 0x21
	ContractCreateRevisionSign   = 0x22
	ContractUploadMerkleProofMsg = 0x23
	ContractUploadRevisionSign   = 0x24
	ContractDownloadDataMsg      = 0x25
	HostBusyHandleReqMsg         = 0x26
	HostCommitFailedMsg          = 0x27
	HostAckMsg                   = 0x28
	HostNegotiateErrorMsg        = 0x29

	// Host Handle Message Set
	HostConfigReqMsg                 = 0x30
	ContractCreateReqMsg             = 0x31
	ContractCreateClientRevisionSign = 0x32
	ContractUploadReqMsg             = 0x33
	ContractUploadClientRevisionSign = 0x34
	ContractDownloadReqMsg           = 0x35
	ClientCommitSuccessMsg           = 0x36
	ClientCommitFailedMsg            = 0x37
	ClientAckMsg                     = 0x38
	ClientNegotiateErrorMsg          = 0x39
)

const (
	RenewWindow = 12 * unit.BlocksPerHour
)

// The block generation rate for Ethereum is 15s/block. Therefore, 240 blocks
// can be generated in an hour
var (
	ResponsibilityLockTimeout = 60 * time.Second
)

// Default rentPayment values
var (
	DefaultRentPayment = RentPayment{
		Fund:         common.PtrBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)).MultInt64(1e4),
		StorageHosts: 3,
		Period:       3 * unit.BlocksPerDay,

		// TODO: remove these fields
		ExpectedStorage:    1e12,                                // 1 TB
		ExpectedUpload:     uint64(200e9) / unit.BlocksPerMonth, // 200 GB per month
		ExpectedDownload:   uint64(100e9) / unit.BlocksPerMonth, // 100 GB per month
		ExpectedRedundancy: 2.0,
	}
)
