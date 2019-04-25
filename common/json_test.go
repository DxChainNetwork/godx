package common

import (
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

// test simple save and load data
func TestJSONCompat(t *testing.T) {
	err := SaveJSONCompat(metadata, testFile, testData)
	if err != nil {
		t.Fatalf("error: %s \n", err.Error())
	}

	time.Sleep(time.Microsecond)

	var p1 = person{}
	err = LoadJSONCompat(metadata, testFile, p1)
	if err != nil {
		t.Fatalf("error loading: %s \n", err.Error())
	}
}

// test concurrent loading / saving errors
func TestLoadingSavingConcurrent(t *testing.T) {

	var errHandle = make(chan error)

	go func() {
		err := SaveJSONCompat(metadata, persistFile, testData)
		if err != nil {
			errHandle <- err
		}
	}()

	go func() {
		err := LoadJSONCompat(metadata, persistFile, testData)
		if err != nil {
			errHandle <- err
		}
	}()

	select {
	case err := <-errHandle:
		if err != ErrFileInUse {
			t.Fatalf("error: %s \n", err.Error())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("error: timeout")
	}
}

// test hash value unequal error
func TestCorruptedFile(t *testing.T) {
	err := LoadJSONCompat(metadata, corruptedFile, testData)
	if err.Error() != ErrCorrupted.Error() {
		t.Errorf("error: %s \n", err.Error())
	}
}

// test loading manual hash value
func TestManualHash(t *testing.T) {
	var p1 = person{}
	err := LoadJSONCompat(metadata, manualFile, p1)
	if err != nil {
		t.Fatalf("error loading: %s \n", err.Error())
	}
}

// test file validation suffix error
func TestFileSuffixError(t *testing.T) {
	var p1 = person{}
	err := LoadJSONCompat(metadata, manualFile+tempSuffix, p1)
	if err != ErrBadFilenameSuffix {
		t.Fatalf("error: %s \n", err.Error())
	}
}
