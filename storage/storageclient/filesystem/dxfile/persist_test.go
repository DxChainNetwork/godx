// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"reflect"
	"testing"
)

func TestSegmentPersistNumPages(t *testing.T) {
	tests := []struct {
		numSectors uint32
	}{
		{0},
		{1},
		{100},
		{100000},
	}
	for i, test := range tests {
		numPages:= segmentPersistNumPages(test.numSectors)
		seg := randomSegment(test.numSectors)
		segBytes, _ := rlp.EncodeToBytes(seg)
		if PageSize * int(numPages) < len(segBytes) {
			t.Errorf("Test %d: pages not enough to hold data: %d * %d < %d", i, PageSize, numPages, len(segBytes))
		}
	}
}

func TestHostAddress_DecodeRLP_EncodeRLP(t *testing.T) {
	tests := []*hostAddress{
		{address: randomAddress(), used: false},
		{address: randomAddress(), used: true},
	}
	for i, test := range tests {
		b, err := rlp.EncodeToBytes(test)
		if err != nil {
			t.Fatalf(err.Error())
		}
		var ha *hostAddress
		if err := rlp.DecodeBytes(b, &ha); err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(ha, test) {
			t.Errorf("Test %d: expect %+v, got %+v", i, test, ha)
		}
	}
}

func TestSegment_EncodeRLP_DecodeRLP(t *testing.T) {
	tests := []*segment{
		randomSegment(0),
		randomSegment(1),
		randomSegment(100),
	}
	for i, test := range tests {
		b, err := rlp.EncodeToBytes(test)
		if err != nil {
			t.Fatalf(err.Error())
		}
		var seg *segment
		if err := rlp.DecodeBytes(b, &seg); err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(seg.sectors, test.sectors) {
			t.Errorf("Test %d: expect %+v, got %+v", i, test, seg)
		}
	}
}

func randomSegment(numSectors uint32) *segment {
	seg := &segment{sectors: make([][]*sector, numSectors)}
	for i := range seg.sectors {
		seg.sectors[i] = append(seg.sectors[i], randomSector())
	}
	return seg
}

func randomSector() *sector {
	s := &sector{}
	rand.Read(s.hostAddress[:])
	rand.Read(s.merkleRoot[:])
	return s
}

func randomAddress() (addr common.Address) {
	rand.Read(addr[:])
	return
}
