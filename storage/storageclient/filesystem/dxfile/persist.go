package dxfile

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage/storageclient/erasurecode"
)

const (
	// PageSize is the page size of persist Data
	PageSize = 4096

	// sectorPersistSize is the size of rlp encoded string of a Sector
	sectorPersistSize = 70

	// Overhead for persistSegment persist Data. The value is larger than Data actually used
	segmentPersistOverhead = 32
)

type (
	// persistHostTable is unmarshaled form of hostTable. Instead of a map, it is marshaled as slice
	persistHostTable []*persistHostAddress

	// persistHostAddress is the persist Data structure for rlp encode and decode
	persistHostAddress struct {
		Address enode.ID
		Used    bool
	}

	// persistSegment is the structure a dxfile is split into
	persistSegment struct {
		Sectors [][]*Sector // Sectors contains the recoverable message about the persistSector in the persistSegment
		Index   uint64      // Index is the Index of the specific Segment
		Stuck   bool        // Stuck indicates whether the Segment is Stuck or not
	}

	// persistSector is the smallest unit of storage. It the erasure code encoded persistSegment
	persistSector struct {
		MerkleRoot common.Hash
		HostID     enode.ID
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

// Sector implements rlp encode rule
func (s *Sector) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, persistSector{
		MerkleRoot: s.MerkleRoot,
		HostID:     s.HostID,
	})
}

// Sector implements rlp decode rule
func (s *Sector) DecodeRLP(st *rlp.Stream) error {
	var ps persistSector
	if err := st.Decode(&ps); err != nil {
		return err
	}
	s.MerkleRoot, s.HostID = ps.MerkleRoot, ps.HostID
	return nil
}

// Segment implements rlp encode rule to encode the Sectors field
func (s *Segment) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, persistSegment{
		Sectors: s.Sectors,
		Index:   s.Index,
		Stuck:   s.Stuck,
	})
}

// Segment implements rlp decode rule to decode the Sectors field
func (s *Segment) DecodeRLP(st *rlp.Stream) error {
	var ps persistSegment
	if err := st.Decode(&ps); err != nil {
		return err
	}
	s.Sectors, s.Index, s.Stuck = ps.Sectors, ps.Index, ps.Stuck
	return nil
}

// segmentPersistSize is the helper function to calculate the number of pages to be used for
// the persist of a Segment
func segmentPersistNumPages(numSectors uint32) uint64 {
	sectorsSize := sectorPersistSize * numSectors
	dataSize := segmentPersistOverhead + sectorsSize
	numPages := dataSize / PageSize
	if dataSize%PageSize != 0 {
		numPages++
	}
	return uint64(numPages)
}

// validate validate the params in the metadata. If a potential error found, return the error.
// Note validate will not check params for erasure coding or cipher key since they are checked in other functions.
func (md Metadata) validate() error {
	if md.HostTableOffset != PageSize {
		return fmt.Errorf("HostTableOffset unexpected: %d != %d", md.HostTableOffset, PageSize)
	}
	if md.SegmentOffset <= md.HostTableOffset {
		return fmt.Errorf("SegmentOffset not larger than hostTableOffset: %d <= %d", md.SegmentOffset, md.HostTableOffset)
	}
	if md.SegmentOffset%PageSize != 0 {
		return fmt.Errorf("Segment Offset not divisible by PageSize %d %% %d != 0", md.SegmentOffset, PageSize)
	}
	return nil
}

// newErasureCode is the helper function to create the erasureCoder based on metadata params
func (md Metadata) newErasureCode() (erasurecode.ErasureCoder, error) {
	switch md.ErasureCodeType {
	case erasurecode.ECTypeStandard:
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors)
	case erasurecode.ECTypeShard:
		var shardSize int
		if len(md.ECExtra) >= 4 {
			shardSize = int(binary.LittleEndian.Uint32(md.ECExtra))
		} else {
			shardSize = erasurecode.EncodedShardUnit
		}
		return erasurecode.New(md.ErasureCodeType, md.MinSectors, md.NumSectors, shardSize)
	default:
		return nil, erasurecode.ErrInvalidECType
	}
}

// erasureCodeToParams is the the helper function to interpret the erasureCoder to params
// return minSectors, numSectors, and extra
func erasureCodeToParams(ec erasurecode.ErasureCoder) (uint32, uint32, []byte) {
	minSectors := ec.MinSectors()
	numSectors := ec.NumSectors()
	switch ec.Type() {
	case erasurecode.ECTypeStandard:
		return minSectors, numSectors, nil
	case erasurecode.ECTypeShard:
		extra := ec.Extra()
		extraBytes := make([]byte, 4)
		shardSize := extra[0].(int)
		binary.LittleEndian.PutUint32(extraBytes, uint32(shardSize))
		return minSectors, numSectors, extraBytes
	default:
		panic("erasure code type not recognized")
	}
}

// newCipherKey create a new cipher key based on metadata params
func (md Metadata) newCipherKey() (crypto.CipherKey, error) {
	return crypto.NewCipherKey(md.CipherKeyCode, md.CipherKey)
}

// segmentSize is the helper function to calculate the Segment size based on metadata info
func (md Metadata) segmentSize() uint64 {
	return md.SectorSize * uint64(md.MinSectors)
}

// numSegments is the number of segments of a dxfile based on metadata info
func (md Metadata) numSegments() uint64 {
	num := md.FileSize / md.segmentSize()
	if md.FileSize%md.segmentSize() != 0 || num == 0 {
		num++
	}
	return num
}
