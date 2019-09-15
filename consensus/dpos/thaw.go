// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"strconv"

	"github.com/DxChainNetwork/godx/common"
)

// MarkThawingAddress mark the given addr that will be thawed in next next epoch
func MarkThawingAddress(stateDB stateDB, addr common.Address, currentEpoch int64, keyPrefix string) {
	// create thawing address: "thawing_" + currentEpoch
	thawingAddress := getThawingAddress(currentEpoch)
	if !stateDB.Exist(thawingAddress) {
		stateDB.CreateAccount(thawingAddress)
		// before thawing deposit, mark thawingAddress as not empty account to avoid being deleted by stateDB
		stateDB.SetNonce(thawingAddress, 1)
	}
	// set thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	setAddrInThawingAddress(stateDB, thawingAddress, addr, keyPrefix)
}

// ThawingDeposit thawing the deposit for the candidate or delegator cancel in currentEpoch-2
func ThawingDeposit(stateDB stateDB, currentEpoch int64) {
	// create thawing address: "thawing_" + currentEpochID
	thawingAddress := getThawingAddress(currentEpoch)
	if stateDB.Exist(thawingAddress) {
		// iterator whole thawing account trie, and thawing the deposit of every delegator
		stateDB.ForEachStorage(thawingAddress, func(key, value common.Hash) bool {
			if value == (common.Hash{}) {
				return false
			}

			addr := common.BytesToAddress(value.Bytes()[12:])

			// if candidate deposit thawing flag exists, then thawing it
			candidateThawingKey := append([]byte(PrefixCandidateThawing), addr.Bytes()...)
			if common.BytesToHash(candidateThawingKey) == value {

				// set 0 for candidate deposit
				setCandidateDeposit(stateDB, addr, common.BigInt0)
			}

			// if vote deposit thawing flag exists, then thawing it
			voteThawingKey := append([]byte(PrefixVoteThawing), addr.Bytes()...)
			if common.BytesToHash(voteThawingKey) == value {

				// set 0 for vote deposit
				setVoteDeposit(stateDB, addr, common.BigInt0)
			}

			// candidate or vote deposit does not allow to submit repeatedly, so thawing directly set 0
			stateDB.SetState(thawingAddress, key, common.Hash{})
			return true
		})

		// mark the thawingAddress as empty account, that will be deleted by stateDB
		stateDB.SetNonce(thawingAddress, 0)
	}
}

// getThawingAddress return the thawing address with the epoch
func getThawingAddress(epoch int64) common.Address {
	epochIDStr := strconv.FormatInt(epoch, 10)
	thawingAddress := common.BytesToAddress([]byte(PrefixThawingAddr + epochIDStr))
	return thawingAddress
}

// setAddrInThawingAddress set the address in the thawing address
func setAddrInThawingAddress(state stateDB, thawingAddress, addr common.Address, keyPrefix string) {
	// set thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	keyAndValue := makeThawingAddressKey(keyPrefix, addr)
	state.SetState(thawingAddress, keyAndValue, keyAndValue)
}

// makeThawingAddressKey makes the key / value for the thawing address
func makeThawingAddressKey(keyPrefix string, addr common.Address) common.Hash {
	// thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	b := append([]byte(keyPrefix), addr.Bytes()...)
	return common.BytesToHash(b)
}
