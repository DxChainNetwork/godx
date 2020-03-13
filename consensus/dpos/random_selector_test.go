// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

// TestSingleSelectWithWeight test whether the lucky spinner will random select according
// to weight
func TestSingleSelectWithWeight(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	countMap := make(map[common.Address]int)
	raw := makeTestEntries(100)
	for i := 0; i != 1000000; i++ {
		res, err := randomSelectAddress(typeLuckySpinner, raw, time.Now().UnixNano(), 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, addr := range res {
			if _, ok := countMap[addr]; !ok {
				countMap[addr] = 1
			} else {
				countMap[addr] += 1
			}
		}
	}
	if err := checkExpectCount(countMap, raw, t); err != nil {
		t.Fatal(err)
	}
}

// checkExpectCount checks whether the selected count is proportional to the vote power
func checkExpectCount(cntMap map[common.Address]int, entries randomSelectorEntries, t *testing.T) error {
	if len(cntMap) == 0 || len(entries) == 0 {
		return fmt.Errorf("empty input")
	}
	allowedBias := float64(300)
	unitAddress, unitVote := entries[len(entries)-1].addr, entries[len(entries)-1].vote
	sumBias := float64(0)
	for _, entry := range entries {
		got := float64(cntMap[entry.addr]) / float64(cntMap[unitAddress])
		expect := entry.vote.Float64() / unitVote.Float64()
		sumBias += (got - expect) / expect * 100
	}
	t.Logf("bias: %.2f%%", sumBias/100)
	if math.Abs(sumBias) > allowedBias {
		return fmt.Errorf("result distribution is strongly biased")
	}
	return nil
}

func makeTestEntries(numEntries int) randomSelectorEntries {
	entries := make(randomSelectorEntries, 0, numEntries)
	for i := 1; i != numEntries; i++ {
		addr := common.BigToAddress(common.NewBigIntUint64(uint64(i)).BigIntPtr())
		vote := common.NewBigInt(int64(i)).Mult(common.NewBigIntUint64(1000000000000000000))

		entries = append(entries, &randomSelectorEntry{addr, vote})
	}
	return entries
}

// TestRandomSelectAddress test randomSelectAddress
func TestRandomSelectAddress(t *testing.T) {
	for i := 0; i != 100; i++ {
		testRandomSelectAddress(t)
	}
}

func testRandomSelectAddress(t *testing.T) {
	tests := []struct {
		typeSelector int
		entrySize    int
		targetSize   int
		expectedSize int
	}{
		{typeLuckyWheel, 4, 4, 4},
		{typeLuckyWheel, 5, 4, 4},
		{typeLuckyWheel, 100, 100, 100},
		{typeLuckyWheel, 100, 4, 4},
		{typeLuckyWheel, 4, 100, 4},
		{typeLuckyWheel, 1000, 21, 21},
		{typeLuckySpinner, 4, 4, 4},
		{typeLuckySpinner, 5, 4, 4},
		{typeLuckySpinner, 100, 100, 100},
		{typeLuckySpinner, 100, 4, 4},
		{typeLuckySpinner, 4, 100, 4},
		{typeLuckySpinner, 1000, 21, 21},
	}
	for i, test := range tests {
		data := makeRandomSelectorData(test.entrySize)
		seed := time.Now().UnixNano()
		selected, err := randomSelectAddress(test.typeSelector, data, seed, test.targetSize)
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

// TestLuckyWheelSelectWithWeight test whether the lucky wheel will
// select address based on weight.
func TestLuckyWheelSelectWithWeight(t *testing.T) {
	for i := 0; i < 10000; i++ {
		testLuckyWheelSelectWithWeight(t)
	}
}

func testLuckyWheelSelectWithWeight(t *testing.T) {
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

// TestRandomSelectAddressType test the error case for randomSelectAddress
func TestRandomSelectAddressType(t *testing.T) {
	tests := []struct {
		typeCode    int
		entries     randomSelectorEntries
		target      int
		expectedErr error
	}{
		{
			typeLuckyWheel, makeRandomSelectorData(10), 4,
			nil,
		},
		{
			typeLuckySpinner, makeRandomSelectorData(10), 4,
			nil,
		},
		{
			typeInvalidRandomSelector, makeRandomSelectorData(10), 5,
			errUnknownRandomAddressSelectorType,
		},
	}
	for _, test := range tests {
		_, err := randomSelectAddress(test.typeCode, test.entries, int64(0), test.target)
		if err != test.expectedErr {
			t.Errorf("Expect error [%v]\n Got error [%v]", test.expectedErr, err)
		}
	}
}

// TestLuckyWheelSelectConsistent test the consistency of luckyWheel.
// Given the same input, the function should always give the same output.
func TestLuckyWheelSelectConsistent(t *testing.T) {
	tests := []struct {
		dataSize   int
		targetSize int
	}{
		{1000, 21},
		{15, 21},
	}
	for _, test := range tests {
		var res []common.Address
		numIter := 10
		data := makeRandomSelectorData(test.dataSize)
		seed := time.Now().UnixNano()
		for i := 0; i != numIter; i++ {
			validators, err := randomSelectAddress(typeLuckyWheel, data, seed, test.targetSize)
			if err != nil {
				t.Fatal(err)
			}
			if len(res) == 0 {
				res = validators
				continue
			}
			// the selected validators should be exactly the same with order
			if err := checkSameValidatorSet(res, validators); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Given the different seed, the lucky wheel selection should return different
// results.
func TestRandomSelectLuckyWheelDifferent(t *testing.T) {
	tests := []struct {
		dataSize   int
		targetSize int
	}{
		{100, 21},
		{15, 21},
	}
	for i, test := range tests {
		for retry := 0; retry != 3; retry++ {
			seed1 := time.Now().UnixNano()
			time.Sleep(1 * time.Nanosecond)
			seed2 := time.Now().UnixNano()
			data := makeRandomSelectorData(test.dataSize)
			validators1, err := randomSelectAddress(typeLuckyWheel, data, seed1, test.targetSize)
			if err != nil {
				t.Fatal(err)
			}
			validators2, err := randomSelectAddress(typeLuckyWheel, data, seed2, test.targetSize)
			if err != nil {
				t.Fatal(err)
			}
			// The possibility of validators1 == validators2 being exactly the same is really small.
			// So if the two validator set is exactly the same, report the error.
			if err := checkSameValidatorSet(validators1, validators2); err != nil {
				break
			} else if err == nil && retry == 3 {
				t.Fatalf("Test %d: different seed should not yield same result", i)
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

func checkSameValidatorSet(vs1, vs2 []common.Address) error {
	if len(vs1) != len(vs2) {
		return fmt.Errorf("size not same. %v != %v", len(vs1), len(vs2))
	}
	for i := range vs1 {
		if vs1[i] != vs2[i] {
			return fmt.Errorf("validators[%d]: %x != %x", i, vs1[i], vs2[i])
		}
	}
	return nil
}
