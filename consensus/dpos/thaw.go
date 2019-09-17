// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"

	"github.com/DxChainNetwork/godx/common"
)

// markThawingAddressAndValue add the thawing diff to the addr's thawing assets in corresponding epoch,
// and mark the address to be thawed in the thawing address
func markThawingAddressAndValue(state stateDB, addr common.Address, curEpoch int64, diff common.BigInt) {
	thawingEpoch := calcThawingEpoch(curEpoch)
	// Add the diff value to thawing assets to be thawed
	addThawingAssets(state, addr, thawingEpoch, diff)
	// Mark the address in the thawing address
	markThawingAddress(state, addr, thawingEpoch)
}

// markThawingAddress mark the given addr that will be thawed in thawingEpoch
func markThawingAddress(stateDB stateDB, addr common.Address, thawingEpoch int64) {
	// create thawing address: "thawing_" + currentEpoch
	thawingAddress := getThawingAddress(thawingEpoch)
	if !stateDB.Exist(thawingAddress) {
		stateDB.CreateAccount(thawingAddress)
		// before thawing deposit, mark thawingAddress as not empty account to avoid being deleted by stateDB
		stateDB.SetNonce(thawingAddress, 1)
	}
	// set thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	setAddrInThawingAddress(stateDB, thawingAddress, addr)
}

// thawAllFrozenAssetsInEpoch thaw the frozenAssets for all candidates or delegator effected in
// the epoch
func thawAllFrozenAssetsInEpoch(state stateDB, epoch int64) error {
	thawingAddress := getThawingAddress(epoch)
	var err error
	// For each thawing address, thaw specified amount, and remove the related storage fields.
	forEachEntryInThawingAddress(state, thawingAddress, func(addr common.Address) {
		thawValue := getThawingAssets(state, addr, epoch)
		err = subFrozenAssets(state, addr, thawValue)
		if err != nil {
			// If error happens, this is a really critical error, meaning the system is
			// not consistent with itself. Thus no need to report every error.
			return
		}
		// Remove fields in both thawing address and thawing assets in account address
		removeThawingAssets(state, addr, epoch)
		removeAddrInThawingAddress(state, thawingAddress, addr)
	})
	removeAddressInState(state, thawingAddress)
	return err
}

// forEachEntryInThawingAddress execute the cb callback function on each entry in the thawing address
func forEachEntryInThawingAddress(state stateDB, thawingAddress common.Address, cb func(address common.Address)) {
	if !state.Exist(thawingAddress) {
		return
	}
	state.ForEachStorage(thawingAddress, func(key, value common.Hash) bool {
		// If empty value, directly return
		if value == (common.Hash{}) {
			return false
		}
		// Execute the callback for the address from value
		addr := getAddressFromThawingValue(value)
		cb(addr)
		return true
	})
}

// getThawingAddress return the thawing address with the epoch
func getThawingAddress(epoch int64) common.Address {
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(epoch))
	b := append([]byte(PrefixThawingAddr), epochBytes...)
	thawingAddress := common.BytesToAddress(b)
	return thawingAddress
}

// setAddrInThawingAddress set the address in the thawing address
func setAddrInThawingAddress(state stateDB, thawingAddress, addr common.Address) {
	// set thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	keyAndValue := makeThawingAddressKey(addr)
	state.SetState(thawingAddress, keyAndValue, keyAndValue)
}

// removeAddrInThawingAddress removes an entry specified by addr in thawingAddress
func removeAddrInThawingAddress(state stateDB, thawingAddress, addr common.Address) {
	key := makeThawingAddressKey(addr)
	state.SetState(thawingAddress, key, common.Hash{})
}

// makeThawingAddressKey makes the key / value for the thawing address
func makeThawingAddressKey(addr common.Address) common.Hash {
	// thawing flag for from address: "candidate_" + from ==> "candidate_" + from
	return common.BytesToHash(addr.Bytes())
}

// getAddressFromThawingValue returns the address from the key / value in the thawing address
// storage trie
func getAddressFromThawingValue(h common.Hash) common.Address {
	return common.BytesToAddress(h[common.HashLength-common.AddressLength:])
}

// calcThawingEpoch calculate the epoch to be thawed for the thawing record in the current epoch
func calcThawingEpoch(curEpoch int64) int64 {
	return curEpoch + ThawingEpochDuration
}
