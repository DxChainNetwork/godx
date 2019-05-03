// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package erasurecode

import (
	"errors"
	"fmt"
	"io"
)

const (
	// ECTypeInvalid is the type not supported
	ECTypeInvalid uint8 = iota

	// ECTypeStandard is the type code for standardErasureCode
	ECTypeStandard

	// ECTypeShard is the type code for shardErasureCode
	ECTypeShard
)

// ErrInvalidECType is the error that the input type code is not supported
var ErrInvalidECType = errors.New("invalid erasure code type")

// ErasureCoder is the interface supported for this package.
// Implemented types are
//	 ECTypeStandard - standardErasureCode
// 	 ECTypeShard - shardErasureCode
// Recommend to use the standard erasure code instead of the sharding one because of performance
type ErasureCoder interface {
	// Type return the type of the code
	Type() uint8

	// NumSectors return the total number of encoded sectors
	NumSectors() uint32

	// MinSectors return the number of minimum sectors that is required to recover the original data
	MinSectors() uint32

	// Encode encode the segment to sectors
	Encode(data []byte) ([][]byte, error)

	// Recover decode the input sectors to the original data with length outLen
	Recover(sectors [][]byte, n int, w io.Writer) error
}

// New returns a new ErasureCoder. Type supported are ECTypeStandard, and ECTypeShard.
// The two parameters followed is parameters used for erasure code: num of data sectors and total
// number of sectors. Additional arguments could be attached for param specification.
func New(ecType uint8, minSectors uint32, numSectors uint32, extra ...interface{}) (ErasureCoder, error) {
	switch ecType {
	case (&standardErasureCode{}).Type():
		return newStandardErasureCode(minSectors, numSectors)
	case (&shardErasureCode{}).Type():
		if extra != nil && len(extra) != 0 {
			shardSize, isInt := extra[0].(int)
			if !isInt {
				return nil, fmt.Errorf("using shardErasureCode, the first argument should be of int type")
			}
			return newShardErasureCode(minSectors, numSectors, shardSize)
		}
		return newShardErasureCode(minSectors, numSectors, EncodedShardUnit)
	default:
		return nil, ErrInvalidECType
	}
}
