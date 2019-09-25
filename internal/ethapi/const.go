// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package ethapi

const (
	// StorageContractTxGas defines the default gas for storage contract tx
	StorageContractTxGas = 90000

	// DposTxGas defines the default gas for dpos tx
	DposTxGas = 1000000
)

var (
	// pre-compiled contracts addresses
	ApplyCandidateContractAddr  = []byte{13}
	CancelCandidateContractAddr = []byte{14}
	VoteContractAddr            = []byte{15}
	CancelVoteContractAddr      = []byte{16}
)
