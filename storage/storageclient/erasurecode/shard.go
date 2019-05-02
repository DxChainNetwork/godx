package erasurecode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// encodedShardSize is the size of piece unit size in sector
const encodedShardSize = 64

var ErrInsufficientData = errors.New("not sufficient data for recovery")

// shardErasureCode is the erasure code that segment is divided to pieces and each
// piece data could be independently recovered.
type shardErasureCode struct {
	standardErasureCode
}

// newShardErasureCode create a new shardErasureCode
func newShardErasureCode(minSectors, numSectors uint32) (*shardErasureCode, error) {
	sec, err := newStandardErasureCode(minSectors, numSectors)
	if err != nil {
		return nil, err
	}
	return &shardErasureCode{*sec}, nil
}

func (sec *shardErasureCode) Type() uint8 {
	return ECTypeShard
}

// Encode encode the segment to sectors
func (sec *shardErasureCode) Encode(segment []byte) ([][]byte, error) {
	// append 0s if data is not divisible by shardSize
	shardSize := sec.shardSize()
	if tail := len(segment) % shardSize; tail != 0 {
		segment = append(segment, make([]byte, tail)...)
	}
	numShard := len(segment) / shardSize

	// Create shards to put encoded results
	encodedSectors := make([][]byte, sec.numSectors)
	for i := 0; i < len(encodedSectors); i++ {
		encodedSectors[i] = make([]byte, encodedShardSize * numShard)
	}

	raw := bytes.NewBuffer(segment)
	destOffset := 0
	for {
		offShift, err := sec.encodeShard(raw, shardSize, encodedSectors, destOffset)
		if err == io.EOF || offShift == 0 {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return encodedSectors, nil
}

// encodeShard is the helper function to read shardSize of data from the input reader as raw data,
// and call the underlying standard EC algorithm on the raw data.
func (sec *shardErasureCode) encodeShard(r io.Reader, shardSize int, dest [][]byte, destOffset int) (int, error) {
	shard := make([]byte, shardSize)
	n, err := r.Read(shard)
	if err == io.EOF || n == 0 {
		return 0, io.EOF
	}
	if err != nil {
		return 0, err
	}
	if n != shardSize {
		return 0, fmt.Errorf("cannot read the whole shard expect %d, have %d", shardSize, n)
	}
	encodedShards, err := sec.standardErasureCode.Encode(shard)
	if err != nil {
		return 0, fmt.Errorf("cannot encode data: %x", shard)
	}
	if len(encodedShards) != len(dest) {
		return 0, fmt.Errorf("dest not have expected size: expect %d, have %d", len(encodedShards), len(dest))
	}
	offShift := len(encodedShards[0])
	for i := 0; i < len(dest); i++ {
		copy(dest[i][destOffset:], encodedShards[i])
	}
	return offShift, nil
}

// Recover write the recovered data to w as size of outSize
func (sec *shardErasureCode) Recover(sectors [][]byte, outSize int, w io.Writer) error {
	err := sec.recoveryDataCheck(sectors, outSize)
	if err != nil {
		return err
	}
	recoveredSize := 0
	for recoveredSize < outSize {
		recoverSize := sec.shardSize()
		if outSize < recoveredSize + encodedShardSize {
			recoverSize = outSize - recoveredSize
		}
		shardData := sec.prepareNextShardData(sectors, recoverSize)
		if err != nil {
			return fmt.Errorf("cannot recover: %v", err)
		}
		err := sec.standardErasureCode.Recover(shardData, recoverSize, w)
		if err != nil {
			return fmt.Errorf("cannot recover: %v", err)
		}
		recoveredSize += recoveredSize
	}
	return nil
}

// recoveryDataCheck is the helper function for sanity check called by Recover
func (sec *shardErasureCode) recoveryDataCheck(sectors [][]byte, outSize int) error {
	if shardSize := sec.shardSize(); outSize % shardSize != 0 {
		return fmt.Errorf("outSize not divisible by shardSize: %d %% %d", outSize, shardSize)
	}
	if uint32(len(sectors)) != sec.NumSectors() {
		return fmt.Errorf("input sectors not match numSectors: %d != %d", len(sectors), sec.numSectors)
	}
	if outSize < 0 {
		return fmt.Errorf("negative outSize: %d", outSize)
	}
	sectorSize, validSectors := 0, 0
	for _, sector := range sectors {
		if sector == nil || len(sector) == 0 {
			// invalid sector
			continue
		}
		validSectors++
		if sectorSize == 0 {
			if len(sector) % encodedShardSize != 0 {
				return fmt.Errorf("sectorSize not divisible by encodedShardSize: %d %% %d != 0", sectorSize, encodedShardSize)
			}
			if uint32(len(sector)) * sec.numSectors < uint32(outSize) {
				return ErrInsufficientData
			}
			sectorSize = len(sector)
		} else if len(sector) != sectorSize {
			// sectors not in equal size
			return fmt.Errorf("sector length not equal: %d != %d", len(sector), sectorSize)
		}
	}
	if uint32(validSectors) < sec.minSectors{
		return ErrInsufficientData
	}
	return nil
}

// prepareNextShardData is the helper function to prepare the data used for next recover.
func (sec *shardErasureCode) prepareNextShardData(sectors [][]byte, offset int) [][]byte {
	data := make([][]byte, len(sectors))
	for i, sector := range sectors {
		data[i] = sector[offset: offset+encodedShardSize]
	}
	return data
}

// shardSize is a helper function that return the size a segment should be sharded to
func (sec *shardErasureCode) shardSize() int {
	return encodedShardSize * int(sec.minSectors)
}




