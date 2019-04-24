package rlp

import (
	"bytes"
	"fmt"
)

var dataStorage = new(bool)
var exampleReader = bytes.NewReader([]byte{0x80})

var dataStroage1 = new(string)
var exampleReader1 = bytes.NewReader([]byte{0x83, 0x66, 0x66, 0x66})

func ExampleDecoder() {
	err := Decode(exampleReader, dataStorage)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(*dataStorage)

	// Output: false
}

func ExampleDecoder2() {
	err := Decode(exampleReader1, dataStroage1)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(*dataStroage1)
	// Output: fff
}
