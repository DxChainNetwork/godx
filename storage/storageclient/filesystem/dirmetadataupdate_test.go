package filesystem

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common/math"
	"testing"
)

func TestUint32Overflow(t *testing.T) {
	num1 := math.MaxUint32
	num2 := math.MaxUint32
	num3 := num1 + num2
	fmt.Println(num3)
}
