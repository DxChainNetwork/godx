// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.
package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/twofishgcm"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
		numPages := segmentPersistNumPages(test.numSectors)
		seg := randomSegment(test.numSectors)
		segBytes, _ := rlp.EncodeToBytes(seg)
		if PageSize*int(numPages) < len(segBytes) {
			t.Errorf("Test %d: pages not enough to hold Data: %d * %d < %d", i, PageSize, numPages, len(segBytes))
		}
	}
}

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

func TestMetadata_EncodeRLP_DecodeRLP(t *testing.T) {
	meta := Metadata {
		HostTableOffset:     PageSize,
		SegmentOffset:       2*PageSize,
		FileSize:            randomUint64(),
		SectorSize:          randomUint64(),
		PagesPerSegment:     randomUint64(),
		LocalPath:           filepath.Join(testDir, t.Name()),
		DxPath:              t.Name(),
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

func randomHostTable(numHosts int) hostTable {
	ht := make(hostTable)
	for i := 0; i != numHosts; i++ {
		ht[randomAddress()] = randomBool()
	}
	return ht
}

func randomSegment(numSectors uint32) *segment {
	seg := &segment{sectors: make([][]*sector, numSectors)}
	for i := range seg.sectors {
		seg.sectors[i] = append(seg.sectors[i], randomSector())
	}
	seg.index = randomUint64()
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

func randomBool() bool {
	b := make([]byte, 2)
	rand.Read(b)
	num := binary.LittleEndian.Uint16(b)
	return num%2 == 0
}

func randomUint64() uint64 {
	b := make([]byte, 8)
	rand.Read(b)
	return binary.LittleEndian.Uint64(b)
}

func randomBytes(num int) []byte{
	b := make([]byte, num)
	rand.Read(b)
	return b
}
