// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storage

import (
	"github.com/DxChainNetwork/godx/common"

	"math/big"
	"time"
)

const (
	IDLE = 0
	BUSY = 1

	// Upload Data Segment Code Msg
	StorageContractUploadRequestMsg         = 0x26
	StorageContractUploadMerkleRootProofMsg = 0x27
	StorageContractUploadClientRevisionMsg  = 0x28
	StorageContractUploadHostRevisionMsg    = 0x29

	// Download Data Segment Code Msg
	StorageContractDownloadRequestMsg      = 0x34
	StorageContractDownloadDataMsg         = 0x35
	StorageContractDownloadHostRevisionMsg = 0x36
	// error msg code
	NegotiationErrorMsg = 0x37
	// stop msg code
	NegotiationStopMsg = 0x38

	////////////////////////////////////

	// Client Handle Message Set
	HostConfigRespMsg          = 0x20
	ContractCreateHostSign     = 0x21
	ContractCreateRevisionSign = 0x22
	UploadMerkleProofMsg       = 0x23

	// Host Handle Message Set
	HostConfigReqMsg                 = 0x30
	ContractCreateReqMsg             = 0x31
	ContractCreateClientRevisionSign = 0x32
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
