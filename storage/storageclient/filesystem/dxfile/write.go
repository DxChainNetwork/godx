package dxfile

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/DxChainNetwork/godx/rlp"
)

// saveAll save all contents of a DxFile to the file.
func (df *DxFile) saveAll() error {
	if df.deleted {
		return errors.New("cannot save the file: file already deleted")
	}
	var updates []dxfileUpdate
	up, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	pagesHostTable := hostTableSize / PageSize
	if hostTableSize%PageSize != 0 {
		pagesHostTable++
	}
	df.metadata.SegmentOffset = df.metadata.HostTableOffset + PageSize*pagesHostTable
	segmentPersistSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	for i := range df.segments {
		offset := df.metadata.SegmentOffset + uint64(i)*segmentPersistSize
		update, err := df.createSegmentUpdate(uint64(i), offset)
		if err != nil {
			return err
		}
		df.segments[i].offset = offset
		updates = append(updates, update)
	}
	up, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	return df.applyUpdates(updates)
}

// rename create a series of transactions to rename the file to a new file
func (df *DxFile) rename(dxPath string, newFilePath string) error {
	if df.deleted {
		return errors.New("cannot rename the file: file already deleted")
	}
	var updates []dxfileUpdate
	du, err := df.createDeleteUpdate()
	if err != nil {
		return fmt.Errorf("cannot create delete update: %v", err)
	}
	updates = append(updates, du)
	df.filePath = newFilePath
	df.metadata.DxPath = dxPath
	up, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	pagesHostTable := hostTableSize / PageSize
	if hostTableSize%PageSize != 0 {
		pagesHostTable++
	}
	df.metadata.SegmentOffset = df.metadata.HostTableOffset + PageSize*pagesHostTable
	segmentPersistSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	for i := range df.segments {
		offset := df.metadata.SegmentOffset + uint64(i)*segmentPersistSize
		update, err := df.createSegmentUpdate(uint64(i), offset)
		if err != nil {
			return err
		}
		updates = append(updates, update)
	}
	up, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	return df.applyUpdates(updates)
}

// delete create and apply the deletion update
func (df *DxFile) delete() error {
	if df.deleted {
		return errors.New("file already deleted")
	}
	du, err := df.createDeleteUpdate()
	if err != nil {
		return fmt.Errorf("cannot create delete update: %v", err)
	}
	return df.applyUpdates([]dxfileUpdate{du})
}

// saveSegment save the segment with the segmentIndex, and write to file
func (df *DxFile) saveSegments(indexes []int) error {
	if df.deleted {
		return errors.New("cannot save the segment: file already deleted")
	}
	updates, err := df.createMetadataHostTableUpdate()
	if err != nil {
		return err
	}
	// Write the segment with the segmentIndex
	for _, index := range indexes {
		df.pruneSegment(index)
		seg := df.segments[index]
		if seg.index != uint64(index) {
			return fmt.Errorf("cannot write segment: data corrupted - segment index not expected")
		}
		up, err := df.createSegmentUpdate(uint64(index), seg.offset)
		if err != nil {
			return fmt.Errorf("cannot write segment: %v", err)
		}
		updates = append(updates, up)
		up, err = df.createMetadataUpdate()
		if err != nil {
			return err
		}
		updates = append(updates, up)
	}

	return df.applyUpdates(updates)
}

// saveHostTableUpdate save the host table as well as the metadata
func (df *DxFile) saveHostTableUpdate() error {
	if df.deleted {
		return errors.New("cannot save the host table: file already deleted")
	}
	updates, err := df.createMetadataHostTableUpdate()
	if err != nil {
		return err
	}
	return df.applyUpdates(updates)
}

// saveMetadata only save the metadata
func (df *DxFile) saveMetadata() error {
	if df.deleted {
		return errors.New("cannot save the metadata: file already deleted")
	}
	up, err := df.createMetadataUpdate()
	if err != nil {
		return err
	}
	return df.applyUpdates([]dxfileUpdate{up})
}

// createMetadataHostTableUpdate creates the update for metadata and hostTable
func (df *DxFile) createMetadataHostTableUpdate() ([]dxfileUpdate, error) {
	var updates []dxfileUpdate
	up, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return nil, err
	}
	updates = append(updates, up)

	shiftNeeded := hostTableSize > df.metadata.SegmentOffset-df.metadata.HostTableOffset
	if shiftNeeded {
		shiftUpdates, err := df.segmentShift(hostTableSize)
		if err != nil {
			return nil, err
		}
		updates = append(updates, shiftUpdates...)
	}
	up, err = df.createMetadataUpdate()
	if err != nil {
		return nil, err
	}
	updates = append(updates, up)
	return updates, nil
}

// segmentShift shift segment in persist file to the end of the persist file to give space for hostTable.
// Return the corresponding update and the underlying error.
func (df *DxFile) segmentShift(targetHostTableSize uint64) ([]dxfileUpdate, error) {
	f, err := os.OpenFile(df.filePath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %v: %v", df.filePath, err)
	}
	defer f.Close()

	shiftOffset, numSegToShift, segmentOffsetDiff := df.shiftOffset(targetHostTableSize)
	prevOffset := df.metadata.SegmentOffset
	segmentSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	var updates []dxfileUpdate
	for i := 0; uint64(i) < numSegToShift; i++ {
		seg, err := df.readSegment(f, prevOffset)
		if err != nil {
			return nil, err
		}
		newOffset := prevOffset + shiftOffset
		iu, err := df.createSegmentUpdate(seg.index, newOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to create segment update: %v", err)
		}
		updates = append(updates, iu)
		prevOffset += segmentSize
	}
	df.metadata.SegmentOffset += segmentOffsetDiff
	return updates, nil
}

// shiftOffset calculate for shift operation. return three offsets:
// 1. The size of shift
// 2. The number to segments to shift
// 3. Difference between new and old segment offset
func (df *DxFile) shiftOffset(targetHostTableSize uint64) (uint64, uint64, uint64) {
	if targetHostTableSize < df.metadata.SegmentOffset-df.metadata.HostTableOffset {
		return 0, 0, 0
	}
	numPagePerSeg := segmentPersistNumPages(df.metadata.NumSectors)
	sizePerSeg := PageSize * numPagePerSeg
	prevHostTableSize := df.metadata.SegmentOffset - df.metadata.HostTableOffset
	numShiftSeg := (targetHostTableSize - prevHostTableSize) / sizePerSeg
	if (targetHostTableSize-prevHostTableSize)%sizePerSeg != 0 {
		numShiftSeg++
	}
	numSeg := uint64(len(df.segments))
	if numShiftSeg > uint64(numSeg) {
		return numShiftSeg * sizePerSeg, numSeg, numShiftSeg * sizePerSeg
	}
	return numSeg * sizePerSeg, numShiftSeg, numShiftSeg * sizePerSeg
}

// createMetadataUpdate create an insert update for metadata
func (df *DxFile) createMetadataUpdate() (dxfileUpdate, error) {
	df.metadata.TimeUpdate = unixNow()
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
func (df *DxFile) createHostTableUpdate() (dxfileUpdate, uint64, error) {
	hostTableBytes, err := rlp.EncodeToBytes(df.hostTable)
	if err != nil {
		return nil, 0, err
	}
	iu, err := df.createInsertUpdate(df.metadata.HostTableOffset, hostTableBytes)
	if err != nil {
		return nil, 0, err
	}
	return iu, uint64(len(hostTableBytes)), nil
}

// createSegmentShiftUpdate create an segment update
func (df *DxFile) createSegmentUpdate(segmentIndex uint64, offset uint64) (dxfileUpdate, error) {
	if segmentIndex > uint64(len(df.segments)) {
		return nil, fmt.Errorf("unexpected index: %d", segmentIndex)
	}
	segment := df.segments[segmentIndex]
	if segment.index != segmentIndex {
		return nil, fmt.Errorf("data corrupted: segment index not align: %d != %d", segment.index, segmentIndex)
	}
	segment.index = segmentIndex
	segment.offset = offset
	segBytes, err := rlp.EncodeToBytes(segment)
	if err != nil {
		return nil, fmt.Errorf("cannot encode segment: %+v", segment)
	}
	// if the segment does not fit in, prune sectors with unused hosts
	if limit := PageSize * segmentPersistNumPages(df.metadata.NumSectors); uint64(len(segBytes)) > limit {
		return nil, fmt.Errorf("segment bytes exceed limit: %d > %d", len(segBytes), limit)
	}
	if int64(offset) < 0 {
		return nil, fmt.Errorf("uint64 overflow: %v", int64(offset))
	}
	return df.createInsertUpdate(offset, segBytes)
}

func unixNow() uint64 {
	return uint64(time.Now().Unix())
}
