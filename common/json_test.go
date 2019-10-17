// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package common

import (
	"runtime"
	"testing"
	"time"
)

type person struct {
	Name string
	Age  int
}

var metadata = Metadata{
	Header:  "DxChain JSON Test",
	Version: "1.3.0",
}

var testFile = "./testdata/test.json"
var persistFile = "./testdata/persist.json"
var corruptedFile = "./testdata/corrupted.json"
var manualFile = "./testdata/manual.json"

var testData = person{
	Name: "mzhang",
	Age:  30,
}

func init() {
	if runtime.GOOS == "windows" {
		testFile = "./testdata/windows/test.json"
		persistFile = "./testdata/windows/persist.json"
		corruptedFile = "./testdata/windows/corrupted.json"
		manualFile = "./testdata/windows/manual.json"
	}
}

// test simple save and load data
func TestJSONCompat(t *testing.T) {
	err := SaveDxJSON(metadata, testFile, testData)
	if err != nil {
		t.Fatalf("error: %s \n", err.Error())
	}

	time.Sleep(time.Microsecond)

	var p1 = person{}
	err = LoadDxJSON(metadata, testFile, p1)
	if err != nil {
		t.Fatalf("error loading: %s \n", err.Error())
	}
}

// test hash value unequal error
func TestCorruptedFile(t *testing.T) {
	err := LoadDxJSON(metadata, corruptedFile, testData)
	if err == nil || err.Error() != ErrCorrupted.Error() {
		t.Errorf("error: %v", err)
	}
}

// test loading manual hash value
func TestManualHash(t *testing.T) {
	var p1 = person{}
	err := LoadDxJSON(metadata, manualFile, p1)
	if err != nil {
		t.Fatalf("error loading: %s \n", err.Error())
	}
}

// test file validation suffix error
func TestFileSuffixError(t *testing.T) {
	var p1 = person{}
	err := LoadDxJSON(metadata, manualFile+tempSuffix, p1)
	if err != ErrBadFilenameSuffix {
		t.Fatalf("error: %s \n", err.Error())
	}
}
