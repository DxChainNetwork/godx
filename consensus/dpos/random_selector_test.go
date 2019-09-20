// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"strconv"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
)

// TestRandomSelectAddress test randomSelectAddress
func TestRandomSelectAddress(t *testing.T) {
	for i := 0; i != 100; i++ {
		testRandomSelectAddress(t)
	}
}

func testRandomSelectAddress(t *testing.T) {
	tests := []struct {
		entrySize    int
		targetSize   int
		expectedSize int
	}{
		{4, 4, 4},
		{5, 4, 4},
		{100, 100, 100},
		{100, 4, 4},
		{4, 100, 4},
		{1000, 21, 21},
	}
	for i, test := range tests {
		data := makeRandomSelectorData(test.entrySize)
		seed := time.Now().UnixNano()
		selected, err := randomSelectAddress(typeLuckyWheel, data, seed, test.targetSize)
		if err != nil {
			t.Fatal(err)
		}
		if len(selected) != test.expectedSize {
			t.Errorf("Test %d: size expect [%v], got [%v]", i, test.expectedSize, len(selected))
		}
		// Loop over selected to see whether there is any address being selected multiple times
		m := make(map[common.Address]struct{})
		for _, v := range selected {
			if _, exist := m[v]; exist {
				t.Fatalf("Test %d: duplicate selected address %x", i, v)
			}
			m[v] = struct{}{}
		}
	}
}

// TestRandomSelectAddress_Weight test whether the function randomSelectAddress will
// select address based on weight.
func TestRandomSelectAddressWeight(t *testing.T) {
	for i := 0; i < 10000; i++ {
		testRandomSelectAddressWeight(t)
	}
}

func testRandomSelectAddressWeight(t *testing.T) {
	// Create entry data. Only one of the addresses are given a high weight
	data := makeRandomSelectorData(5)
	selectedIndex := 1
	seed := time.Now().UnixNano()
	data[selectedIndex].vote = common.NewBigIntUint64(1e18)
	selectedAddr := data[selectedIndex].addr
	// Select one from the entries. Expect the selectedAddr should be selected
	selected, err := randomSelectAddress(typeLuckyWheel, data, seed, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("not 1 address selected: %v", len(selected))
	}
	if selected[0] != selectedAddr {
		t.Error("not the expected entry being selected")
	}
}

// TestRandomSelectAddressError test the error case for randomSelectAddress
func TestRandomSelectAddressError(t *testing.T) {
	tests := []struct {
		typeCode    int
		entries     randomSelectorEntries
		target      int
		expectedErr error
	}{
		{
			1, makeRandomSelectorData(10), 5,
			errUnknownRandomAddressSelectorType,
		},
		{
			0, makeRandomSelectorData(10), 4,
			nil,
		},
	}
	for _, test := range tests {
		_, err := randomSelectAddress(test.typeCode, test.entries, int64(0), test.target)
		if err != test.expectedErr {
			t.Errorf("Expect error [%v]\n Got error [%v]", test.expectedErr, err)
		}
	}
}

// TestRandomSelectAddressConsistent test the consistency of randomSelectAddressConsistent.
// Given the same input, the function should always give the same output.
func TestRandomSelectAddressConsistent(t *testing.T) {
	var res []common.Address
	num := 10
	data := makeRandomSelectorData(100)
	seed := time.Now().UnixNano()
	for i := 0; i != num; i++ {
		validators, err := randomSelectAddress(typeLuckyWheel, data, seed, 21)
		if err != nil {
			t.Fatal(err)
		}
		if len(res) == 0 {
			res = validators
			continue
		}
		// the selected validators should be exactly the same with order
		if len(res) != len(validators) {
			t.Fatalf("Round %d, validator size not equal. Got %v, Expect %v", i, len(validators), len(res))
		}
		for j := range res {
			if res[j] != validators[j] {
				t.Errorf("Round %d, validator[%d] not equal. Got %v, expect %v", i, j,
					validators[j], res[j])
			}
		}
	}
}

func makeRandomSelectorData(num int) randomSelectorEntries {
	var entries randomSelectorEntries
	for i := 0; i != num; i++ {
		addr := common.BigToAddress(common.NewBigIntUint64(uint64(i)).BigIntPtr())
		entries = append(entries, &randomSelectorEntry{addr, common.BigInt1})
	}
	return entries
}

func TestLuckyWheel(t *testing.T) {
	// test 1: candidates less than maxValidatorSize
	// mock some vote proportion
	votes := make(randomSelectorEntries, 0)
	for i := 0; i < MaxValidatorSize-1; i++ {
		str := strconv.FormatUint(uint64(i+1), 10)
		voteProportion := randomSelectorEntry{
			addr: common.HexToAddress("0x" + str),
			vote: common.NewBigIntUint64(uint64(i + 1)),
		}
		votes = append(votes, &voteProportion)
	}

	// make random seed
	blockHash := common.HexToAddress("0xb7c653791455fdb56fca714c0090c8dffa83a50c546b1dc4ab4dd73b91639b38")
	epochID := int64(1001)
	seed := int64(binary.LittleEndian.Uint32(crypto.Keccak512(blockHash.Bytes()))) + epochID
	rs, err := newRandomAddressSelector(typeLuckyWheel, votes, seed, MaxValidatorSize)
	if err != errRandomSelectNotEnoughEntries {
		t.Fatalf("expect %v, got %v", errRandomSelectNotEnoughEntries, nil)
	}
	result := votes.listAddresses()

	// check result
	if len(result) != MaxValidatorSize-1 {
		t.Errorf("LuckyTurntable candidates with the number of maxValidatorSize - 1,want result length: %d,got: %d", MaxValidatorSize-1, len(result))
	}

	for i, addr := range result {
		str := strconv.FormatUint(uint64(i+1), 10)
		if addr != common.HexToAddress("0x"+str) {
			t.Errorf("LuckyTurntable candidates with the number of maxValidatorSize - 1,want elected addr: %s,got: %s", common.HexToAddress("0x"+str).String(), addr.String())
		}
	}

	// test 2: candidates more than maxValidatorSize
	// add another maxValidatorSize-1 candidates
	for i := 0; i < MaxValidatorSize-1; i++ {
		str := strconv.FormatUint(uint64(i+MaxValidatorSize-1), 10)
		voteProportion := randomSelectorEntry{
			addr: common.HexToAddress("0x" + str),
			vote: common.NewBigIntUint64(uint64(i + 1)),
		}
		votes = append(votes, &voteProportion)
	}

	rs, err = newRandomAddressSelector(typeLuckyWheel, votes, seed, MaxValidatorSize)
	if err != nil {
		t.Fatal(err)
	}
	result = rs.RandomSelect()
	if len(result) != MaxValidatorSize {
		t.Errorf("candidates with the number of maxValidatorSize - 1,want result length: %d,got: %d", MaxValidatorSize, len(result))
	}
}
