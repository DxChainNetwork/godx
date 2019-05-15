package storagemanager

import (
	"github.com/DxChainNetwork/godx/common/math"
	"math/rand"
	"strconv"
	"testing"
)

func TestIsFree(t *testing.T) {
	for i := 0; i < 10; i++ {
		var vec = BitVector(rand.Uint64() >> 1)
		bit := getReversedBinary(int64(vec), t)

		for i := 0; i < 64; i++ {
			free := vec.isFree(uint16(i))

			if bit[i] == 1 {
				if free {
					t.Error("the bit should be used")
				}
			}

			if bit[i] == 0 {
				if !free {
					t.Error("the bit should be free")
				}
			}
		}
	}
}

func getReversedBinary(num int64, t *testing.T) []int {
	var err error
	var b = make([]int, 64)
	str := strconv.FormatInt(num, 2)

	for i := 0; i < len(str); i++ {
		b[len(b)-len(str)+i], err = strconv.Atoi(string(str[i]))
		if err != nil {
			t.Error(err.Error())
		}
	}

	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return b
}

func TestSetUsage(t *testing.T) {
	for i := 0; i < 10; i++ {
		var vec = BitVector(rand.Uint64() >> 1)
		for i := 0; i < 64; i++ {
			vec.setUsage(uint16(i))
		}
		if vec != math.MaxUint64 {
			t.Error("some usage may not be set")
		}
	}
}

func TestClearUsage(t *testing.T) {

	for i := 0; i < 10; i++ {
		var vec = BitVector(rand.Uint64() >> 1)
		for i := 0; i < 64; i++ {
			vec.clearUsage(uint16(i))
		}
		if vec != 0 {
			t.Error("some usage may not be set")
		}
	}
}
