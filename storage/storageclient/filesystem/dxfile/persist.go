package dxfile

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"io"
	"os"
)

const PageSize = 4096

// TODO: how to create updates to create dxfile

// readDxFile read and create a DxFile from disk specified with path.
func readDxFile(path string, wal *writeaheadlog.Wal) (*DxFile, error) {
	var ID fileID
	_, err := rand.Read(ID[:])
	if err != nil {
		return nil, fmt.Errorf("cannot create a random ID: %v", err)
	}
	df := &DxFile{
		ID:       ID,
		filename: path,
		wal:      wal,
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %v", path, err)
	}
	header, segmentOffset, err := readHeader(f)
	if err != nil {
		return nil, err
	}
	df.fileHeader = header
	// TODO: new df.erasureCoder
	// read segments
	off, err := f.Seek(segmentOffset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot read first segment: %v", err)
	}
	if off%PageSize != 0 {
		return nil, fmt.Errorf("segment offset not allowed: %d", off)
	}
	// TODO: implement page per segment
	segmentSize := df.fileHeader.SegmentPersistSize()
	for {
		seg, err := readSegment(f, segmentSize)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read segment: %v", err)
		}
		df.dataSegments = append(df.dataSegments, seg)
	}
	return df, nil
}

// readHeader is the helper function that read the header using rlp encoding,
// return the decoded header, segment offset, and error
func readHeader(r io.Reader) (*fileHeader, int64, error) {
	headerLength, segmentOffset, err := readOverhead(r)
	if err != nil {
		return nil, 0, err
	}
	if headerLength > segmentOffset {
		return nil, 0, fmt.Errorf("failed to read header: segmentOffset larger than header length")
	}
	headerBytes, err := readExactBytes(r, headerLength)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %v", err)
	}
	var ph *fileHeader
	if err = rlp.DecodeBytes(headerBytes, ph); err != nil {
		return nil, 0, fmt.Errorf("failed to decode header: %v", err)
	}
	return ph, segmentOffset, nil
}

// readOverhead read the first two uint64 as headerLength and segmentOffset
func readOverhead(r io.Reader) (int64, int64, error) {
	headerLength, err := readInt64(r)
	if err != nil {
		return 0, 0, fmt.Errorf("read headerLength: %v", err)
	}
	segmentOffset, err := readInt64(r)
	if err != nil {
		return 0, 0, fmt.Errorf("read segmentOffset: %v", err)
	}
	return headerLength, segmentOffset, nil
}

func readSegment(r io.Reader, length int64) (*Segment, error) {
	segmentBytes, err := readExactBytes(r, length)
	if err != nil {
		return nil, fmt.Errorf("failed to read segment: %v", err)
	}
	var seg *Segment
	if err = rlp.DecodeBytes(segmentBytes, seg); err != nil {
		return nil, err
	}
	return seg, nil
}

// readExactBytes is the helper function to read exactly length of data from r.
// If read data length is not the same as length, return an error.
func readExactBytes(r io.Reader, length int64) ([]byte, error) {
	dataBytes := make([]byte, length)
	n, err := r.Read(dataBytes)
	if n == 0 || err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	if int64(n) != length {
		return nil, fmt.Errorf("not enough data from reader: %d < %d", n, length)
	}
	return dataBytes, nil
}

// readInt64 is a helper function which read a uint64 from reader
func readInt64(r io.Reader) (int64, error) {
	var num int64
	err := binary.Read(r, binary.LittleEndian, &num)
	if err != nil {
		return 0, fmt.Errorf("cannot read uint64: %v", err)
	}
	return num, nil
}

func composeError(errs ...error) error {
	var errMsg = "["
	for _, err := range errs {
		if err == nil {
			continue
		}
		if len(errMsg) != 1 {
			errMsg += "; "
		}
		errMsg += err.Error()
	}
	errMsg += "]"
	if errMsg == "[]" {
		return nil
	}
	return errors.New(errMsg)
}
