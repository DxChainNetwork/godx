package erasurecode

import (
	"errors"
	"io"
)

const (
	ECTypeInvalid uint8 = iota
	ECTypeStandard
	ECTypeShard
)

var ErrInvalidECType = errors.New("invalid erasure code type")

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

	// SupportPartialEncoding indicates whether partial encoding is supported. For standard type, always false
	SupportsPartialEncoding() bool
}

// NewErasureCode returns a new erasure coder. Type supported are ECTypeStandard, and ECTypeShard.
// The two parameters followed is parameters used for erasure code: num of data sectors and total
// number of sectors
func NewErasureCoder(ecType uint8, minSectors uint32, numSectors uint32) (ErasureCoder, error) {
	switch ecType {
	case (&standardErasureCode{}).Type():
		return newStandardErasureCode(minSectors, numSectors)
	case (&shardErasureCode{}).Type():
		return newShardErasureCode(minSectors, numSectors)
	default:
		return nil, ErrInvalidECType
	}
}
