// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"math/big"

	"github.com/DxChainNetwork/godx/core/types"
)

// update current storage contract revision with its revision number incremented, and cost transferred from the client to the host.
func NewRevision(current types.StorageContractRevision, cost *big.Int) types.StorageContractRevision {
	rev := current

	rev.NewValidProofOutputs = make([]types.DxcoinCharge, 2)
	rev.NewMissedProofOutputs = make([]types.DxcoinCharge, 2)

	for i, v := range current.NewValidProofOutputs {
		rev.NewValidProofOutputs[i] = types.DxcoinCharge{
			Address: v.Address,
			Value:   big.NewInt(v.Value.Int64()),
		}
	}

	for i, v := range current.NewMissedProofOutputs {
		rev.NewMissedProofOutputs[i] = types.DxcoinCharge{
			Address: v.Address,
			Value:   big.NewInt(v.Value.Int64()),
		}
	}

	// move valid payout from client to host
	rev.NewValidProofOutputs[0].Value.Sub(current.NewValidProofOutputs[0].Value, cost)
	rev.NewValidProofOutputs[1].Value.Add(current.NewValidProofOutputs[1].Value, cost)

	// move missed payout from client to void, mean that will burn missed payout of client
	rev.NewMissedProofOutputs[0].Value.Sub(current.NewMissedProofOutputs[0].Value, cost)

	// increment revision number
	rev.NewRevisionNumber++

	return rev
}
