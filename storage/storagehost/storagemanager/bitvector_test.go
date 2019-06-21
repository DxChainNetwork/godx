// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

// TestIsFree test if the bits is actually free
// in case
func TestIsFree(t *testing.T) {
	// generate 10 random number and convert to binary
	for i := 0; i < 10; i++ {
		// step to avoid overflow
		var vec = bitVector(rand.Uint64() >> 1)
		bit := getReversedBinary(int64(vec), t)
		// because there is 64 bit in binary format
		for i := 0; i < 64; i++ {
			free := vec.isFree(uint64(i))
			// the bit is actually filled
			if bit[i] == 1 && free {
				t.Error("the bit should be used")
			}
			// the bit is actually free
			if bit[i] == 0 && !free {
				t.Error("the bit should be free")
			}
		}
	}
}

// TestSetUsage check if the function could clear
// the Usage and update the input vector
func TestSetUsage(t *testing.T) {
	// take 10 turns and generate 10 random number
	for i := 0; i < 10; i++ {
		// step to avoid overflow
		var vec = bitVector(rand.Uint64() >> 1)
		for i := 0; i < 64; i++ {
			// set every Usage
			vec.setUsage(uint64(i))
		}
		// after all Usage been set, the result should be the maximum uin64
		if vec != math.MaxUint64 {
			t.Error("some Usage may not be set")
		}
	}
}

// TestClearUsage check if the function could clear
// a Usage and update the number
func TestClearUsage(t *testing.T) {
	// take 10 turns and generate random number
	for i := 0; i < 10; i++ {
		var vec = bitVector(rand.Uint64() >> 1)
		for i := 0; i < 64; i++ {
			// clear the Usage on each bit
			vec.clearUsage(uint64(i))
		}

		// after all Usage been cleared, vec should be 0
		if vec != 0 {
			t.Error("some Usage may not be set")
		}
	}
}

// helper function, convert decimal number to binary format,
// and store the binary format in order, for example:
// 66 in binary: 1000010
// return an array: [0 1 0 0 0 0 1 0 0 0 0 0 0 0 0
// 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
// 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]
func getReversedBinary(num int64, t *testing.T) []int {
	var err error
	var b = make([]int, 64)
	// convert binary in string format
	str := strconv.FormatInt(num, 2)
	// loop through each bit and store in according index
	for i := 0; i < len(str); i++ {
		b[len(b)-len(str)+i], err = strconv.Atoi(string(str[i]))
		if err != nil {
			t.Error(err.Error())
		}
	}
	// switch the order
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

// TestBitVector test the operations for bitVector
func TestBitVector(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	testTimes := 10000
	if testing.Short() {
		testTimes = 100
	}
	vec := bitVector(0)
	usage := make([]bool, bitVectorGranularity)
	for i := 0; i != testTimes; i++ {
		n := rand.Intn(64)
		addOrDelete := rand.Int()%2 == 0
		if addOrDelete {
			vec.setUsage(uint64(n))
			usage[n] = true
		} else {
			vec.clearUsage(uint64(n))
			usage[n] = false
		}
		if i%10 == 0 {
			// Check consistency every 10 updates
			for idx := 0; idx != bitVectorGranularity; idx++ {
				if vec.isFree(uint64(idx)) == usage[idx] {
					t.Errorf("isFree not expected at index [%d] at %d update", idx, i)
				}
			}
		}
	}
}
