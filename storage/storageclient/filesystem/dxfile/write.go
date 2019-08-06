// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
)

// TODO better error handling
// saveAll save all contents of a DxFile to the file.
func (df *DxFile) saveAll() error {
	if df.deleted {
		return errors.New("cannot save the file: file already deleted")
	}
	var updates []storage.FileUpdate
	// create updates for hostTable
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

	// create updates for segments
	for i := range df.segments {
		df.pruneSegment(i)
		offset := df.metadata.SegmentOffset + uint64(i)*segmentPersistSize
		update, err := df.createSegmentUpdate(uint64(i), offset)
		if err != nil {
			return err
		}
		df.segments[i].offset = offset
		updates = append(updates, update)
	}

	// create update for metadata
	up, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)

	// save all updates
	return storage.ApplyUpdates(df.wal, updates)
}

// rename create a series of transactions to rename the file to a new file
func (df *DxFile) rename(dxPath storage.DxPath, newFilePath storage.SysPath) error {
	if df.deleted {
		return errors.New("cannot rename the file: file already deleted")
	}
	var updates []storage.FileUpdate
	// create updates for delete
	du, err := df.createDeleteUpdate()
	if err != nil {
		return fmt.Errorf("cannot create delete update: %v", err)
	}
	updates = append(updates, du)
	df.filePath = newFilePath
	df.metadata.DxPath = dxPath
	// create updates for hostTable
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

	// create updates for segments
	for i := range df.segments {
		df.pruneSegment(i)
		offset := df.metadata.SegmentOffset + uint64(i)*segmentPersistSize
		update, err := df.createSegmentUpdate(uint64(i), offset)
		if err != nil {
			return err
		}
		updates = append(updates, update)
	}

	// create updates for metadata
	up, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	// apply updates
	return storage.ApplyUpdates(df.wal, updates)
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
	return storage.ApplyUpdates(df.wal, []storage.FileUpdate{du})
}

// saveSegment save the Segment with the segmentIndex, and write to file
func (df *DxFile) saveSegments(indexes []int) error {
	if df.deleted {
		return errors.New("cannot save the Segment: file already deleted")
	}
	// create updates for hostTable
	updates, err := df.createMetadataHostTableUpdate()
	if err != nil {
		return err
	}
	// Write the Segment with the segmentIndex
	for _, index := range indexes {
		df.pruneSegment(index)
		seg := df.segments[index]
		if seg.Index != uint64(index) {
			return fmt.Errorf("cannot write Segment: data corrupted - Segment Index not expected")
		}
		up, err := df.createSegmentUpdate(uint64(index), seg.offset)
		if err != nil {
			return fmt.Errorf("cannot write Segment: %v", err)
		}
		updates = append(updates, up)
	}
	// create updates for metadata
	up, err := df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, up)
	// apply the updates
	return storage.ApplyUpdates(df.wal, updates)
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
	return storage.ApplyUpdates(df.wal, updates)
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
	return storage.ApplyUpdates(df.wal, []storage.FileUpdate{up})
}

// createMetadataHostTableUpdate creates the update for metadata and hostTable
func (df *DxFile) createMetadataHostTableUpdate() ([]storage.FileUpdate, error) {
	var updates []storage.FileUpdate
	up, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return nil, err
	}
	updates = append(updates, up)
	// If the hostTable does not fit in the current space for hostTable, shift is needed for next segment
	shiftNeeded := hostTableSize > df.metadata.SegmentOffset-df.metadata.HostTableOffset
	if shiftNeeded {
		shiftUpdates, err := df.segmentShift(hostTableSize)
		if err != nil {
			return nil, err
		}
		updates = append(updates, shiftUpdates...)
	}
	// create the updates for metadata
	up, err = df.createMetadataUpdate()
	if err != nil {
		return nil, err
	}
	updates = append(updates, up)
	return updates, nil
}

// segmentShift shift Segment in persist file to the end of the persist file to give space for hostTable.
// Return the corresponding update and the underlying error.
func (df *DxFile) segmentShift(targetHostTableSize uint64) ([]storage.FileUpdate, error) {
	f, err := os.OpenFile(string(df.filePath), os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %v: %v", df.filePath, err)
	}
	defer f.Close()

	// calculate the offsets after the host table update
	shiftOffset, numSegToShift, segmentOffsetDiff := df.shiftOffset(targetHostTableSize)
	prevOffset := df.metadata.SegmentOffset
	segmentSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	// move the segment to the end of DxFile
	var updates []storage.FileUpdate
	for i := 0; uint64(i) < numSegToShift; i++ {
		seg, err := df.readSegment(f, prevOffset)
		if err != nil {
			return nil, err
		}
		newOffset := prevOffset + shiftOffset
		iu, err := df.createSegmentUpdate(seg.Index, newOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to create Segment update: %v", err)
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
// 3. Difference between new and old Segment offset
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
func (df *DxFile) createMetadataUpdate() (storage.FileUpdate, error) {
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
func (df *DxFile) createHostTableUpdate() (storage.FileUpdate, uint64, error) {
	df.metadata.TimeUpdate = unixNow()
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

// createSegmentShiftUpdate create an Segment update
func (df *DxFile) createSegmentUpdate(segmentIndex uint64, offset uint64) (storage.FileUpdate, error) {
	df.metadata.TimeModify = unixNow()
	if segmentIndex > uint64(len(df.segments)) {
		return nil, fmt.Errorf("unexpected Index: %d", segmentIndex)
	}
	segment := df.segments[segmentIndex]
	if segment.Index != segmentIndex {
		return nil, fmt.Errorf("data corrupted: Segment Index not align: %d != %d", segment.Index, segmentIndex)
	}
	segment.Index = segmentIndex
	segment.offset = offset
	segBytes, err := rlp.EncodeToBytes(segment)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Segment: %+v", segment)
	}
	// if the Segment does not fit in, prune Sectors with unused hosts
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
