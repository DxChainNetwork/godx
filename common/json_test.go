package common

import (
	"os"
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

var filename = "test.json"

func TestJSONCompat(t *testing.T) {
	p := person{
		Name: "mzhang",
		Age:  30,
	}
	err := SaveJSONCompat(metadata, filename, p)
	if err != nil {
		t.Errorf("error: %s", err.Error())
	}

	time.Sleep(time.Second)

	var p1 = person{}
	err = LoadJSONCompat(metadata, filename, p1)
	if err != nil {
		t.Errorf("error loading: %s", err.Error())
	}

	err = os.Remove(filename)
	if err != nil {
		t.Errorf("failed to remove the file: %s", err.Error())
	}
}
