// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/twofishgcm"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

// TestSegmentPersistNumPages test function segmentPersistNumPages
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
		numPages := segmentPersistNumPages(test.numSectors)
		seg := randomSegment(test.numSectors)
		segBytes, _ := rlp.EncodeToBytes(seg)
		if PageSize*int(numPages) < len(segBytes) {
			t.Errorf("Test %d: pages not enough to hold Data: %d * %d < %d", i, PageSize, numPages, len(segBytes))
		}
	}
}

// TestHostTable_DecodeRLP_EncodeRLP test the decode and encode RLP rule for HostTable
func TestHostTable_DecodeRLP_EncodeRLP(t *testing.T) {
	tests := []hostTable{
		randomHostTable(0),
		randomHostTable(1),
		randomHostTable(1024),
	}
	for i, test := range tests {
		b, err := rlp.EncodeToBytes(test)
		if err != nil {
			t.Fatalf(err.Error())
		}
		ht := make(hostTable)
		if err := rlp.DecodeBytes(b, &ht); err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(test, ht) {
			t.Errorf("Test %d: expect %+v, got %+v", i, test, ht)
		}
	}
}

// TestSegment_EncodeRLP_DecodeRLP test the RLP decode and encode rule for Segment
func TestSegment_EncodeRLP_DecodeRLP(t *testing.T) {
	tests := []*Segment{
		randomSegment(0),
		randomSegment(1),
		randomSegment(100),
	}
	for i, test := range tests {
		b, err := rlp.EncodeToBytes(test)
		if err != nil {
			t.Fatalf(err.Error())
		}
		var seg *Segment
		if err := rlp.DecodeBytes(b, &seg); err != nil {
			t.Fatalf(err.Error())
		}
		if !reflect.DeepEqual(seg.Sectors, test.Sectors) {
			t.Errorf("Test %d: expect %+v, got %+v", i, test, seg)
		}
	}
}

// TestMetadata_EncodeRLP_DecodeRLP test the RLP decode and encode rule for Metadata
func TestMetadata_EncodeRLP_DecodeRLP(t *testing.T) {
	path, err := storage.NewDxPath(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	meta := Metadata{
		HostTableOffset:     PageSize,
		SegmentOffset:       2 * PageSize,
		FileSize:            randomUint64(),
		SectorSize:          randomUint64(),
		LocalPath:           testDir.Join(path),
		DxPath:              path,
		CipherKeyCode:       crypto.GCMCipherCode,
		CipherKey:           randomBytes(twofishgcm.GCMCipherKeyLength),
		TimeModify:          uint64(time.Now().Unix()),
		TimeUpdate:          uint64(time.Now().Unix()),
		TimeAccess:          uint64(time.Now().Unix()),
		TimeCreate:          uint64(time.Now().Unix()),
		Health:              100,
		StuckHealth:         100,
		TimeLastHealthCheck: uint64(time.Now().Unix()),
		NumStuckSegments:    32,
		TimeRecentRepair:    uint64(time.Now().Unix()),
		LastRedundancy:      100,
		FileMode:            0777,
		ErasureCodeType:     erasurecode.ECTypeShard,
		MinSectors:          10,
		NumSectors:          30,
		ECExtra:             []byte{},
		Version:             "1.0.0",
	}
	b, err := rlp.EncodeToBytes(meta)
	if err != nil {
		t.Fatalf(err.Error())
	}
	var md Metadata
	err = rlp.DecodeBytes(b, &md)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !reflect.DeepEqual(meta, md) {
		t.Errorf("not Equal\n\texpect %+v\n\tgot %+v", meta, md)
	}
}
