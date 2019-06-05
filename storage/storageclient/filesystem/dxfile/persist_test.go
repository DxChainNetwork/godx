// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/DxChainNetwork/godx/common"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/crypto/twofishgcm"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
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
	meta := Metadata{
		HostTableOffset:     PageSize,
		SegmentOffset:       2 * PageSize,
		FileSize:            randomUint64(),
		SectorSize:          randomUint64(),
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

// randomHostTable create a random hostTable
func randomHostTable(numHosts int) hostTable {
	ht := make(hostTable)
	for i := 0; i != numHosts; i++ {
		ht[randomAddress()] = randomBool()
	}
	return ht
}

// randomSegment create a random segment
func randomSegment(numSectors uint32) *Segment {
	seg := &Segment{Sectors: make([][]*Sector, numSectors)}
	for i := range seg.Sectors {
		seg.Sectors[i] = append(seg.Sectors[i], randomSector())
	}
	return seg
}

// randomSector create a random sector
func randomSector() *Sector {
	s := &Sector{}
	rand.Read(s.HostID[:])
	rand.Read(s.MerkleRoot[:])
	return s
}

// randomAddress create a random enodeID
func randomAddress() (addr enode.ID) {
	rand.Read(addr[:])
	return
}

// randomBool create a random true/false
func randomBool() bool {
	b := make([]byte, 2)
	rand.Read(b)
	num := binary.LittleEndian.Uint16(b)
	return num%2 == 0
}

// randomUint64 create a random Uint64
func randomUint64() uint64 {
	b := make([]byte, 8)
	rand.Read(b)
	return binary.LittleEndian.Uint64(b)
}

// randomBytes create a random bytes of size input num
func randomBytes(num int) []byte {
	b := make([]byte, num)
	rand.Read(b)
	return b
}

// randomHash creates a random hash
func randomHash() common.Hash {
	var h common.Hash
	rand.Read(h[:])
	return h
}
