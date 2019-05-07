package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
)

const (
	// PageSize is the page size of persist data
	PageSize = 4096

	// sectorPersistSize is the size of rlp encoded string of a sector
	sectorPersistSize = 56

	// Overhead for persistSegment persist data. The value is larger than data actually used
	segmentPersistOverhead = 16

	// Duplication rate is the expected duplication of the size of the sectors.
	// A certain sector of certain index of a segment could have multiple sectors
	redundancyRate float64 = 1.0
)

type (
	// persistHostTable is unmarshaled form of hostTable. Instead of a map, it is marshaled as slice
	persistHostTable []*persistHostAddress

	// persistHostAddress is the persist data structure for rlp encode and decode
	persistHostAddress struct {
		Address common.Address
		Used    bool
	}

	// persistSegment is the structure a dxfile is split into
	persistSegment struct {
		Sectors [][]*sector // Sectors contains the recoverable message about the persistSector in the persistSegment
		Index   uint64      // Index is the index of the specific segment
	}

	// persistSector is the smallest unit of storage. It the erasure code encoded persistSegment
	persistSector struct {
		MerkleRoot  common.Hash
		HostAddress common.Address
	}
)

// hostTable implement rlp encode rule, and is encoded as a slice
func (ht hostTable) EncodeRLP(w io.Writer) error {
	var pht persistHostTable
	for addr, used := range ht {
		pha := &persistHostAddress{
			Address: addr,
			Used:    used,
		}
		pht = append(pht, pha)
	}
	return rlp.Encode(w, pht)
}

// hostTable implement rlp decode rule, and is decoded from a slice to map.
// Note if the receiver map already has some keys, the keys are removed.
func (ht hostTable) DecodeRLP(st *rlp.Stream) error {
	for k := range ht {
		delete(ht, k)
	}
	var pht persistHostTable
	if err := st.Decode(&pht); err != nil {
		return err
	}
	for _, pha := range pht {
		if _, found := ht[pha.Address]; found {
			return fmt.Errorf("multiple keys: %x", pha.Address)
		}
		ht[pha.Address] = pha.Used
	}
	return nil
}

// sector implements rlp encode rule
func (s *sector) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, persistSector{
		MerkleRoot:  s.merkleRoot,
		HostAddress: s.hostAddress,
	})
}

// sector implements rlp decode rule
func (s *sector) DecodeRLP(st *rlp.Stream) error {
	var ps persistSector
	if err := st.Decode(&ps); err != nil {
		return err
	}
	s.merkleRoot, s.hostAddress = ps.MerkleRoot, ps.HostAddress
	return nil
}

// segment implements rlp encode rule to encode the sectors field
func (s *segment) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, persistSegment{
		Sectors: s.sectors,
		Index: s.index,
	})
}

// segment implements rlp decode rule to decode the sectors field
func (s *segment) DecodeRLP(st *rlp.Stream) error {
	var ps persistSegment
	if err := st.Decode(&ps); err != nil {
		return err
	}
	s.sectors, s.index = ps.Sectors, ps.Index
	return nil
}

// segmentPersistSize is the helper function to calculate the number of pages to be used for
// the persist of a segment
func segmentPersistNumPages(numSectors uint32) uint64 {
	sectorsSize := sectorPersistSize * numSectors
	sectorsSizeWithRedundancy := float64(sectorsSize) * (1 + redundancyRate)
	dataSize := segmentPersistOverhead + int(sectorsSizeWithRedundancy)
	numPages := dataSize / PageSize
	if dataSize%PageSize != 0 {
		numPages++
	}
	return uint64(numPages)
}
