package bloombits

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/DxChainNetwork/godx/core/types"
)

// Tests that batched bloom bits are correctly rotated from the input bloom
// filters.
func TestGenerator(t *testing.T) {
	// Generate the input and the rotated output
	var input, output [types.BloomBitLength][types.BloomByteLength]byte

	for i := 0; i < types.BloomBitLength; i++ {
		for j := 0; j < types.BloomBitLength; j++ {
			bit := byte(rand.Int() % 2)

			input[i][j/8] |= bit << byte(7-j%8)
			output[types.BloomBitLength-1-j][i/8] |= bit << byte(7-i%8)
		}
	}
	// Crunch the input through the generator and verify the result
	gen, err := NewGenerator(types.BloomBitLength)
	if err != nil {
		t.Fatalf("failed to create bloombit generator: %v", err)
	}
	for i, bloom := range input {
		if err := gen.AddBloom(uint(i), bloom); err != nil {
			t.Fatalf("bloom %d: failed to add: %v", i, err)
		}
	}
	for i, want := range output {
		have, err := gen.Bitset(uint(i))
		if err != nil {
			t.Fatalf("output %d: failed to retrieve bits: %v", i, err)
		}
		if !bytes.Equal(have, want[:]) {
			t.Errorf("output %d: bit vector mismatch have %x, want %x", i, have, want)
		}
	}
}
