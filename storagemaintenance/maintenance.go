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

	// StrPrefixExpSC is the prefix string for construct contract status address
	StrPrefixExpSC = "ExpiredStorageContract_"

	// ProofedStatus indicate the contract that is proofed
	ProofedStatus = []byte{'1'}

	// NotProofedStatus indicate the contract that is not proofed
	NotProofedStatus = []byte{'0'}

	// KeyClientCollateral is the key to store client collateral value into trie
	KeyClientCollateral = common.BytesToHash([]byte("ClientCollateral"))

	// KeyHostCollateral is the key to store host collateral value into trie
	KeyHostCollateral = common.BytesToHash([]byte("HostCollateral"))

	// KeyFileSize is the key to store file size into trie
	KeyFileSize = common.BytesToHash([]byte("FileSize"))

	// KeyUnlockHash is the key to store unlock hash into trie
	KeyUnlockHash = common.BytesToHash([]byte("UnlockHash"))

	// KeyFileMerkleRoot is the key to store file merkle root into trie
	KeyFileMerkleRoot = common.BytesToHash([]byte("FileMerkleRoot"))

	// KeyRevisionNumber is the key to store revision number into trie
	KeyRevisionNumber = common.BytesToHash([]byte("RevisionNumber"))

	// KeyWindowStart is the key to store window start into trie
	KeyWindowStart = common.BytesToHash([]byte("WindowStart"))

	// KeyWindowEnd is the key to store window end into trie
	KeyWindowEnd = common.BytesToHash([]byte("WindowEnd"))

	// KeyClientAddress is the key to store client address into trie
	KeyClientAddress = common.BytesToHash([]byte("ClientAddress"))

	// KeyHostAddress is the key to store host address into trie
	KeyHostAddress = common.BytesToHash([]byte("HostAddress"))

	// KeyClientValidProofOutput is the key to store client valid proof output into trie
	KeyClientValidProofOutput = common.BytesToHash([]byte("ClientValidProofOutput"))

	// KeyClientMissedProofOutput is the key to store client missed proof output into trie
	KeyClientMissedProofOutput = common.BytesToHash([]byte("ClientMissedProofOutput"))

	// KeyHostValidProofOutput is the key to store host valid proof output into trie
	KeyHostValidProofOutput = common.BytesToHash([]byte("HostValidProofOutput"))

	// KeyHostMissedProofOutput is the key to store host missed proof output into trie
	KeyHostMissedProofOutput = common.BytesToHash([]byte("HostMissedProofOutput"))
)

// MaintenanceMissedProof maintains missed storage proof
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

		// mark the statusAddr as empty account, that will be deleted by stateDB
		state.SetNonce(statusAddr, 0)
	}
}
