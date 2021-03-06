// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

// TestNewErasureCode_ErasureCodeToParams test conversion from metadata params to ec, and ec to params
func TestNewErasureCode_ErasureCodeToParams(t *testing.T) {
	tests := []struct {
		erasureCodeType uint8
		minSectors      uint32
		numSectors      uint32
		extra           []byte
		extraExp        []byte
		err             error
	}{
		{
			erasureCodeType: erasurecode.ECTypeStandard,
			minSectors:      10,
			numSectors:      30,
			extra:           []byte{},
			extraExp:        []byte{},
		},
		{
			erasureCodeType: erasurecode.ECTypeShard,
			minSectors:      10,
			numSectors:      30,
			extra:           makeUint32Byte(128),
			extraExp:        makeUint32Byte(128),
		},
		{
			erasureCodeType: erasurecode.ECTypeShard,
			minSectors:      10,
			numSectors:      30,
			extra:           []byte{},
			extraExp:        makeUint32Byte(64),
		},
		{
			erasureCodeType: erasurecode.ECTypeInvalid,
			err:             erasurecode.ErrInvalidECType,
		},
	}
	for i, test := range tests {
		md := &Metadata{
			ErasureCodeType: test.erasureCodeType,
			MinSectors:      test.minSectors,
			NumSectors:      test.numSectors,
			ECExtra:         test.extra,
		}
		ec, err := md.newErasureCode()
		if (err == nil) != (test.err == nil) {
			t.Fatalf("Test %d: expect error %v, got error %v", i, test.err, err)
		}
		if err != nil {
			continue
		}
		recoveredMin, recoveredNum, recoveredExtra, _ := erasureCodeToParams(ec)
		if recoveredMin != test.minSectors {
			t.Errorf("Test %d: Recovered minSectors not expected. Want %v, Got %v", i, test.minSectors, recoveredMin)
		}
		if recoveredNum != test.numSectors {
			t.Errorf("Test %d: Recovered numSectors not expected. Want %v, Got %v", i, test.numSectors, recoveredNum)
		}
		if !bytes.Equal(recoveredExtra, test.extraExp) {
			t.Errorf("Test %d: Recovered extra not expected. Want %x, Got %x", i, test.extraExp, recoveredExtra)
		}
	}
}

// makeUint32Byte return a 32 byte as the value of num
func makeUint32Byte(num uint32) []byte {
	uint32Byte := make([]byte, 4)
	binary.LittleEndian.PutUint32(uint32Byte, num)
	return uint32Byte
}
