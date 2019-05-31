// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

const (
	PersistContractManagerHeader  = "Storage Contract Manager Settings"
	PersistContractManagerVersion = "1.0"
	PersistFileName               = "storagecontractmanager.json"
)

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
)
