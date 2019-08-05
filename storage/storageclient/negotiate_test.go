// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

var (
	clientAddress = common.HexToAddress("0xb639db6974c87ff799820089761d7bee72d23e1b")
	hostAddress   = common.HexToAddress("0x5f144608ca454a66dd3d7f11089a5ede0721e583")
)

func TestNewRevision(t *testing.T) {

	// mock last revision
	currentRev := types.StorageContractRevision{
		NewValidProofOutputs: []types.DxcoinCharge{
			{Address: clientAddress, Value: new(big.Int).SetInt64(2000000)},
			{Address: hostAddress, Value: new(big.Int).SetInt64(1000000)},
		},
		NewMissedProofOutputs: []types.DxcoinCharge{
			{Address: clientAddress, Value: new(big.Int).SetInt64(1900000)},
			{Address: hostAddress, Value: new(big.Int).SetInt64(1000000)},
		},
	}

	// calculate present revision
	cost := new(big.Int).SetInt64(10000)
	newRev := NewRevision(currentRev, cost)

	// check the new revision
	if newRev.NewValidProofOutputs[0].Value.Int64() != (2000000 - 10000) {
		t.Errorf("wrong new client valid output,wanted %d,getted %d", (2000000 - 10000), newRev.NewValidProofOutputs[0].Value.Int64())
	}

	if newRev.NewValidProofOutputs[1].Value.Int64() != (1000000 + 10000) {
		t.Errorf("wrong new host valid output,wanted %d,getted %d", (2000000 + 10000), newRev.NewValidProofOutputs[1].Value.Int64())
	}

	if newRev.NewMissedProofOutputs[0].Value.Int64() != (1900000 - 10000) {
		t.Errorf("wrong new client missed output,wanted %d,getted %d", (1900000 - 10000), newRev.NewMissedProofOutputs[0].Value.Int64())
	}

	if newRev.NewMissedProofOutputs[1].Value.Int64() != 1000000 {
		t.Errorf("wrong new host missed output,wanted %d,getted %d", 1000000, newRev.NewMissedProofOutputs[1].Value.Int64())
	}
}
