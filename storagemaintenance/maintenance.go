// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagemaintenance

import (
	"bytes"
	"math/big"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/state"
)

var (
	StrPrefixExpSC = "ExpiredStorageContract_"

	ProofedStatus    = []byte{'1'}
	NotProofedStatus = []byte{'0'}

	KeyClientCollateral        = common.BytesToHash([]byte("ClientCollateral"))
	KeyHostCollateral          = common.BytesToHash([]byte("HostCollateral"))
	KeyFileSize                = common.BytesToHash([]byte("FileSize"))
	KeyUnlockHash              = common.BytesToHash([]byte("UnlockHash"))
	KeyFileMerkleRoot          = common.BytesToHash([]byte("FileMerkleRoot"))
	KeyRevisionNumber          = common.BytesToHash([]byte("RevisionNumber"))
	KeyWindowStart             = common.BytesToHash([]byte("WindowStart"))
	KeyWindowEnd               = common.BytesToHash([]byte("WindowEnd"))
	KeyClientAddress           = common.BytesToHash([]byte("ClientAddress"))
	KeyHostAddress             = common.BytesToHash([]byte("HostAddress"))
	KeyClientValidProofOutput  = common.BytesToHash([]byte("ClientValidProofOutput"))
	KeyClientMissedProofOutput = common.BytesToHash([]byte("ClientMissedProofOutput"))
	KeyHostValidProofOutput    = common.BytesToHash([]byte("HostValidProofOutput"))
	KeyHostMissedProofOutput   = common.BytesToHash([]byte("HostMissedProofOutput"))
)

// maintenance missed storage proof
func MaintenanceMissedProof(height uint64, state *state.StateDB) {
	windowEndStr := strconv.FormatUint(height, 10)
	statusAddr := common.BytesToAddress([]byte(StrPrefixExpSC + windowEndStr))

	if state.Exist(statusAddr) {
		state.ForEachStorage(statusAddr, func(key, value common.Hash) bool {
			flag := value.Bytes()[11:12]
			if bytes.Equal(flag, NotProofedStatus) {
				contractAddr := common.BytesToAddress(value[12:])

				// retrieve storage contract filed data
				clientAddressHash := state.GetState(contractAddr, KeyClientAddress)
				hostAddressHash := state.GetState(contractAddr, KeyHostAddress)
				clientMpoHash := state.GetState(contractAddr, KeyClientMissedProofOutput)
				hostMpoHash := state.GetState(contractAddr, KeyHostMissedProofOutput)

				// return back the remain amount to client and host
				clientMpo := new(big.Int).SetBytes(clientMpoHash.Bytes())
				hostMpo := new(big.Int).SetBytes(hostMpoHash.Bytes())
				state.AddBalance(common.BytesToAddress(clientAddressHash.Bytes()), clientMpo)
				state.AddBalance(common.BytesToAddress(hostAddressHash.Bytes()), hostMpo)

				// deduct the sum missed output from contract account
				totalValue := new(big.Int).Add(clientMpo, hostMpo)
				state.SubBalance(contractAddr, totalValue)
			}
			return true
		})
	}
}
