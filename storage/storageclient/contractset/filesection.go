// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"errors"
	"fmt"
	"os"
)

// fileSection allow a file to be divided into sections. The user is able to
// read from or write at a particular file section
type fileSection struct {
	f     *os.File
	start int64
	end   int64
}

// newFileSection validate the arguments passed in and initialize a new fileSection
// object, which allows the data to be written into file sections
func newFileSection(f *os.File, start, end int64) (fs *fileSection, err error) {
	// start and end index validation
	if start < 0 {
		err = errors.New("filesection cannot start with index that is smaller than 0")
		return
	}

	if end < start && end != remainingFile {
		err = errors.New("filesection cannot end with an index that is smaller than the start index")
		return
	}

	// fileSection initialization
	fs = &fileSection{
		f:     f,
		start: start,
		end:   end,
	}

	return
}

// WriteAt will write the data into a specific file section
func (fs *fileSection) WriteAt(data []byte, offset int64) (err error) {
	// validation
	if err = fs.readWriteValidation(offset, len(data)); err != nil {
		return
	}

	// write data into file
	_, err = fs.f.WriteAt(data, fs.start+offset)
	return
}

// ReadAt will read the data from a specific file section
func (fs *fileSection) ReadAt(data []byte, offset int64) (err error) {
	// validation
	if err = fs.readWriteValidation(offset, len(data)); err != nil {
		return
	}

	// read data from the file
	_, err = fs.f.ReadAt(data, offset+fs.start)
	return
}

// Truncate will shrink the file to the size specified
// the size is measured in bytes
// for example: if size = 1, only one byte data will be remained in the file
func (fs *fileSection) Truncate(size int64) (err error) {
	// validation
	if size < 0 {
		return fmt.Errorf("the turncated file size cannot be smaller than 0")
	}

	if fs.end != remainingFile {
		return fmt.Errorf("error truncating the file section, the section can only be truncated if it is located at the end")
	}

	return fs.f.Truncate(size)
}

// Size returns the size of the file section
func (fs *fileSection) Size() (sectionSize int64, err error) {
	// get the size of the file
	fi, err := fs.f.Stat()
	if err != nil {
		return
	}

	fileSize := fi.Size()

	// get the section size
	sectionSize = fileSize - fs.start

	// section size cannot be smaller than 0
	if sectionSize < 0 {
		sectionSize = 0
	}

	// section size cannot be bigger than max end - start
	if sectionSize > fs.end-fs.start && fs.end != remainingFile {
		sectionSize = fs.end - fs.start
	}

	return
}

// Sync will force a file to be committed from the memory to disk,
// adding more reliability to the system in case the program crashed
// before the file closed
func (fs *fileSection) Sync() (err error) {
	return fs.f.Sync()
}

// Close will close the file
func (fs *fileSection) Close() error {
	return fs.f.Close()
}

// readWriteValidation will validate the arguments when executing writing
// or reading operation
func (fs *fileSection) readWriteValidation(offset int64, dataLength int) (err error) {
	if offset < 0 {
		err = errors.New("offset cannot be smaller than 0")
		return
	}
	dataCapacity := fs.start + offset + int64(dataLength)
	if dataCapacity > fs.end && fs.end != remainingFile {
		err = errors.New("the file section does not have enough space to write/read the data")
		return
	}
	return
}
