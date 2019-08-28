// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
)

// persistent related constants
const (
	PersistContractManagerHeader  = "Storage Contract Manager Settings"
	PersistContractManagerVersion = "1.0"
	PersistFileName               = "storagecontractmanager.json"
)

// maintenance related constants
const (
	randomStorageHostsBackup = 30
	randomStorageHostsFactor = 4

	// evalFactor defines the factor that needs to be
	// divided by the calculated min evaluation, which
	// is used to determine if the contract should be
	// prohibited to upload and renew
	evalFactor                         = uint64(100)
	minClientBalanceUploadThreshold    = float64(0.05)
	minContractPaymentRenewalThreshold = float64(0.06)
	minContractPaymentFactor           = float64(0.15)
	maturityDelay                      = uint64(5)

	// if a contract failed to renew for 12 times, consider to replace the contract
	consecutiveRenewFailsBeforeReplacement = 12
	extraRatio                             = 0.02
)

// variables below are used to calculate the maxHostStoragePrice and maxHostDeposit, which set
// a limitation to storage host's configuration
var (
	terabytesToBytes           = common.NewBigIntUint64(1e12)
	blockBytesPerMonthTeraByte = terabytesToBytes.MultUint64(4320)
	maxHostStoragePrice        = common.PtrBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)).MultUint64(300e3).Div(blockBytesPerMonthTeraByte)
	maxHostDeposit             = common.PtrBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)).MultUint64(1e3)
)

// ErrHostFault indicates if the error is caused by the storage host
var (
	ErrHostFault = errors.New("host has returned an error")
)
