package erasurecode

import (
	"io"
)

const (
	ECTypeInvalid uint8 = iota
	ECTypeStandard
	ECTypeShard
)

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
