// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

// TestRandomSelectAddress test randomSelectAddress
func TestRandomSelectAddress(t *testing.T) {
	tests := []struct {
		entrySize    int
		targetSize   int
		expectedSize int
	}{
		{4, 4, 4},
		{100, 100, 100},
		{100, 4, 4},
		{4, 100, 4},
		{10000, 21, 21},
	}
	for i, test := range tests {
		data := makeRandomSelectorData(test.entrySize)
		seed := int64(0)
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
				t.Fatal("duplicate selected address")
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
	seed := time.Now().UnixNano()
	var selectedAddr common.Address
	for selectedAddr = range data {
		data[selectedAddr] = common.NewBigIntUint64(1e18)
		break
	}
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
		entries     map[common.Address]common.BigInt
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

func makeRandomSelectorData(num int) map[common.Address]common.BigInt {
	m := make(map[common.Address]common.BigInt)
	for i := 0; i != num; i++ {
		addr := common.BytesToAddress([]byte{byte(i + 1)})
		m[addr] = common.BigInt1
	}
	return m
}
