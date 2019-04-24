package enode

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

var testIdSlice = idGenerator()
var testId = ID(toArray(testIdSlice))

/*
	Test function Bytes to check if the byte array can be correctly convert to byte slice
*/

func TestBytes(t *testing.T) {
	idArray := toArray(testIdSlice)
	id := ID(idArray).Bytes()
	if !bytes.Equal(testIdSlice, id) {
		t.Errorf("Error Converting byte array to byte slice")
	}
}

/*
	Test function String to check if the byte array ID can be correctly converted to hexadecimal string
*/

func TestString(t *testing.T) {
	result := testId.String()

	if reflect.TypeOf(result).Kind() != reflect.String {
		t.Errorf("Error converting byte array ID to hexdecimal string")
	}

	if len(result) != 64 {
		t.Errorf("Converted hexadecimal string has wrong length. Got %d, expected 64",
			len(result))
	}
	fmt.Println(string(result))
	result = testId.GoString()
	fmt.Println(result)
	result = testId.TerminalString()
	fmt.Println(result)
}
