/*
	Public Functions:
		DecodeBytes -- Tested
		Decode -- Not tested explicitly, will be tested along with the example (NOTE: the original test cases already tested this function)
*/

package rlp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

var rawValueContainer = new(RawValue)
var boolContainer = new(bool)
var stringContainer = new(string)
var byteSliceContainer = new([]byte)
var byteArrayContainer = new([3]byte)
var sliceContainer = new([]string)
var arrayContainer = new([2]string)

var willReadStream = new(Stream)

func TestDecodeBytesWithSupportedDataTypes(t *testing.T) {
	tables := []struct {
		encodedData  []byte
		dataStorage  interface{}
		decodeResult interface{}
	}{
		{results["rawValueData"], rawValueContainer, rawValueData},
		{results["trueBoolData"], boolContainer, trueBoolData},
		{results["falseBoolData"], boolContainer, falseBoolData},
		{results["stringTypeData"], stringContainer, stringTypeData},
		{results["byteSliceTypeData"], byteSliceContainer, byteSliceTypeData},
		{results["byteSliceTypeData"], byteArrayContainer, byteArrayTypeData},
		{results["sliceTypeData"], sliceContainer, sliceTypeData},
		{results["arrayTypeData"], arrayContainer, arrayTypeData},
	}

	for _, table := range tables {
		err := DecodeBytes(table.encodedData, table.dataStorage)
		rval := reflect.ValueOf(table.dataStorage).Elem()

		if err != nil {
			t.Errorf("%s", err)
		} else if !comparator(rval.Type(), table.dataStorage, table.decodeResult) {
			t.Errorf("input %x, got %x want %x", table.encodedData, rval, table.decodeResult)
		}
	}
}

/*
    _____      _            _         ______                _   _               _______        _
   |  __ \    (_)          | |       |  ____|              | | (_)             |__   __|      | |
   | |__) | __ ___   ____ _| |_ ___  | |__ _   _ _ __   ___| |_ _  ___  _ __      | | ___  ___| |_
   |  ___/ '__| \ \ / / _` | __/ _ \ |  __| | | | '_ \ / __| __| |/ _ \| '_ \     | |/ _ \/ __| __|
   | |   | |  | |\ V / (_| | ||  __/ | |  | |_| | | | | (__| |_| | (_) | | | |    | |  __/\__ \ |_
   |_|   |_|  |_| \_/ \__,_|\__\___| |_|   \__,_|_| |_|\___|\__|_|\___/|_| |_|    |_|\___||___/\__|

*/

func TestWillReadSuccessful(t *testing.T) {
	tables := []struct {
		msg          string
		length       uint64
		nRead        uint64
		limited      bool
		stack        []listpos
		listPosition uint64
		remaining    uint64
	}{
		{"11IDONTKNOW", 11, 5, true, []listpos{}, 0, 6},
		{"13IDONTKNOWIT", 13, 0, true, []listpos{}, 0, 13},
		{"18IDONTKNOWITWANTI", 18, 18, false, []listpos{}, 0, 18},
		{"13IZZNTKNOWIT", 13, 2, true, []listpos{{0, 13}}, 2, 11},
	}

	for _, table := range tables {
		willReadStream.r = strings.NewReader(table.msg)
		willReadStream.remaining = table.length
		willReadStream.limited = table.limited
		willReadStream.stack = table.stack

		err := willReadStream.willRead(table.nRead)
		if err != nil {
			t.Errorf("%s", err)
		} else if willReadStream.remaining != table.remaining {
			t.Errorf("input msg %s, limited %t, read %d, got %d, want %d",
				table.msg, table.limited, table.nRead, willReadStream.remaining, table.remaining)
		} else if len(table.stack) > 1 && willReadStream.stack[0].pos != table.listPosition {
			t.Errorf("input msg %s, limited %t, read %d, got %d, want %d", table.msg,
				table.limited, table.nRead, willReadStream.stack[0].pos, table.listPosition)
		}
	}
}

func TestWillReadWithLengthSmallerThanNread(t *testing.T) {
	tables := []struct {
		msg     string
		length  uint64
		nRead   uint64
		limited bool
		stack   []listpos
		errMsg  error
	}{
		{"11IDONTKNOW", 11, 13, true, []listpos{}, ErrValueTooLarge},
		{"13IZZNTKNOWIT", 13, 15, true, []listpos{{0, 13}}, ErrElemTooLarge},
	}

	for _, table := range tables {
		willReadStream.r = strings.NewReader(table.msg)
		willReadStream.remaining = table.length
		willReadStream.limited = table.limited
		willReadStream.stack = table.stack

		err := willReadStream.willRead(table.nRead)
		if err == nil {
			t.Errorf("expected errors. input remaining %d, nRead %d", table.length, table.nRead)
		} else {
			if err != table.errMsg {
				t.Errorf("expected error message: %s, got %s", table.errMsg, err)
			}
		}
	}
}

func TestReadByteWithByteSliceTypeData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		result  byte
	}{
		{[]byte{0x81, 0x66}, 2, true, 0x81},
		{[]byte{0x80}, 1, true, 0x80},
		{[]byte{0x82, 0x66, 0x67}, 3, true, 0x82},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		returnResult, err := willReadStream.readByte()
		if err != nil {
			t.Errorf("%s", err)
		} else if returnResult != table.result {
			t.Errorf("input %v, got %x want %x", table.msg, returnResult, table.result)
		}
	}
}

func TestReadFull(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
	}{
		{[]byte{0x81, 0x54}, 2, true},
		{[]byte{0x80}, 1, true},
		{[]byte{0x82, 0x66, 0x67}, 3, true},
	}

	for _, table := range tables {
		var decodeTestBuf = make([]byte, table.total)
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.stack = willReadStream.stack[:0]

		err := willReadStream.readFull(decodeTestBuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.msg, decodeTestBuf) != 0 {
			t.Errorf("input %v, got %v want %v", table.msg, decodeTestBuf, table.msg)
		} else if willReadStream.remaining != 0 {
			t.Errorf("input %v, got %d want %d", table.msg, willReadStream.remaining, 0)
		}

	}
}

func TestReadUintWithEncodedUintData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		result  uint64
	}{
		{[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 8, true,
			18446744073709551615},
		{[]byte{}, 0, true, 0},
		{[]byte{0xFF}, 1, true, 255},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.uintbuf = make([]byte, 8)

		returnResult, err := willReadStream.readUint(byte(table.total))
		if err != nil {
			t.Errorf("%s", err)
		} else if returnResult != table.result {
			t.Errorf("input %v, got %d, want %d", table.msg, returnResult, table.result)
		}

	}
}

func TestReadKindWithSupportedDataKind(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		kind    Kind
		size    uint64
	}{
		{[]byte{0x7F}, 1, true, Byte, 0},
		{[]byte{0x81, 0x80}, 2, true, String, 1},
		{[]byte{0x82, 0x7F, 0x66}, 3, true, String, 2},
		{[]byte{0xC1, 0x7F}, 2, true, List, 1},
		{[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66},
			58, true, String, 56},
		{[]byte{0xF8, 0x41, 0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x94, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x8A, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x85, 0x66, 0x66, 0x66, 0x66, 0x66, 0x85, 0x66, 0x66,
			0x66, 0x66, 0x66}, 67, true, List, 65},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.uintbuf = make([]byte, 8)

		kind, size, err := willReadStream.readKind()
		if err != nil {
			t.Errorf("%s", err)
		} else if kind != table.kind {
			t.Errorf("input %v, got kind %v, expect kind %v", table.msg, kind, table.kind)
		} else if size != table.size {
			t.Errorf("input %v, got size %d, expect size %d", table.msg, size, table.size)
		}
	}
}

func TestHeadSize(t *testing.T) {
	tables := []struct {
		size           uint64
		returnHeadSize int
	}{
		{5, 1},
		{56, 2},
		{281474976710655, 7},
		{18446744073709551615, 9},
	}

	for _, table := range tables {
		result := headsize(table.size)
		if result != table.returnHeadSize {
			t.Errorf("input size %d, got %d want %d", table.size, result, table.returnHeadSize)
		}
	}
}

func TestRawWithRawValueTypeData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
	}{
		{[]byte{0x80}, 1, true},
		{[]byte{0x84, 0x11, 0x66, 0x78, 0x56}, 5, true},
		{[]byte{0xC5, 0x11, 0x66, 0x78, 0x56, 0x54}, 6, true},
		{[]byte{0xF8, 0x41, 0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x8A,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x85, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x85, 0x66, 0x66, 0x66, 0x66, 0x66}, 67, true},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.uintbuf = make([]byte, 8)

		returnResult, err := willReadStream.Raw()
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(returnResult, table.msg) != 0 {
			t.Errorf("input %v, got %v want %v", table.msg, returnResult, table.msg)
		}
	}
}

func TestBytesWithRlpEncodedByteSlice(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		result  []byte
	}{
		{[]byte{0x80}, 1, true, []byte{}},
		{[]byte{0x66}, 1, true, []byte{'f'}},
		{[]byte{0x82, 0x66, 0x65}, 3, true, []byte{0x66, 0x65}},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		returnResult, err := willReadStream.Bytes()
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.result, returnResult) != 0 {
			t.Errorf("input %x got %v want %v", table.msg, returnResult, table.result)
		}
	}
}

func TestUintWithRlpEncodedUintData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		maxbits int
		result  uint64
	}{
		{[]byte{0x82, 0x45, 0x60}, 3, true, 64, 17760},
		{[]byte{0x78}, 1, true, 64, 120},
		{[]byte{0x082, 0x40, 0x60}, 3, true, 16, 16480},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		returnResult, err := willReadStream.uint(table.maxbits)
		if err != nil {
			t.Errorf("%s", err)
		} else if returnResult != table.result {
			t.Errorf("input %x, got %d, want %d", table.msg, returnResult, table.result)
		}
	}
}

func TestBoolWithRlpEncodedBooleanData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		result  bool
	}{
		{[]byte{0x80}, 1, true, false},
		{[]byte{0x01}, 1, true, true},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		returnResult, err := willReadStream.Bool()
		if err != nil {
			t.Errorf("%s", err)
		} else if returnResult != table.result {
			t.Errorf("input %x, got %t, want %t", table.msg, returnResult, table.result)
		}
	}
}

var testBsPtr = new([]byte)

func TestDecodeByteSliceWithRlpEncodedByteSliceData(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
		result  []byte
	}{
		{[]byte{0x80}, 1, true, []byte{}},
		{[]byte{0x66}, 1, true, []byte{'f'}},
		{[]byte{0x82, 0x66, 0x65}, 3, true, []byte{0x66, 0x65}},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		rval := reflect.ValueOf(testBsPtr).Elem()
		err := decodeByteSlice(willReadStream, rval)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(*testBsPtr, table.result) != 0 {
			t.Errorf("input %x, got %x want %x", table.msg, *testBsPtr, table.result)
		}
	}
}

var testBaPtr0 = new([0]byte)
var testBaPtr1 = new([1]byte)
var testBaPtr2 = new([2]byte)

func TestDecodeByteArrayWithRlpEncodedByteArrayData(t *testing.T) {
	tables := []struct {
		baptr   interface{}
		msg     []byte
		total   uint64
		limited bool
		result  interface{}
	}{
		{testBaPtr0, []byte{0x80}, 1, true, [0]byte{}},
		{testBaPtr1, []byte{0x66}, 1, true, [1]byte{'f'}},
		{testBaPtr2, []byte{0x82, 0x66, 0x65}, 3, true, [2]byte{0x66, 0x65}},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		rval := reflect.ValueOf(table.baptr).Elem()
		tr := reflect.ValueOf(table.result)

		err := decodeByteArray(willReadStream, rval)
		if err != nil {
			t.Errorf("%s", err)
		} else if tr.Interface() != rval.Interface() {
			t.Errorf("input %x, get %x want %x", table.msg, rval, table.result)
		}
	}
}

var testArrayPtr0 = new([3]string)
var testArrayPtr1 = new([2]string)

func TestDecodeListArray(t *testing.T) {
	tables := []struct {
		ArrayPtr interface{}
		msg      []byte
		total    uint64
		limited  bool
		result   interface{}
		decode   decoder
	}{
		{testArrayPtr0, []byte{0xCA, 0x82, 0x66, 0x66, 0x82, 0x66, 0x66, 0x83, 0x66, 0x66,
			0x66}, 11, true, [3]string{"ff", "ff", "fff"}, decodeString},
		{testArrayPtr1, []byte{0xC2, 0x61, 0x66}, 3, true,
			[2]string{"a", "f"}, decodeString},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.kind = -1

		rval := reflect.ValueOf(table.ArrayPtr).Elem()
		tr := reflect.ValueOf(table.result)

		err := decodeListArray(willReadStream, rval, table.decode)
		if err != nil {
			t.Errorf("%s", err)
		} else if tr.Interface() != rval.Interface() {
			t.Errorf("input %x, get %x want %x", table.msg, rval, table.result)
		}
	}
}

var testSlicePtr0 = new([]string)
var testSlicePtr1 = new([]string)

func TestDecodeListSlice(t *testing.T) {
	tables := []struct {
		SlicePtr *[]string
		msg      []byte
		total    uint64
		limited  bool
		result   []string
		decode   decoder
	}{
		{testSlicePtr0, []byte{0xCA, 0x82, 0x66, 0x66, 0x82, 0x66, 0x66, 0x83, 0x66,
			0x66, 0x66}, 11, true, []string{"ff", "ff", "fff"}, decodeString},

		{testSlicePtr1, []byte{0xC2, 0x61, 0x66},
			3, true, []string{"a", "f"}, decodeString},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.stack = willReadStream.stack[:0]
		willReadStream.kind = -1

		rval := reflect.ValueOf(table.SlicePtr).Elem()

		err := decodeListSlice(willReadStream, rval, table.decode)
		if err != nil {
			t.Errorf("%s", err)
		} else if !equalSlice(table.result, *(table.SlicePtr)) {
			t.Errorf("input %x got %x want %x", table.msg, *(table.SlicePtr), table.result)
		}
	}
}

var boolPtrTest1 = new(bool)

func TestDecodeBool(t *testing.T) {
	tables := []struct {
		boolPtr *bool
		msg     []byte
		total   uint64
		limited bool
		result  bool
	}{
		{boolPtrTest1, []byte{0x01}, 1, true, true},
		{boolPtrTest1, []byte{0x80}, 1, true, false},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.stack = willReadStream.stack[:0]
		willReadStream.kind = -1

		rval := reflect.ValueOf(table.boolPtr).Elem()
		err := decodeBool(willReadStream, rval)
		if err != nil {
			t.Errorf("%s", err)
		} else if table.result != *table.boolPtr {
			t.Errorf("input %x, got %t, expected %t", table.msg, *table.boolPtr, table.result)
		}
	}
}

var uintPtr1 = new(uint)

func TestDecodeUint(t *testing.T) {
	tables := []struct {
		uintPtr *uint
		msg     []byte
		total   uint64
		limited bool
		result  uint
	}{
		{uintPtr1, []byte{0x0F}, 1, true, 15},
		{uintPtr1, []byte{0x81, 0xE1}, 2, true, 225},
		{uintPtr1, []byte{0x82, 0xFF, 0xFF}, 3, true, 65535},
		{uintPtr1, []byte{0x84, 0xFF, 0xFF, 0xFF, 0xFF}, 5, true, 4294967295},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited
		willReadStream.stack = willReadStream.stack[:0]
		willReadStream.kind = -1

		rval := reflect.ValueOf(table.uintPtr).Elem()

		err := decodeUint(willReadStream, rval)
		if err != nil {
			t.Errorf("%s", err)
		} else if *table.uintPtr != table.result {
			t.Errorf("input %x, got %d want %d", table.msg, *table.uintPtr, table.result)
		}
	}
}

var rawPtr = new(RawValue)

func TestDecodeRawValue(t *testing.T) {
	tables := []struct {
		msg     []byte
		total   uint64
		limited bool
	}{
		{[]byte{0x80}, 1, true},
		{[]byte{0x84, 0x11, 0x66, 0x78, 0x56}, 5, true},
		{[]byte{0xC5, 0x11, 0x66, 0x78, 0x56, 0x54}, 6, true},
		{[]byte{0xF8, 0x41, 0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x8A, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x85, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x85, 0x66, 0x66, 0x66, 0x66, 0x66}, 67, true},
	}

	for _, table := range tables {
		willReadStream.r = bytes.NewReader(table.msg)
		willReadStream.remaining = table.total
		willReadStream.limited = table.limited

		rval := reflect.ValueOf(rawPtr).Elem()

		err := decodeRawValue(willReadStream, rval)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(*rawPtr, table.msg) != 0 {
			t.Errorf("input %v, got %v want %v", table.msg, *rawPtr, table.msg)
		}
	}
}

func TestMakeDecoder(t *testing.T) {
	tables := []struct {
		val            interface{}
		ts             tags
		resultFunction decoder
	}{
		{raw, defaultTags, decodeRawValue},
		{bigData, defaultTags, decodeBigInt},
		{bigData, defaultTags, decodeBigInt},
		{*bigData, defaultTags, decodeBigIntNoPtr},
		{uintData, defaultTags, decodeUint},
		{boolData, defaultTags, decodeBool},
		{stringData, defaultTags, decodeString},
		{byteSliceData, defaultTags, decodeByteSlice},
		{byteArrayData, defaultTags, decodeByteArray},
	}

	for _, table := range tables {
		rtype := reflect.TypeOf(table.val)
		dec, err := makeDecoder(rtype, table.ts)

		sf1 := reflect.ValueOf(dec).Pointer()
		sf2 := reflect.ValueOf(table.resultFunction).Pointer()

		if err != nil {
			t.Errorf("%s", err)
		} else if sf1 != sf2 {
			t.Errorf("input type %s, got %v, want %v", rtype, sf1, sf2)
		}
	}
}

/*
     _____                                _ _   _               _______        _
    / ____|                              (_) | (_)             |__   __|      | |
   | |     ___  _ __ ___  _ __   ___  ___ _| |_ _  ___  _ __      | | ___  ___| |_
   | |    / _ \| '_ ` _ \| '_ \ / _ \/ __| | __| |/ _ \| '_ \     | |/ _ \/ __| __|
   | |___| (_) | | | | | | |_) | (_) \__ \ | |_| | (_) | | | |    | |  __/\__ \ |_
    \_____\___/|_| |_| |_| .__/ \___/|___/_|\__|_|\___/|_| |_|    |_|\___||___/\__|
                         | |
                         |_|
*/

func TestStreamKind(t *testing.T) {
	tests := []struct {
		input    string
		wantKind Kind
		wantLen  uint64
	}{
		{"00", Byte, 0},
		{"01", Byte, 0},
		{"7F", Byte, 0},
		{"80", String, 0},
		{"B7", String, 55},
		{"B90400", String, 1024},
		{"BFFFFFFFFFFFFFFFFF", String, ^uint64(0)},
		{"C0", List, 0},
		{"C8", List, 8},
		{"F7", List, 55},
		{"F90400", List, 1024},
		{"FFFFFFFFFFFFFFFFFF", List, ^uint64(0)},
	}

	for i, test := range tests {
		// using plainReader to inhibit input limit errors.
		s := NewStream(newPlainReader(unhex(test.input)), 0)
		kind, len, err := s.Kind()
		if err != nil {
			t.Errorf("test %d: Kind returned error: %v", i, err)
			continue
		}
		if kind != test.wantKind {
			t.Errorf("test %d: kind mismatch: got %d, want %d", i, kind, test.wantKind)
		}
		if len != test.wantLen {
			t.Errorf("test %d: len mismatch: got %d, want %d", i, len, test.wantLen)
		}
	}
}

func TestStreamErrors(t *testing.T) {
	withoutInputLimit := func(b []byte) *Stream {
		return NewStream(newPlainReader(b), 0)
	}
	withCustomInputLimit := func(limit uint64) func([]byte) *Stream {
		return func(b []byte) *Stream {
			return NewStream(bytes.NewReader(b), limit)
		}
	}

	type calls []string
	tests := []struct {
		string
		calls
		newStream func([]byte) *Stream // uses bytes.Reader if nil
		error     error
	}{
		{"C0", calls{"Bytes"}, nil, ErrExpectedString},
		{"C0", calls{"Uint"}, nil, ErrExpectedString},
		{"89000000000000000001", calls{"Uint"}, nil, errUintOverflow},
		{"00", calls{"List"}, nil, ErrExpectedList},
		{"80", calls{"List"}, nil, ErrExpectedList},
		{"C0", calls{"List", "Uint"}, nil, EOL},
		{"C8C9010101010101010101", calls{"List", "Kind"}, nil, ErrElemTooLarge},
		{"C3C2010201", calls{"List", "List", "Uint", "Uint", "ListEnd", "Uint"}, nil, EOL},
		{"00", calls{"ListEnd"}, nil, errNotInList},
		{"C401020304", calls{"List", "Uint", "ListEnd"}, nil, errNotAtEOL},

		// Non-canonical integers (e.g. leading zero bytes).
		{"00", calls{"Uint"}, nil, ErrCanonInt},
		{"820002", calls{"Uint"}, nil, ErrCanonInt},
		{"8133", calls{"Uint"}, nil, ErrCanonSize},
		{"817F", calls{"Uint"}, nil, ErrCanonSize},
		{"8180", calls{"Uint"}, nil, nil},

		// Non-valid boolean
		{"02", calls{"Bool"}, nil, errors.New("rlp: invalid boolean value: 2")},

		// Size tags must use the smallest possible encoding.
		// Leading zero bytes in the size tag are also rejected.
		{"8100", calls{"Uint"}, nil, ErrCanonSize},
		{"8100", calls{"Bytes"}, nil, ErrCanonSize},
		{"8101", calls{"Bytes"}, nil, ErrCanonSize},
		{"817F", calls{"Bytes"}, nil, ErrCanonSize},
		{"8180", calls{"Bytes"}, nil, nil},
		{"B800", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"B90000", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"B90055", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"BA0002FFFF", calls{"Bytes"}, withoutInputLimit, ErrCanonSize},
		{"F800", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"F90000", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"F90055", calls{"Kind"}, withoutInputLimit, ErrCanonSize},
		{"FA0002FFFF", calls{"List"}, withoutInputLimit, ErrCanonSize},

		// Expected EOF
		{"", calls{"Kind"}, nil, io.EOF},
		{"", calls{"Uint"}, nil, io.EOF},
		{"", calls{"List"}, nil, io.EOF},
		{"8180", calls{"Uint", "Uint"}, nil, io.EOF},
		{"C0", calls{"List", "ListEnd", "List"}, nil, io.EOF},

		{"", calls{"List"}, withoutInputLimit, io.EOF},
		{"8180", calls{"Uint", "Uint"}, withoutInputLimit, io.EOF},
		{"C0", calls{"List", "ListEnd", "List"}, withoutInputLimit, io.EOF},

		// Input limit errors.
		{"81", calls{"Bytes"}, nil, ErrValueTooLarge},
		{"81", calls{"Uint"}, nil, ErrValueTooLarge},
		{"81", calls{"Raw"}, nil, ErrValueTooLarge},
		{"BFFFFFFFFFFFFFFFFFFF", calls{"Bytes"}, nil, ErrValueTooLarge},
		{"C801", calls{"List"}, nil, ErrValueTooLarge},

		// Test for list element size check overflow.
		{"CD04040404FFFFFFFFFFFFFFFFFF0303", calls{"List", "Uint", "Uint", "Uint", "Uint", "List"}, nil, ErrElemTooLarge},

		// Test for input limit overflow. Since we are counting the limit
		// down toward zero in Stream.remaining, reading too far can overflow
		// remaining to a large value, effectively disabling the limit.
		{"C40102030401", calls{"Raw", "Uint"}, withCustomInputLimit(5), io.EOF},
		{"C4010203048180", calls{"Raw", "Uint"}, withCustomInputLimit(6), ErrValueTooLarge},

		// Check that the same calls are fine without a limit.
		{"C40102030401", calls{"Raw", "Uint"}, withoutInputLimit, nil},
		{"C4010203048180", calls{"Raw", "Uint"}, withoutInputLimit, nil},

		// Unexpected EOF. This only happens when there is
		// no input limit, so the reader needs to be 'dumbed down'.
		{"81", calls{"Bytes"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"81", calls{"Uint"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"BFFFFFFFFFFFFFFF", calls{"Bytes"}, withoutInputLimit, io.ErrUnexpectedEOF},
		{"C801", calls{"List", "Uint", "Uint"}, withoutInputLimit, io.ErrUnexpectedEOF},

		// This test verifies that the input position is advanced
		// correctly when calling Bytes for empty strings. Kind can be called
		// any number of times in between and doesn't advance.
		{"C3808080", calls{
			"List",  // enter the list
			"Bytes", // past first element

			"Kind", "Kind", "Kind", // this shouldn't advance

			"Bytes", // past second element

			"Kind", "Kind", // can't hurt to try

			"Bytes", // past final element
			"Bytes", // this one should fail
		}, nil, EOL},
	}

testfor:
	for i, test := range tests {
		if test.newStream == nil {
			test.newStream = func(b []byte) *Stream { return NewStream(bytes.NewReader(b), 0) }
		}
		s := test.newStream(unhex(test.string))
		rs := reflect.ValueOf(s)
		for j, call := range test.calls {
			fval := rs.MethodByName(call)
			ret := fval.Call(nil)
			err := "<nil>"
			if lastret := ret[len(ret)-1].Interface(); lastret != nil {
				err = lastret.(error).Error()
			}
			if j == len(test.calls)-1 {
				want := "<nil>"
				if test.error != nil {
					want = test.error.Error()
				}
				if err != want {
					t.Log(test)
					t.Errorf("test %d: last call (%s) error mismatch\ngot:  %s\nwant: %s",
						i, call, err, test.error)
				}
			} else if err != "<nil>" {
				t.Log(test)
				t.Errorf("test %d: call %d (%s) unexpected error: %q", i, j, call, err)
				continue testfor
			}
		}
	}
}

func TestStreamList(t *testing.T) {
	s := NewStream(bytes.NewReader(unhex("C80102030405060708")), 0)

	len, err := s.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len != 8 {
		t.Fatalf("List returned invalid length, got %d, want 8", len)
	}

	for i := uint64(1); i <= 8; i++ {
		v, err := s.Uint()
		if err != nil {
			t.Fatalf("Uint error: %v", err)
		}
		if i != v {
			t.Errorf("Uint returned wrong value, got %d, want %d", v, i)
		}
	}

	if _, err := s.Uint(); err != EOL {
		t.Errorf("Uint error mismatch, got %v, want %v", err, EOL)
	}
	if err = s.ListEnd(); err != nil {
		t.Fatalf("ListEnd error: %v", err)
	}
}

func TestStreamRaw(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{
			"C58401010101",
			"8401010101",
		},
		{
			"F842B84001010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101",
			"B84001010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101010101",
		},
	}
	for i, tt := range tests {
		s := NewStream(bytes.NewReader(unhex(tt.input)), 0)
		s.List()

		want := unhex(tt.output)
		raw, err := s.Raw()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, raw) {
			t.Errorf("test %d: raw mismatch: got %x, want %x", i, raw, want)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	r := bytes.NewReader(nil)

	if err := Decode(r, nil); err != errDecodeIntoNil {
		t.Errorf("Decode(r, nil) error mismatch, got %q, want %q", err, errDecodeIntoNil)
	}

	var nilptr *struct{}
	if err := Decode(r, nilptr); err != errDecodeIntoNil {
		t.Errorf("Decode(r, nilptr) error mismatch, got %q, want %q", err, errDecodeIntoNil)
	}

	if err := Decode(r, struct{}{}); err != errNoPointer {
		t.Errorf("Decode(r, struct{}{}) error mismatch, got %q, want %q", err, errNoPointer)
	}

	expectErr := "rlp: type chan bool is not RLP-serializable"
	if err := Decode(r, new(chan bool)); err == nil || err.Error() != expectErr {
		t.Errorf("Decode(r, new(chan bool)) error mismatch, got %q, want %q", err, expectErr)
	}

	if err := Decode(r, new(uint)); err != io.EOF {
		t.Errorf("Decode(r, new(int)) error mismatch, got %q, want %q", err, io.EOF)
	}
}

type decodeTest struct {
	input string
	ptr   interface{}
	value interface{}
	error string
}

type invalidTail1 struct {
	A uint `rlp:"tail"`
	B string
}

type invalidTail2 struct {
	A uint
	B string `rlp:"tail"`
}

type tailUint struct {
	A    uint
	Tail []uint `rlp:"tail"`
}

var (
	veryBigInt = big.NewInt(0).Add(
		big.NewInt(0).Lsh(big.NewInt(0xFFFFFFFFFFFFFF), 16),
		big.NewInt(0xFFFF),
	)
)

var decodeTests = []decodeTest{

	// booleans
	{input: "01", ptr: new(bool), value: true},
	{input: "80", ptr: new(bool), value: false},
	{input: "02", ptr: new(bool), error: "rlp: invalid boolean value: 2"},

	// integers
	{input: "05", ptr: new(uint32), value: uint32(5)},
	{input: "80", ptr: new(uint32), value: uint32(0)},
	{input: "820505", ptr: new(uint32), value: uint32(0x0505)},
	{input: "83050505", ptr: new(uint32), value: uint32(0x050505)},
	{input: "8405050505", ptr: new(uint32), value: uint32(0x05050505)},
	{input: "850505050505", ptr: new(uint32), error: "rlp: input string too long for uint32"},
	{input: "C0", ptr: new(uint32), error: "rlp: expected input string or byte for uint32"},
	{input: "00", ptr: new(uint32), error: "rlp: non-canonical integer (leading zero bytes) for uint32"},
	{input: "8105", ptr: new(uint32), error: "rlp: non-canonical size information for uint32"},
	{input: "820004", ptr: new(uint32), error: "rlp: non-canonical integer (leading zero bytes) for uint32"},
	{input: "B8020004", ptr: new(uint32), error: "rlp: non-canonical size information for uint32"},

	// slices
	{input: "C0", ptr: new([]uint), value: []uint{}},
	{input: "C80102030405060708", ptr: new([]uint), value: []uint{1, 2, 3, 4, 5, 6, 7, 8}},
	{input: "F8020004", ptr: new([]uint), error: "rlp: non-canonical size information for []uint"},

	// arrays
	{input: "C50102030405", ptr: new([5]uint), value: [5]uint{1, 2, 3, 4, 5}},
	{input: "C0", ptr: new([5]uint), error: "rlp: input list has too few elements for [5]uint"},
	{input: "C102", ptr: new([5]uint), error: "rlp: input list has too few elements for [5]uint"},
	{input: "C6010203040506", ptr: new([5]uint), error: "rlp: input list has too many elements for [5]uint"},
	{input: "F8020004", ptr: new([5]uint), error: "rlp: non-canonical size information for [5]uint"},

	// zero sized arrays
	{input: "C0", ptr: new([0]uint), value: [0]uint{}},
	{input: "C101", ptr: new([0]uint), error: "rlp: input list has too many elements for [0]uint"},

	// byte slices
	{input: "01", ptr: new([]byte), value: []byte{1}},
	{input: "80", ptr: new([]byte), value: []byte{}},
	{input: "8D6162636465666768696A6B6C6D", ptr: new([]byte), value: []byte("abcdefghijklm")},
	{input: "C0", ptr: new([]byte), error: "rlp: expected input string or byte for []uint8"},
	{input: "8105", ptr: new([]byte), error: "rlp: non-canonical size information for []uint8"},

	// byte arrays
	{input: "02", ptr: new([1]byte), value: [1]byte{2}},
	{input: "8180", ptr: new([1]byte), value: [1]byte{128}},
	{input: "850102030405", ptr: new([5]byte), value: [5]byte{1, 2, 3, 4, 5}},

	// byte array errors
	{input: "02", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "80", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "820000", ptr: new([5]byte), error: "rlp: input string too short for [5]uint8"},
	{input: "C0", ptr: new([5]byte), error: "rlp: expected input string or byte for [5]uint8"},
	{input: "C3010203", ptr: new([5]byte), error: "rlp: expected input string or byte for [5]uint8"},
	{input: "86010203040506", ptr: new([5]byte), error: "rlp: input string too long for [5]uint8"},
	{input: "8105", ptr: new([1]byte), error: "rlp: non-canonical size information for [1]uint8"},
	{input: "817F", ptr: new([1]byte), error: "rlp: non-canonical size information for [1]uint8"},

	// zero sized byte arrays
	{input: "80", ptr: new([0]byte), value: [0]byte{}},
	{input: "01", ptr: new([0]byte), error: "rlp: input string too long for [0]uint8"},
	{input: "8101", ptr: new([0]byte), error: "rlp: input string too long for [0]uint8"},

	// strings
	{input: "00", ptr: new(string), value: "\000"},
	{input: "8D6162636465666768696A6B6C6D", ptr: new(string), value: "abcdefghijklm"},
	{input: "C0", ptr: new(string), error: "rlp: expected input string or byte for string"},

	// big ints
	{input: "01", ptr: new(*big.Int), value: big.NewInt(1)},
	{input: "89FFFFFFFFFFFFFFFFFF", ptr: new(*big.Int), value: veryBigInt},
	{input: "10", ptr: new(big.Int), value: *big.NewInt(16)}, // non-pointer also works
	{input: "C0", ptr: new(*big.Int), error: "rlp: expected input string or byte for *big.Int"},
	{input: "820001", ptr: new(big.Int), error: "rlp: non-canonical integer (leading zero bytes) for *big.Int"},
	{input: "8105", ptr: new(big.Int), error: "rlp: non-canonical size information for *big.Int"},

	// structs
	{
		input: "C50583343434",
		ptr:   new(simplestruct),
		value: simplestruct{5, "444"},
	},
	{
		input: "C601C402C203C0",
		ptr:   new(recstruct),
		value: recstruct{1, &recstruct{2, &recstruct{3, nil}}},
	},

	// struct errors
	{
		input: "C0",
		ptr:   new(simplestruct),
		error: "rlp: too few elements for rlp.simplestruct",
	},
	{
		input: "C105",
		ptr:   new(simplestruct),
		error: "rlp: too few elements for rlp.simplestruct",
	},
	{
		input: "C7C50583343434C0",
		ptr:   new([]*simplestruct),
		error: "rlp: too few elements for rlp.simplestruct, decoding into ([]*rlp.simplestruct)[1]",
	},
	{
		input: "83222222",
		ptr:   new(simplestruct),
		error: "rlp: expected input list for rlp.simplestruct",
	},
	{
		input: "C3010101",
		ptr:   new(simplestruct),
		error: "rlp: input list has too many elements for rlp.simplestruct",
	},
	{
		input: "C501C3C00000",
		ptr:   new(recstruct),
		error: "rlp: expected input string or byte for uint, decoding into (rlp.recstruct).Child.I",
	},
	{
		input: "C0",
		ptr:   new(invalidTail1),
		error: "rlp: invalid struct tag \"tail\" for rlp.invalidTail1.A (must be on last field)",
	},
	{
		input: "C0",
		ptr:   new(invalidTail2),
		error: "rlp: invalid struct tag \"tail\" for rlp.invalidTail2.B (field type is not slice)",
	},
	{
		input: "C50102C20102",
		ptr:   new(tailUint),
		error: "rlp: expected input string or byte for uint, decoding into (rlp.tailUint).Tail[1]",
	},

	// struct tag "tail"
	{
		input: "C3010203",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{unhex("02"), unhex("03")}},
	},
	{
		input: "C20102",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{unhex("02")}},
	},
	{
		input: "C101",
		ptr:   new(tailRaw),
		value: tailRaw{A: 1, Tail: []RawValue{}},
	},

	// struct tag "-"
	{
		input: "C20102",
		ptr:   new(hasIgnoredField),
		value: hasIgnoredField{A: 1, C: 2},
	},

	// RawValue
	{input: "01", ptr: new(RawValue), value: RawValue(unhex("01"))},
	{input: "82FFFF", ptr: new(RawValue), value: RawValue(unhex("82FFFF"))},
	{input: "C20102", ptr: new([]RawValue), value: []RawValue{unhex("01"), unhex("02")}},

	// pointers
	{input: "00", ptr: new(*[]byte), value: &[]byte{0}},
	{input: "80", ptr: new(*uint), value: uintp(0)},
	{input: "C0", ptr: new(*uint), error: "rlp: expected input string or byte for uint"},
	{input: "07", ptr: new(*uint), value: uintp(7)},
	{input: "817F", ptr: new(*uint), error: "rlp: non-canonical size information for uint"},
	{input: "8180", ptr: new(*uint), value: uintp(0x80)},
	{input: "C109", ptr: new(*[]uint), value: &[]uint{9}},
	{input: "C58403030303", ptr: new(*[][]byte), value: &[][]byte{{3, 3, 3, 3}}},

	// check that input position is advanced also for empty values.
	{input: "C3808005", ptr: new([]*uint), value: []*uint{uintp(0), uintp(0), uintp(5)}},

	// interface{}
	{input: "00", ptr: new(interface{}), value: []byte{0}},
	{input: "01", ptr: new(interface{}), value: []byte{1}},
	{input: "80", ptr: new(interface{}), value: []byte{}},
	{input: "850505050505", ptr: new(interface{}), value: []byte{5, 5, 5, 5, 5}},
	{input: "C0", ptr: new(interface{}), value: []interface{}{}},
	{input: "C50183040404", ptr: new(interface{}), value: []interface{}{[]byte{1}, []byte{4, 4, 4}}},
	{
		input: "C3010203",
		ptr:   new([]io.Reader),
		error: "rlp: type io.Reader is not RLP-serializable",
	},

	// fuzzer crashes
	{
		input: "c330f9c030f93030ce3030303030303030bd303030303030",
		ptr:   new(interface{}),
		error: "rlp: element is larger than containing list",
	},
}

func uintp(i uint) *uint { return &i }

func runTests(t *testing.T, decode func([]byte, interface{}) error) {
	for i, test := range decodeTests {
		input, err := hex.DecodeString(test.input)
		if err != nil {
			t.Errorf("test %d: invalid hex input %q", i, test.input)
			continue
		}
		err = decode(input, test.ptr)
		if err != nil && test.error == "" {
			t.Errorf("test %d: unexpected Decode error: %v\ndecoding into %T\ninput %q",
				i, err, test.ptr, test.input)
			continue
		}
		if test.error != "" && fmt.Sprint(err) != test.error {
			t.Errorf("test %d: Decode error mismatch\ngot  %v\nwant %v\ndecoding into %T\ninput %q",
				i, err, test.error, test.ptr, test.input)
			continue
		}
		deref := reflect.ValueOf(test.ptr).Elem().Interface()
		if err == nil && !reflect.DeepEqual(deref, test.value) {
			t.Errorf("test %d: value mismatch\ngot  %#v\nwant %#v\ndecoding into %T\ninput %q",
				i, deref, test.value, test.ptr, test.input)
		}
	}
}

func TestDecodeWithByteReader(t *testing.T) {
	runTests(t, func(input []byte, into interface{}) error {
		return Decode(bytes.NewReader(input), into)
	})
}

// plainReader reads from a byte slice but does not
// implement ReadByte. It is also not recognized by the
// size validation. This is useful to test how the decoder
// behaves on a non-buffered input stream.
type plainReader []byte

func newPlainReader(b []byte) io.Reader {
	return (*plainReader)(&b)
}

func (r *plainReader) Read(buf []byte) (n int, err error) {
	if len(*r) == 0 {
		return 0, io.EOF
	}
	n = copy(buf, *r)
	*r = (*r)[n:]
	return n, nil
}

func TestDecodeWithNonByteReader(t *testing.T) {
	runTests(t, func(input []byte, into interface{}) error {
		return Decode(newPlainReader(input), into)
	})
}

func TestDecodeStreamReset(t *testing.T) {
	s := NewStream(nil, 0)
	runTests(t, func(input []byte, into interface{}) error {
		s.Reset(bytes.NewReader(input), 0)
		return s.Decode(into)
	})
}

type testDecoder struct{ called bool }

func (t *testDecoder) DecodeRLP(s *Stream) error {
	if _, err := s.Uint(); err != nil {
		return err
	}
	t.called = true
	return nil
}

func TestDecodeDecoder(t *testing.T) {
	var s struct {
		T1 testDecoder
		T2 *testDecoder
		T3 **testDecoder
	}
	if err := Decode(bytes.NewReader(unhex("C3010203")), &s); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if !s.T1.called {
		t.Errorf("DecodeRLP was not called for (non-pointer) testDecoder")
	}

	if s.T2 == nil {
		t.Errorf("*testDecoder has not been allocated")
	} else if !s.T2.called {
		t.Errorf("DecodeRLP was not called for *testDecoder")
	}

	if s.T3 == nil || *s.T3 == nil {
		t.Errorf("**testDecoder has not been allocated")
	} else if !(*s.T3).called {
		t.Errorf("DecodeRLP was not called for **testDecoder")
	}
}

type byteDecoder byte

func (bd *byteDecoder) DecodeRLP(s *Stream) error {
	_, err := s.Uint()
	*bd = 255
	return err
}

func (bd byteDecoder) called() bool {
	return bd == 255
}

// This test verifies that the byte slice/byte array logic
// does not kick in for element types implementing Decoder.
func TestDecoderInByteSlice(t *testing.T) {
	var slice []byteDecoder
	if err := Decode(bytes.NewReader(unhex("C101")), &slice); err != nil {
		t.Errorf("unexpected Decode error %v", err)
	} else if !slice[0].called() {
		t.Errorf("DecodeRLP not called for slice element")
	}

	var array [1]byteDecoder
	if err := Decode(bytes.NewReader(unhex("C101")), &array); err != nil {
		t.Errorf("unexpected Decode error %v", err)
	} else if !array[0].called() {
		t.Errorf("DecodeRLP not called for array element")
	}
}

func ExampleDecode() {
	input, _ := hex.DecodeString("C90A1486666F6F626172")

	type example struct {
		A, B    uint
		private uint // private fields are ignored
		String  string
	}

	var s example
	err := Decode(bytes.NewReader(input), &s)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Decoded value: %#v\n", s)
	}
	// Output:
	// Decoded value: rlp.example{A:0xa, B:0x14, private:0x0, String:"foobar"}
}

func ExampleDecode_structTagNil() {
	// In this example, we'll use the "nil" struct tag to change
	// how a pointer-typed field is decoded. The input contains an RLP
	// list of one element, an empty string.
	input := []byte{0xC1, 0x80}

	// This type uses the normal rules.
	// The empty input string is decoded as a pointer to an empty Go string.
	var normalRules struct {
		String *string
	}
	Decode(bytes.NewReader(input), &normalRules)
	fmt.Printf("normal: String = %q\n", *normalRules.String)

	// This type uses the struct tag.
	// The empty input string is decoded as a nil pointer.
	var withEmptyOK struct {
		String *string `rlp:"nil"`
	}
	Decode(bytes.NewReader(input), &withEmptyOK)
	fmt.Printf("with nil tag: String = %v\n", withEmptyOK.String)

	// Output:
	// normal: String = ""
	// with nil tag: String = <nil>
}

func ExampleStream() {
	input, _ := hex.DecodeString("C90A1486666F6F626172")
	s := NewStream(bytes.NewReader(input), 0)

	// Check what kind of value lies ahead
	kind, size, _ := s.Kind()
	fmt.Printf("Kind: %v size:%d\n", kind, size)

	// Enter the list
	if _, err := s.List(); err != nil {
		fmt.Printf("List error: %v\n", err)
		return
	}

	// Decode elements
	fmt.Println(s.Uint())
	fmt.Println(s.Uint())
	fmt.Println(s.Bytes())

	// Acknowledge end of list
	if err := s.ListEnd(); err != nil {
		fmt.Printf("ListEnd error: %v\n", err)
	}
	// Output:
	// Kind: 2 size:9
	// 10 <nil>
	// 20 <nil>
	// [102 111 111 98 97 114] <nil>
}

func BenchmarkDecode(b *testing.B) {
	enc := encodeTestSlice(90000)
	b.SetBytes(int64(len(enc)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var s []uint
		r := bytes.NewReader(enc)
		if err := Decode(r, &s); err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func BenchmarkDecodeIntSliceReuse(b *testing.B) {
	enc := encodeTestSlice(100000)
	b.SetBytes(int64(len(enc)))
	b.ReportAllocs()
	b.ResetTimer()

	var s []uint
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(enc)
		if err := Decode(r, &s); err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func encodeTestSlice(n uint) []byte {
	s := make([]uint, n)
	for i := uint(0); i < n; i++ {
		s[i] = i
	}
	b, err := EncodeToBytes(s)
	if err != nil {
		panic(fmt.Sprintf("encode error: %v", err))
	}
	return b
}

/*

 _____ _   _ _______ ______ _____  _   _          _        ______ _    _ _   _  _____ _______ _____ ____  _   _
|_   _| \ | |__   __|  ____|  __ \| \ | |   /\   | |      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
  | | |  \| |  | |  | |__  | |__) |  \| |  /  \  | |      | |__  | |  | |  \| | |       | |    | || |  | |  \| |
  | | | . ` |  | |  |  __| |  _  /| . ` | / /\ \ | |      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
 _| |_| |\  |  | |  | |____| | \ \| |\  |/ ____ \| |____  | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_____|_| \_|  |_|  |______|_|  \_\_| \_/_/    \_\______| |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func comparator(typ reflect.Type, val, result interface{}) bool {
	kind := typ.Kind()
	data := reflect.ValueOf(val).Elem()

	rval := reflect.ValueOf(val).Elem()
	rresult := reflect.ValueOf(result)

	switch {

	case kind == reflect.Array && isByte(typ.Elem()):
		// convert them to slice first, otherwise, cannot compare
		copyVal := reflect.New(rval.Type()).Elem()
		copyResult := reflect.New(rresult.Type()).Elem()

		copyVal.Set(rval)
		copyResult.Set(rresult)

		rval = copyVal
		rresult = copyResult

		valSlice := rval.Slice(0, rval.Len()).Bytes()
		resSlice := rresult.Slice(0, rresult.Len()).Bytes()

		if bytes.Compare(valSlice, resSlice) != 0 {
			return false
		}
		return true

	case kind == reflect.Slice && isByte(typ.Elem()):
		if bytes.Compare(data.Bytes(), reflect.ValueOf(result).Bytes()) != 0 {
			return false
		}
		return true

	case kind == reflect.Slice || kind == reflect.Array:
		if rval.Len() != rresult.Len() {
			return false
		}
		for i := 0; i < rval.Len(); i++ {
			if rval.Index(i).String() != rresult.Index(i).String() {
				return false
			}
		}

		return true

	case kind == reflect.Bool:
		if data.Bool() == result {
			return true
		}
		return false

	case kind == reflect.String:
		if data.String() == result {
			return true
		}
		return false

	default:
		return false
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
