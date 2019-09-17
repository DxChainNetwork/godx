// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"math"

	"github.com/DxChainNetwork/godx/common"
)

// hashToUint64 convert the hash to uint64. Only the last 8 bytes in the hash is interpreted as
// uint64
func hashToUint64(hash common.Hash) uint64 {
	return binary.BigEndian.Uint64(hash[common.HashLength-8:])
}

// uint64ToHash convert the uint64 value to the hash. The value is written in the last 8 bytes
// in the hash
func uint64ToHash(value uint64) common.Hash {
	var h common.Hash
	binary.BigEndian.PutUint64(h[common.HashLength-8:], value)
	return h
}

func hashToFloat64(hash common.Hash) float64 {
	bits := binary.BigEndian.Uint64(hash[common.HashLength-8:])
	return math.Float64frombits(bits)
}

// float64ToHash convert the float64 to hash. Only the last 8 bytes are used
func float64ToHash(value float64) common.Hash {
	var hash common.Hash
	bits := math.Float64bits(value)
	binary.BigEndian.PutUint64(hash[common.HashLength-8:], bits)
	return hash
}
