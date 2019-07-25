// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"

	"math/big"
	"time"
)

var (
	// HostBusyHandleReqErr defines that client sent the contract request too frequently. If this error is occurred
	// the host's evaluation will not be deducted
	HostBusyHandleReqErr = errors.New("client must wait until the host finish its's previous request")


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
	NegotiationStopMsg               = 0x36
	ClientCommitSuccessMsg           = 0x37
	ClientCommitFailedMsg            = 0x38
	ClientAckMsg                     = 0x39
	ClientNegotiateErrorMsg          = 0x40
)

// The block generation rate for Ethereum is 15s/block. Therefore, 240 blocks
// can be generated in an hour
var (
	BlockPerMin    = uint64(4)
	BlockPerHour   = uint64(240)
	BlocksPerDay   = 24 * BlockPerHour
	BlocksPerWeek  = 7 * BlocksPerDay
	BlocksPerMonth = 30 * BlocksPerDay
	BlocksPerYear  = 365 * BlocksPerDay

	ResponsibilityLockTimeout = 60 * time.Second
)

// Default rentPayment values
var (
	DefaultRentPayment = RentPayment{
		Fund:         common.PtrBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
		StorageHosts: 3,
		Period:       3 * BlocksPerDay,
		RenewWindow:  12 * BlockPerHour,

		ExpectedStorage:    1e12,                           // 1 TB
		ExpectedUpload:     uint64(200e9) / BlocksPerMonth, // 200 GB per month
		ExpectedDownload:   uint64(100e9) / BlocksPerMonth, // 100 GB per month
		ExpectedRedundancy: 2.0,
	}
)
