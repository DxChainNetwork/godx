// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package hostnegotiation

import (
	"github.com/DxChainNetwork/godx/crypto/merkle"
	"github.com/DxChainNetwork/godx/storage"
)

const (
	expectedValidProofPaybackCounts      = 2
	expectedMissedProofPaybackCounts     = 2
	validProofPaybackHostAddressIndex    = 1
	missedProofPaybackHostAddressIndex   = 1
	validProofPaybackClientAddressIndex  = 0
	missedProofPaybackClientAddressIndex = 0
	contractRequiredSignatures           = 2
	hostSignIndex                        = 1
)

// sectorHeight is the height of the merkle tree constructed
// based on the data uploaded. Data uploaded will be divided
// into data pieces based on the LeafSize
var sectorHeight = func() uint64 {
	height := uint64(0)
	for 1<<height < (storage.SectorSize / merkle.LeafSize) {
		height++
	}
	return height
}()
