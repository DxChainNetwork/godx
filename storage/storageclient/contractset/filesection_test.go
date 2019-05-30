// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractset

import (
	"encoding/json"
	"os"
	"path"
	"testing"
)

var testDataPath = "./testdata"

func TestFileSection_NewFileSection(t *testing.T) {
	filename := "testnew.log"
	file, err := openFile(path.Join(testDataPath, filename))
	if err != nil {
		t.Errorf("failed to open the file %s: %s", filename, err.Error())
	}

	_, err = newFileSection(file, -100, 50)
	if err == nil {
		t.Errorf("by setting the start position to be negative number, the error should be returned")
	}

	_, err = newFileSection(file, 20, 10)
	if err == nil {
		t.Errorf("since the end index 10 is smaller than the start index 20, error should be returned")
	}

	_, err = newFileSection(file, 100, -1)
	if err != nil {
		t.Errorf("failed to create newFileSection %s", err.Error())
	}
}

func TestFileSection_WriteRead(t *testing.T) {
	filename := "write.log"
	msg := "THIS IS A TEST"

	fs, _, length, err := fileSectionWriteData(filename, msg, 0, remainingFile)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	// read the data from the file section
	result := make([]byte, length)
	err = fs.ReadAt(result, 0)
	if err != nil {
		t.Fatalf("failed to read data from the file section")
	}

	// decode the JSON message
	var decodeJSON string
	err = json.Unmarshal(result, &decodeJSON)
	if err != nil {
		t.Fatalf("failed to decode the JSON message: %s", err.Error())
	}

	if decodeJSON != msg {
		t.Errorf("failed to read the exact data from the file section: expected %s, got %s",
			msg, decodeJSON)
	}
}

func TestFileSection_Size(t *testing.T) {
	filename := "size.log"
	msg := "THIS IS A SIZE TEST"

	fs, file, _, err := fileSectionWriteData(filename, msg, 0, remainingFile)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	size, err := fs.Size()
	if err != nil {
		t.Fatalf("failed to get the size of the file section")
	}

	fi, err := file.Stat()
	if err != nil {
		t.Fatalf("failed to get the status of the file: %s", err.Error())
	}

	if fi.Size()-fs.start != size {
		t.Errorf("expected size of file section %v, got %v", fi.Size()-fs.start, size)
	}

}

func TestFileSection_ReadWriteValidation(t *testing.T) {
	filename := "validate.log"
	msg := "THIS IS A Validation TEST"

	fs, _, length, err := newTestFileSection(filename, msg, 0, 1)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	defer fs.Close()

	if err := fs.readWriteValidation(-100, length); err == nil {
		t.Fatalf("by providing negative offset, the error should occur")
	}

	if err := fs.readWriteValidation(0, length); err == nil {
		t.Fatalf("data length %v + start 0 + offset 0 is greater than end index 1, error should returned",
			length)
	}

	if err := fs.readWriteValidation(0, 0); err != nil {
		t.Errorf("failed the read and write validation: %s", err.Error())
	}
}

func fileSectionWriteData(filename string, msg string, start, end int64) (fs *fileSection, file *os.File, length int, err error) {
	file, err = openFile(path.Join(testDataPath, filename))
	if err != nil {
		return
	}

	fs, err = newFileSection(file, start, end)
	if err != nil {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	length = len(data)

	err = fs.WriteAt(data, 0)
	if err != nil {
		return
	}

	return
}

func newTestFileSection(filename string, msg string, start, end int64) (fs *fileSection, file *os.File, length int, err error) {
	file, err = openFile(path.Join(testDataPath, filename))
	if err != nil {
		return
	}

	fs, err = newFileSection(file, start, end)
	if err != nil {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	length = len(data)
	return
}

func openFile(filename string) (file *os.File, err error) {
	file, err = os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0600)
	return
}
