package bitvector

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common/math"
	"testing"
)

func TestBitVector(t *testing.T){

	vec := BitVector(1)
	for i := 0; i < 64; i++{
		vec = vec << 1
		fmt.Println(vec)
	}

	fmt.Println(uint64(math.MaxUint64))

}
