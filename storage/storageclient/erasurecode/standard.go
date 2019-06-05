// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package erasurecode

import (
	"fmt"
	"io"

	"github.com/klauspost/reedsolomon"
)

// standardECode is an implementation of ErasureCoder, which could encode segment as a whole
type standardErasureCode struct {
	enc reedsolomon.Encoder

	// EC code related fields
	numSectors uint32 // number of total sectors
	minSectors uint32 // minimum sectors required
}

// newStandardErasureCode create a standardErasureCode based on params provided
func newStandardErasureCode(minSectors, numSectors uint32) (*standardErasureCode, error) {
	if minSectors > numSectors {
		return nil, fmt.Errorf("wrong initialization params: minSectors > numSectors")
	}
	dataShards, parityShards := minSectors, numSectors-minSectors
	enc, err := reedsolomon.New(int(dataShards), int(parityShards))
	if err != nil {
		return nil, err
	}
	return &standardErasureCode{
		enc:        enc,
		numSectors: numSectors,
		minSectors: minSectors,
	}, nil
}

// Type return the type of the code
func (sec *standardErasureCode) Type() uint8 {
	return ECTypeStandard
}

// NumSectors return the total number of encoded sectors
func (sec *standardErasureCode) NumSectors() uint32 {
	return sec.numSectors
}

// MinSectors return the number of minimum sectors that is required to recover the original data
func (sec *standardErasureCode) MinSectors() uint32 {
	return sec.minSectors
}

// Extra of standardErasureCode return nothing
func (sec *standardErasureCode) Extra() []interface{} {
	return nil
}

// Encode encode the segment to sectors
func (sec *standardErasureCode) Encode(data []byte) ([][]byte, error) {
	sectors, err := sec.enc.Split(data)
	if err != nil {
		return nil, err
	}
	return sec.encodeSectors(sectors)
}

// EncodeSectors encode the sectors and fill in values
func (sec *standardErasureCode) encodeSectors(sectors [][]byte) ([][]byte, error) {
	if len(sectors) < int(sec.minSectors) {
		return nil, fmt.Errorf("not enough sectors provided: %d < %d", len(sectors), sec.minSectors)
	}
	sectorSize := len(sectors[0])
	for len(sectors) < int(sec.numSectors) {
		sectors = append(sectors, make([]byte, sectorSize))
	}
	err := sec.enc.Encode(sectors)
	if err != nil {
		return nil, err
	}
	return sectors, nil
}

// Recover decode the input sectors to the original data with length outLen
func (sec *standardErasureCode) Recover(sectors [][]byte, outLen int, w io.Writer) error {
	err := sec.enc.ReconstructData(sectors)
	if err != nil {
		return err
	}
	return sec.enc.Join(w, sectors, outLen)
}
