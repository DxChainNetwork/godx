package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
	"os"
)

func (df *DxFile) saveAll() error {

}

// saveHostTableUpdate save the host table as well as the metadata
func (df *DxFile) saveHostTableUpdate() error {
	var updates []dxfileUpdate
	iu, err := df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)
	iu, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)

	shiftNeeded := hostTableSize > df.metadata.SegmentOffset - df.metadata.HostTableOffset
	if shiftNeeded {
		shiftUpdates, err := df.segmentShift(hostTableSize)
		if err != nil {
			return err
		}
		updates = append(updates, shiftUpdates...)
	}
	return df.applyUpdates(updates)
}

// segmentShift shift the first segment in persist file to the end of the persist file.
// Return the corresponding update and the underlying error.
func (df *DxFile) segmentShift(targetHostTableSize uint64) ([]*insertUpdate, error) {
	f, err := os.OpenFile(df.filePath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %v: %v", df.filePath, err)
	}
	defer f.Close()

	var updates []*insertUpdate
	for targetHostTableSize > df.metadata.SegmentOffset - df.metadata.HostTableOffset {
		// read the segment at first segment
		seg, err := df.readSegment(f, df.metadata.SegmentOffset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// increment the segment offset
		df.metadata.SegmentOffset += segmentPersistNumPages(df.metadata.NumSectors) * PageSize
		newOffset := PageSize * segmentPersistNumPages(df.metadata.NumSectors) + df.metadata.SegmentOffset
		iu, err := df.createSegmentUpdate(seg.index, newOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to create segment update: %v", err)
		}
		updates = append(updates, iu)
	}
	return updates, nil
}

// saveMetadataUpdate create and save the metadata, return the list of update and underlying error
func (df *DxFile) saveMetadataUpdate() error{
	iu, err := df.createMetadataUpdate()
	if err != nil {
		return err
	}
	return df.applyUpdates([]dxfileUpdate{iu})
}

// createMetadataUpdate create an insert update for metadata
func (df *DxFile) createMetadataUpdate() (*insertUpdate, error) {
	metaBytes, err := rlp.EncodeToBytes(df.metadata)
	if err != nil {
		return nil, err
	}
	if len(metaBytes) > PageSize {
		// This shall never happen
		return nil, fmt.Errorf("metadata should not have length larger than %v", PageSize)
	}
	return df.createInsertUpdate(0, metaBytes)
}

// createHostTableUpdate create a hostTable update. Return the insertUpdate, size of hostTable bytes
// and the error
func (df *DxFile) createHostTableUpdate() (*insertUpdate, uint64, error) {
	hostTableBytes, err := rlp.EncodeToBytes(df.hostTable)
	if err != nil {
		return nil, 0, err
	}
	iu, err := df.createInsertUpdate(int64(df.metadata.HostTableOffset), hostTableBytes)
	if err != nil {
		return nil, 0, err
	}
	return iu, uint64(len(hostTableBytes)), nil
}

// createSegmentShiftUpdate create an segment update
func (df *DxFile) createSegmentUpdate(segmentIndex uint64, offset uint64) (*insertUpdate, error) {
	if segmentIndex > uint64(len(df.segments)) {
		return nil, fmt.Errorf("unexpected index: %d", segmentIndex)
	}
	segment := df.segments[segmentIndex]
	if segment.index != segmentIndex {
		return nil, fmt.Errorf("data corrupted: segment index not align: %d != %d", segment.index, segmentIndex)
	}
	segment.offset = offset
	segBytes, err := rlp.EncodeToBytes(segment)
	if err != nil {
		return nil, fmt.Errorf("cannot enocde segment: %+v", segment)
	}
	if limit := PageSize * segmentPersistNumPages(df.metadata.NumSectors); uint64(len(segBytes)) > limit {
		return nil, fmt.Errorf("segment bytes exceed limit: %d > %d", len(segBytes), limit)
	}
	if int64(offset) < 0 {
		return nil, fmt.Errorf("uint64 overflow: %v", int64(offset))
	}
	return df.createInsertUpdate(int64(offset), segBytes)
}

// TODO: implement this
func (df *DxFile) save() error {
	return nil
}
