// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"math/big"
	"math/rand"
	"sync"

	"github.com/DxChainNetwork/godx/common"
)

// randomAddressSelector is the random selection algorithm for selecting multiple addresses
// The entries are passed in during initialization
type randomAddressSelector interface {
	RandomSelect() []common.Address
}

const (
	typeLuckyWheel = iota
)

type (
	// luckyWheel is the structure for lucky wheel random selection
	luckyWheel struct {
		// Initialized fields
		rand    *rand.Rand
		entries randomSelectorEntries
		target  int

		// results
		results []common.Address

		// In process
		sumVotes common.BigInt
		once     sync.Once
	}

	// randomSelectorEntries is the list of randomSelectorEntry,
	// and is a sortable address list of address with a count by descending order
	randomSelectorEntries []*randomSelectorEntry

	// randomSelectorEntry is the entry in the lucky wheel
	randomSelectorEntry struct {
		addr common.Address
		vote common.BigInt
	}
)

// randomSelectAddress randomly select entries based on weight from the entries.
func randomSelectAddress(typeCode int, data randomSelectorEntries, seed int64, target int) ([]common.Address, error) {
	ras, err := newRandomAddressSelector(typeCode, data, seed, target)
	if err != nil {
		return []common.Address{}, err
	}
	return ras.RandomSelect(), nil
}

// newRandomAddressSelector creates a randomAddressSelector with sepecified typeCode
func newRandomAddressSelector(typeCode int, entries randomSelectorEntries, seed int64, target int) (randomAddressSelector, error) {
	switch typeCode {
	case typeLuckyWheel:
		return newLuckyWheel(entries, seed, target)
	}
	return nil, errUnknownRandomAddressSelectorType
}

// newLuckyWheel create a lucky wheel for random selection. target is used for specifying
// the target number to be selected
func newLuckyWheel(entries randomSelectorEntries, seed int64, target int) (*luckyWheel, error) {
	sumVotes := common.BigInt0
	for _, entry := range entries {
		sumVotes = sumVotes.Add(entry.vote)
	}
	lw := &luckyWheel{
		rand:     rand.New(rand.NewSource(seed)),
		target:   target,
		entries:  make(randomSelectorEntries, len(entries)),
		results:  make([]common.Address, target),
		sumVotes: sumVotes,
	}
	// Make a copy of the input entries. Thus the modification of lw.entries will not effect
	// the input entries
	copy(lw.entries, entries)
	return lw, nil
}

// RandomSelect return the result of the random selection of lucky wheel
func (lw *luckyWheel) RandomSelect() []common.Address {
	lw.once.Do(lw.randomSelect)
	return lw.results
}

// RandomSelect is a helper function that randomly select addresses from the lucky wheel.
// The execution result is added to lw.results field
func (lw *luckyWheel) randomSelect() {
	// If the number of entries is less than target, return shuffled entries
	if len(lw.entries) < lw.target {
		lw.shuffleAndWriteEntriesToResult()
		return
	}
	// Else execute the random selection algorithm
	for i := 0; i < lw.target; i++ {
		// Execute the selection
		selectedIndex := lw.selectSingleEntry()
		selectedEntry := lw.entries[selectedIndex]
		// Add to result, and remove from entry
		lw.results[i] = selectedEntry.addr
		if selectedIndex == len(lw.entries)-1 {
			lw.entries = lw.entries[:len(lw.entries)-1]
		} else {
			lw.entries = append(lw.entries[:selectedIndex], lw.entries[selectedIndex+1:]...)
		}
		// Subtract the vote weight from sumVotes
		lw.sumVotes.Sub(selectedEntry.vote)
	}
}

// selectSingleEntry select a single entry from the lucky Wheel. Return the selected index.
// No values updated in this function.
func (lw *luckyWheel) selectSingleEntry() int {
	selected := randomBigInt(lw.rand, lw.sumVotes)
	for i, entry := range lw.entries {
		vote := entry.vote
		// The entry is selected
		if selected.Cmp(vote) <= 0 {
			return i
		}
		selected = selected.Sub(vote)
	}
	// Sanity: This shall never reached if code is correct. If this happens, currently
	// return the last entry of the entries
	// TODO: Should we panic here?
	return len(lw.entries) - 1
}

// shuffleAndWriteEntriesToResult shuffle and write the entries to results.
// The function is only used when the target is smaller than entry length.
func (lw *luckyWheel) shuffleAndWriteEntriesToResult() {
	list := lw.entries.listAddresses()
	lw.rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
	lw.results = list
	return
}

// listAddresses return the list of addresses of the entries
func (entries randomSelectorEntries) listAddresses() []common.Address {
	res := make([]common.Address, 0, len(entries))
	for _, entry := range entries {
		res = append(res, entry.addr)
	}
	return res
}

// randomBigInt return a random big integer between 0 and max using r as randomization
func randomBigInt(r *rand.Rand, max common.BigInt) common.BigInt {
	randNum := new(big.Int).Rand(r, max.BigIntPtr())
	return common.PtrBigInt(randNum)
}

func (entries randomSelectorEntries) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries randomSelectorEntries) Len() int      { return len(entries) }
func (entries randomSelectorEntries) Less(i, j int) bool {
	if entries[i].vote.Cmp(entries[j].vote) != 0 {
		return entries[i].vote.Cmp(entries[j].vote) == 1
	}
	return entries[i].addr.String() > entries[j].addr.String()
}

// RemoveEntry remove the entry with the given index
func (entries randomSelectorEntries) RemoveEntry(index int) {
	length := entries.Len()
	pre := entries[:index]
	suff := make(randomSelectorEntries, 0)
	if index != length {
		suff = entries[index+1:]
	}
	entries = append(pre, suff...)
}
