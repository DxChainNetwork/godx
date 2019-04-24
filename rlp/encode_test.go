/*
	Public Functions:
		Encode -- Not test explicitly, tested along with EncodeToBytes
               -- Will be "tested" with an example as well
		EncodeToBytes
		EncodeToReader
*/

package rlp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type uniqueStruct1 struct {
	name string
	age  int
}

type uinqueStruct2 struct {
	Name  string
	Hobby []string
	age   int
}

// custom encoding rules, allowing to encode private structure fields
func (u *uniqueStruct1) EncodeRLP(w io.Writer) error {
	err := Encode(w, u.name)
	if err != nil {
		return err
	}
	return nil
}

var rawValueData RawValue = []byte{0x83, 0x66, 0x7F, 0x47}
var encoderValueData = uniqueStruct1{"ffaa", 25}
var bigIntData = big.NewInt(30)
var uintTypeData uint = 30
var trueBoolData = true
var falseBoolData = false
var stringTypeData = "test"
var byteSliceTypeData = []byte{'a', 'b', 'c'}
var byteArrayTypeData = [3]byte{'a', 'b', 'c'}
var sliceTypeData = []string{"test", "test"}
var arrayTypeData = [2]string{"test1", "test2"}
var structureTypeData = uinqueStruct2{"mzhang", []string{"running"}, 24}
var ptrTypeData = &uintTypeData

var results = map[string][]byte{
	"rawValueData":      {0x83, 0x66, 0x7F, 0x47},
	"encoderValueData":  {0x84, 0x66, 0x66, 0x61, 0x61},
	"bigIntData":        {0x1E},
	"uintTypeData":      {0x1E},
	"trueBoolData":      {0x01},
	"falseBoolData":     {0x80},
	"byteSliceTypeData": {0x83, 0x61, 0x62, 0x63},
	"stringTypeData":    {0x84, 0x74, 0x65, 0x73, 0x74},
	"sliceTypeData":     {0xCA, 0x84, 0x74, 0x65, 0x73, 0x74, 0x84, 0x74, 0x65, 0x73, 0x74},
	"arrayTypeData":     {0xCC, 0x85, 0x74, 0x65, 0x73, 0x74, 0x31, 0x85, 0x74, 0x65, 0x73, 0x74, 0x32},
	"structureTypeData": {0xD0, 0x86, 0x6D, 0x7A, 0x68, 0x61, 0x6E, 0x67, 0xC8, 0x87, 0x72, 0x75, 0x6E, 0x6E, 0x69, 0x6E, 0x67},
}

func TestEncodeToBytesWithRlpSupportedData(t *testing.T) {

	tables := []struct {
		valueToEncode interface{}
		encodedResult []byte
	}{
		{rawValueData, results["rawValueData"]},
		{&encoderValueData, results["encoderValueData"]},
		{bigIntData, results["bigIntData"]},
		{*bigIntData, results["bigIntData"]},
		{uintTypeData, results["uintTypeData"]},
		{trueBoolData, results["trueBoolData"]},
		{falseBoolData, results["falseBoolData"]},
		{stringTypeData, results["stringTypeData"]},
		{byteSliceTypeData, results["byteSliceTypeData"]},
		{byteArrayTypeData, results["byteSliceTypeData"]},
		{sliceTypeData, results["sliceTypeData"]},
		{arrayTypeData, results["arrayTypeData"]},
		{structureTypeData, results["structureTypeData"]},
		{ptrTypeData, results["uintTypeData"]},
	}

	for _, table := range tables {
		returnResult, err := EncodeToBytes(table.valueToEncode)
		if err != nil {
			t.Errorf("ERROR: %s", err)
		} else if bytes.Compare(returnResult, table.encodedResult) != 0 {
			t.Errorf("input %x, got %x expected %x",
				table.valueToEncode, returnResult, table.encodedResult)
		}
	}
}

func TestEncodeToReaderWithRawValueData(t *testing.T) {
	size, reader, err := EncodeToReader(rawValueData)
	if err != nil {
		t.Errorf("%s", err)
	} else if size != 4 {
		t.Errorf("input %x, got size %d, expected size %d", rawValueData, size, 4)
	}
	byteContainer := make([]byte, 4)
	_, err = reader.Read(byteContainer)
	if err != io.EOF {
		t.Errorf("%s", err)
	} else if bytes.Compare(byteContainer, results["rawValueData"]) != 0 {
		t.Errorf("input %x, got %x, expected %x", rawValueData, byteContainer, rawValueData)
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

type byteEncoderTest byte

func (b byteEncoderTest) EncodeRLP(w io.Writer) error {
	fmt.Println("bytEncoder implemented Encoder Interface")
	return nil
}

func TestIsByteWithByteData(t *testing.T) {

	tables := []struct {
		x byte
		y bool
	}{
		{'f', true},
		{'t', true},
		{'z', true},
		{'D', true},
		{'H', true},
		{'G', true},
		{'%', true},
		{'*', true},
		{'~', true},
		{0x80, true},
		{0x90, true},
		{0x30, true},
		{5, true},
		{32, true},
		{255, true},
	}

	for _, table := range tables {
		rtype := reflect.TypeOf(table.x)
		isByteTestResult := isByte(rtype)
		if isByteTestResult != table.y {
			t.Errorf("%c is type of %s, got %t, want %t", table.x, rtype, table.y, isByteTestResult)
		}
	}
}

func TestIsByteWithNonByteData(t *testing.T) {
	// byte type data that implements encoder interface
	var test byteEncoderTest = 'f'

	tables := []struct {
		x interface{}
		y bool
	}{
		{"love", false},
		{12.5, false},
		{256, false},
		{[]byte{'g'}, false},
		{[]string{"good", "bad"}, false},
		{true, false},
		{0x8091, false},
		{0x9034, false},
		{0x3083, false},
		{test, false},
	}

	for _, table := range tables {
		rtype := reflect.TypeOf(table.x)
		isByteTestResult := isByte(rtype)
		if isByteTestResult != table.y {
			t.Errorf("%c is type of %s, got %t, want %t",
				table.x, rtype, table.y, isByteTestResult)
		}
	}
}

func TestPutInt(t *testing.T) {
	byteBuffer := make([]byte, 8)
	tables := []struct {
		b      []byte
		i      uint64
		size   int
		result []byte
	}{
		{byteBuffer, 10, 1, []byte{10, 0, 0, 0, 0, 0, 0, 0}},
		{byteBuffer, 1024, 2, []byte{4, 0, 0, 0, 0, 0, 0, 0}},
		{byteBuffer, 65538, 3, []byte{1, 0, 2, 0, 0, 0, 0, 0}},
		{byteBuffer, 16779216, 4, []byte{1, 0, 7, 208, 0, 0, 0, 0}},
		{byteBuffer, 4294997296, 5, []byte{1, 0, 0, 117, 48, 0, 0, 0}},
		{byteBuffer, 1099511927776, 6, []byte{1, 0, 0, 4, 147, 224, 0, 0}},
		{byteBuffer, 281474976715656, 7, []byte{1, 0, 0, 0, 0, 19, 136, 0}},
		{byteBuffer, 18446744073709551615, 8, []byte{255, 255, 255, 255, 255, 255, 255, 255}},
	}

	for _, table := range tables {
		putintResult := putint(table.b, table.i)
		if putintResult != table.size {
			t.Errorf("%d can be represented with %d bytes, got %d, want %d", table.i, table.size, putintResult, table.size)
		}
		if bytes.Compare(byteBuffer, table.result) != 0 {
			t.Errorf("%d stored in byteBuffer, got %d, want %d", table.b, table.b, table.result)
		}
	}
}

var stringHeaderTest = new(encbuf)

func TestEncodeStringHeaderWithDifferentHeaderSize(t *testing.T) {
	stringHeaderTest.sizebuf = make([]byte, 9)

	tables := []struct {
		size   int
		result []byte
	}{
		{1, []byte{0x81}},
		{10, []byte{0x8A}},
		{15, []byte{0x8F}},
		{55, []byte{0xB7}},
		{64, []byte{0xB8, 0x40}},
		{16779216, []byte{0xBB, 0x01, 0x00, 0x07, 0xD0}},
	}

	for _, table := range tables {
		stringHeaderTest.encodeStringHeader(table.size)
		if bytes.Compare(stringHeaderTest.str, table.result) != 0 {
			t.Errorf("size is %d, got %x, want %x",
				table.size, stringHeaderTest.str, table.result)
		}
		stringHeaderTest.str = stringHeaderTest.str[:0]
	}
}

var stringTest = new(encbuf)

func TestEncodeStringWithByteSlice(t *testing.T) {
	stringTest.sizebuf = make([]byte, 9)

	tables := []struct {
		b      []byte
		result []byte
	}{
		// RULE 1
		{[]byte{'a'}, []byte{0x61}},
		// RULE 2
		{[]byte{'f', 'f', 'f', 'f'}, []byte{0x84, 0x66, 0x66, 0x66, 0x66}},
		// RULE 3
		{[]byte{'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f'},
			[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		stringTest.encodeString(table.b)
		if bytes.Compare(stringTest.str, table.result) != 0 {
			t.Errorf("input: %x, got %x, want %x", table.b, stringTest.str, table.result)
		}
		stringTest.str = stringTest.str[:0]
	}
}

func TestIntSizePositiveWithNumbersCanBeRepresentInDifferentByte(t *testing.T) {
	tables := []struct {
		i    uint64
		size int
	}{
		{10, 1},
		{1024, 2},
		{65538, 3},
		{16779216, 4},
		{4294997296, 5},
		{1099511927776, 6},
		{281474976715656, 7},
		{18446744073709551615, 8},
	}

	for _, table := range tables {
		result := intsize(table.i)
		if result != table.size {
			t.Errorf("input: %d, got %x, want %x", table.i, result, table.size)
		}
	}
}

var sizeTest = new(encbuf)

func TestSizeWithEmptyAndFullByteSlice(t *testing.T) {
	tables := []struct {
		b      []byte
		lhsize int
		size   int
	}{
		{[]byte{'f', 'f', 'f'}, 5, 8},
		{[]byte{}, 5, 5},
	}

	for _, table := range tables {
		sizeTest.str = table.b
		sizeTest.lhsize = table.lhsize
		result := sizeTest.size()
		if table.size != result {
			t.Errorf("input str %x, lhsize %d, got %d, want %d",
				sizeTest.str, sizeTest.lhsize, result, table.size)
		}
	}
}

var rawValueTest = new(encbuf)

func TestWriteRawValueWithRlpEncodedByteAndStringAndList(t *testing.T) {
	// RawValue contains pre-encoded data
	tables := []struct {
		r      RawValue
		result []byte
	}{
		{[]byte{0x80}, []byte{0x80}},
		{[]byte{0x84, 0x66, 0x66, 0x66, 0x66}, []byte{0x84, 0x66, 0x66, 0x66, 0x66}},
		{[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66},
			[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		err := writeRawValue(reflect.ValueOf(table.r), rawValueTest)
		if err != nil {
			t.Errorf("%s", err)
		}
		if bytes.Compare(table.result, rawValueTest.str) != 0 {
			t.Errorf("input %x, got %x, want %x", table.r, rawValueTest.str, table.result)
		}
		rawValueTest.str = rawValueTest.str[:0]
	}
}

var EncoderTest = new(encbuf)

type dataEncoder int

func (d *dataEncoder) EncodeRLP(w io.Writer) error {
	*d = 10
	return nil
}

func TestWriteEncoder(t *testing.T) {
	var testVal1 dataEncoder = 1

	tables := []struct {
		val interface{}
	}{
		{&testVal1},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.val)

		err := writeEncoder(rval, EncoderTest)
		if err != nil {
			t.Errorf("%s", err)
		}
		if testVal1 != 10 {
			t.Errorf("got %d, want 10", testVal1)
		}
	}
}

func TestWriteNonEncoder(t *testing.T) {
	var testVal1 dataEncoder = 1

	tables := []struct {
		val interface{}
	}{
		{&testVal1},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.val)
		// rval.Elem is addressable
		err := writeEncoderNoPtr(rval.Elem(), EncoderTest)
		if err != nil {
			t.Errorf("%s", err)
		} else if testVal1 != 10 {
			t.Errorf("got %d, want 10", testVal1)
		}
	}
}

func TestPutHeadWithDifferentSizeListHead(t *testing.T) {
	buf := make([]byte, 9)

	tables := []struct {
		smalltag     byte
		largetag     byte
		size         uint64
		resultReturn int
		listHeaders  []byte
	}{
		{0xC0, 0xF7, 1, 1, []byte{0xC1, 0, 0, 0, 0, 0, 0, 0, 0}},
		{0xC0, 0xF7, 55, 1, []byte{0xF7, 0, 0, 0, 0, 0, 0, 0, 0}},
		{0xC0, 0xF7, 56, 2, []byte{0xF8, 0x38, 0, 0, 0, 0, 0, 0, 0}},
	}

	for _, table := range tables {
		result := puthead(buf, table.smalltag, table.largetag, table.size)
		if result != table.resultReturn {
			t.Errorf("input size: %d, got size %d, want size %d",
				table.size, result, table.resultReturn)
		}
		if bytes.Compare(buf, table.listHeaders) != 0 {
			t.Errorf("input size: %d, got listheader %x, want listheader %d",
				table.size, buf, table.listHeaders)
		}
	}
}

type Person struct {
	name string
	age  uint
}

var raw RawValue = []byte{0x82, 0x66, 0x50}
var encoData dataEncoder = 10
var bigData = big.NewInt(20)
var uintData uint64 = 50
var boolData = true
var stringData = "ffff"
var byteSliceData = []byte{'f', 0x80, 'd'}
var byteArrayData = [2]byte{'f', 0x80}
var arrayData = [2]string{"lalal", "aaaa"}
var sliceData = []string{"blabla", "omgomg"}
var structData = Person{"mzhang", 24}
var strctPtrData = &structData
var arrayPtrData = &arrayData
var byteArrayPtrData = &byteArrayData
var defaultTags = tags{false, false, false}

func TestMakeWriterWithRlpSupportedDatType(t *testing.T) {
	tables := []struct {
		val            interface{}
		ts             tags
		resultFunction interface{}
	}{
		{raw, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeRawValue"},
		{&encoData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeEncoder"},
		{encoData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeEncoderNoPtr"},
		{bigData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeBigIntPtr"},
		{*bigData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeBigIntNoPtr"},
		{uintData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeUint"},
		{boolData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeBool"},
		{stringData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeString"},
		{byteSliceData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeBytes"},
		{byteArrayData, defaultTags, "github.com/DxChainNetwork/godx/rlp.writeByteArray"},
		{arrayData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makeSliceWriter.func1"},
		{sliceData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makeSliceWriter.func1"},
		{structData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makeStructWriter.func1"},
		{strctPtrData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makePtrWriter.func4"},
		{arrayPtrData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makePtrWriter.func4"},
		{byteArrayPtrData, defaultTags, "github.com/DxChainNetwork/godx/rlp.makePtrWriter.func4"},
	}

	for _, table := range tables {
		rtype := reflect.TypeOf(table.val)
		writer, err := makeWriter(rtype, table.ts)
		funcName := getFunctionName(writer)

		if err != nil {
			t.Errorf("%s", err)
		} else if funcName != table.resultFunction {
			t.Errorf("input type: %s, input kind: %s function got %s, function want %s",
				rtype, rtype.Kind(), funcName, table.resultFunction)
		}
	}
}

var resetEncbuf = new(encbuf)

func TestResetWithEmptyAndFullByteSlice(t *testing.T) {
	tables := []struct {
		lhsize int
		str    []byte
		lheads []*listhead
	}{
		{10, []byte{0x82, 0x66, 0x67}, []*listhead{{1, 2}, {3, 4}}},
		{100, []byte{}, []*listhead{}},
	}

	for _, table := range tables {
		resetEncbuf.lhsize = table.lhsize
		resetEncbuf.str = table.str
		resetEncbuf.lheads = table.lheads
		resetEncbuf.reset()
		if resetEncbuf.lhsize != 0 {
			t.Errorf("lhsize: got %d, want 0", resetEncbuf.lhsize)
		}
		if len(resetEncbuf.str) != 0 {
			t.Errorf("str: got %x, want []", resetEncbuf.str)
		}
		if len(resetEncbuf.lheads) != 0 {
			t.Errorf("lheads: got %x, want []", resetEncbuf.lheads)
		}
	}
}

var bigIntEncbuf = new(encbuf)
var bg0 = big.NewInt(0)
var bg1 = big.NewInt(50)
var bg2 = big.NewInt(1024)
var bg3 = big.NewInt(65538)
var bg4 = big.NewInt(16779216)
var bg5 = big.NewInt(4294997296)
var bg6 = big.NewInt(1099511927776)
var bg7 = big.NewInt(281474976715656)

func TestWriteBigIntWithDifferentSizeBigInt(t *testing.T) {
	bigIntEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		i      *big.Int
		result []byte
	}{
		{bg0, []byte{0x80}},
		{bg1, []byte{0x32}},
		{bg2, []byte{0x82, 0x04, 0x00}},
		{bg3, []byte{0x83, 0x01, 0x00, 0x02}},
		{bg4, []byte{0x84, 0x01, 0x00, 0x07, 0xD0}},
		{bg5, []byte{0x85, 0x01, 0x00, 0x00, 0x75, 0x30}},
		{bg6, []byte{0x86, 0x01, 0x00, 0x00, 0x04, 0x93, 0xE0}},
		{bg7, []byte{0x87, 0x01, 0x00, 0x00, 0x00, 0x00, 0x13, 0x88}},
	}

	for _, table := range tables {
		err := writeBigInt(table.i, bigIntEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(bigIntEncbuf.str, table.result) != 0 {
			t.Errorf("input %d, got %x, want %x", table.i, bigIntEncbuf.str, table.result)
		}
		bigIntEncbuf.reset()
	}
}

var bigIntPtrNil *big.Int

func TestWriteBigIntPtrWithNilAndNonNilData(t *testing.T) {
	bigIntEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		i      *big.Int
		result []byte
	}{
		{bigIntPtrNil, []byte{0x80}},
		{bg4, []byte{0x84, 0x01, 0x00, 0x07, 0xD0}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.i)
		err := writeBigIntPtr(rval, bigIntEncbuf)

		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(bigIntEncbuf.str, table.result) != 0 {
			t.Errorf("input %s, got %x, want %x", rval, bigIntEncbuf.str, table.result)
		}
		bigIntEncbuf.reset()
	}
}

func TestWriteBigIntNoPtr(t *testing.T) {
	bigIntEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		i      big.Int
		result []byte
	}{
		{*bg0, []byte{0x80}},
		{*bg1, []byte{0x32}},
		{*bg2, []byte{0x82, 0x04, 0x00}},
		{*bg3, []byte{0x83, 0x01, 0x00, 0x02}},
		{*bg4, []byte{0x84, 0x01, 0x00, 0x07, 0xD0}},
		{*bg5, []byte{0x85, 0x01, 0x00, 0x00, 0x75, 0x30}},
		{*bg6, []byte{0x86, 0x01, 0x00, 0x00, 0x04, 0x93, 0xE0}},
		{*bg7, []byte{0x87, 0x01, 0x00, 0x00, 0x00, 0x00, 0x13, 0x88}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.i)
		err := writeBigIntNoPtr(rval, bigIntEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(bigIntEncbuf.str, table.result) != 0 {
			t.Errorf("input %s, got %x, want %x", rval, bigIntEncbuf.str, table.result)
		}
		bigIntEncbuf.reset()
	}
}

var uintEncbuf = new(encbuf)

var u0 uint = 0
var u1 uint8 = 10
var u2 uint16 = 65535
var u3 uint32 = 4294967295
var u4 uint64 = 18446744073709551615
var u5 uint = 18446744073709551515
var u6 uint = 129

func TestWriteUintWithDifferentSizeUintTypeData(t *testing.T) {
	uintEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		i      interface{}
		result []byte
	}{
		{u0, []byte{0x80}},
		{u1, []byte{0x0A}},
		{u2, []byte{0x82, 0xFF, 0xFF}},
		{u3, []byte{0x84, 0xFF, 0xFF, 0xFF, 0xFF}},
		{u4, []byte{0x88, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{u5, []byte{0x88, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x9B}},
		{u6, []byte{0x81, 0x81}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.i)
		err := writeUint(rval, uintEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(uintEncbuf.str, table.result) != 0 {
			t.Errorf("input %d, got %x, want %x", table.i, uintEncbuf.str, table.result)
		}
		uintEncbuf.reset()
	}
}

var boolEncbuf = new(encbuf)

func TestWriteBoolWithTrueAndFalse(t *testing.T) {
	boolEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		i      bool
		result []byte
	}{
		{true, []byte{0x01}},
		{false, []byte{0x80}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.i)
		err := writeBool(rval, boolEncbuf)
		if err != nil {
			t.Errorf("input %t, got %x, want %x", table.i, boolEncbuf.str, table.result)
		}
		boolEncbuf.reset()
	}
}

var testEncbuf = new(encbuf)

func TestWriteStringWithEmptyStringSingleByteShortAndLongString(t *testing.T) {
	testEncbuf.sizebuf = make([]byte, 9)

	tables := []struct {
		s      string
		result []byte
	}{
		{"", []byte{0x80}},
		{"f", []byte{0x66}},
		{"ff", []byte{0x82, 0x66, 0x66}},
		{"ffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.s)
		err := writeString(rval, testEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.result, testEncbuf.str) != 0 {
			t.Errorf("input %s, got %x want %x", table.s, testEncbuf.str, table.result)
		}
		testEncbuf.reset()

	}
}

func TestWriteByteArrayWithEmptyArraySingleByteShortAndLongArray(t *testing.T) {
	tables := []struct {
		ba     interface{}
		result []byte
	}{
		{[0]byte{}, []byte{0x80}},
		{[1]byte{'f'}, []byte{0x66}},
		{[2]byte{'f', 'f'}, []byte{0x82, 0x66, 0x66}},
		{[56]byte{'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f'},
			[]byte{0xB8, 0x38, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
				0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.ba)
		err := writeByteArray(rval, testEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.result, testEncbuf.str) != 0 {
			t.Errorf("input %s, got %x want %x", table.ba, testEncbuf.str, table.result)
		}
		testEncbuf.reset()

	}
}

var emptySlice = []string{}
var emptyArray = [0]string{}
var testSlice = []string{"fff", "fff"}
var testArray = [3]string{"ff", "f", "fff"}
var testSlice2 = []string{"ffffffffffffffffffff", "ffffffffffffffffffff", "ffffffffff", "fffff", "fffff"}

func TestMakeSliceWriterWithEmptySliceAndArrayFullSliceAndArray(t *testing.T) {
	tables := []struct {
		l        interface{}
		ts       tags
		result   []byte
		headsize int
	}{
		{emptySlice, defaultTags, []byte{}, 1},
		{emptyArray, defaultTags, []byte{}, 1},
		{testSlice, defaultTags, []byte{0x83, 0x66, 0x66, 0x66, 0x83, 0x66, 0x66, 0x66}, 1},
		{testArray, defaultTags, []byte{0x82, 0x66, 0x66, 0x66, 0x83, 0x66, 0x66, 0x66}, 1},
		{testSlice2, defaultTags, []byte{0x94, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x94, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x8A, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66,
			0x85, 0x66, 0x66, 0x66, 0x66, 0x66, 0x85, 0x66, 0x66, 0x66, 0x66, 0x66}, 2},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.l)
		rtype := rval.Type()
		writer, err := makeSliceWriter(rtype, table.ts)
		if err != nil {
			t.Errorf("%s", err)
		}
		err = writer(rval, testEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.result, testEncbuf.str) != 0 {
			t.Errorf("input %s, got %x, want %x", table.l, testEncbuf.str, table.result)
		} else if testEncbuf.lhsize != table.headsize {
			t.Errorf("input %s, got headsize %d, want %d",
				table.l, testEncbuf.lhsize, table.headsize)
		}

		testEncbuf.reset()
	}
}

type Person1 struct {
	Name  string
	Age   uint
	Hobby []string `rlp:"tail"`
}

type Person2 struct {
	Name  string `rlp:"-"`
	Age   uint
	Hobby []string
}

var p1 = Person1{"mzhang", 24, []string{"fff", "fff"}}
var p2 = Person2{"dontcareignored", 24, []string{"fff", "fff"}}

func TestMakeStructWriterWithStructureWithDifferentTags(t *testing.T) {
	tables := []struct {
		p        interface{}
		headsize int
		result   []byte
	}{
		{p1, 1, []byte{0x86, 0x6D, 0x7A, 0x68, 0x61, 0x6E, 0x67, 0x18, 0x83,
			0x66, 0x66, 0x66, 0x83, 0x66, 0x66, 0x66}},
		{p2, 2, []byte{0x18, 0x83, 0x66, 0x66, 0x66, 0x83, 0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.p)
		rtype := rval.Type()

		writer, err := makeStructWriter(rtype)
		if err != nil {
			t.Errorf("%s", err)
		}
		err = writer(rval, testEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(table.result, testEncbuf.str) != 0 {
			t.Errorf("input %v, got %x, want %x", table.p, testEncbuf.str, table.result)
		} else if testEncbuf.lhsize != table.headsize {
			t.Errorf("input %v, size got %d, want %d", table.p, testEncbuf.lhsize, table.headsize)
		}

		testEncbuf.reset()
	}
}

var sptrNil *string
var baptrNil *[3]byte
var saptrNil *[3]string
var stest = "fff"
var sptr = &stest

func TestMakePtrWriterWithDifferentTypePointer(t *testing.T) {
	tables := []struct {
		ptrTyp interface{}
		result []byte
	}{
		{sptrNil, []byte{0x80}},
		{baptrNil, []byte{0x80}},
		{saptrNil, []byte{}},
		{sptr, []byte{0x83, 0x66, 0x66, 0x66}},
	}

	for _, table := range tables {
		rval := reflect.ValueOf(table.ptrTyp)
		rtype := rval.Type()
		writer, err := makePtrWriter(rtype)
		if err != nil {
			t.Errorf("%s", err)
		}
		err = writer(rval, testEncbuf)
		if err != nil {
			t.Errorf("%s", err)
		} else if bytes.Compare(testEncbuf.str, table.result) != 0 {
			t.Errorf("input: %v, got %x want %x", table.ptrTyp, testEncbuf.str, table.result)
		}

		testEncbuf.reset()
	}
}

var lh = new(listhead)

func TestListEndWithDifferentListLength(t *testing.T) {
	tables := []struct {
		str      []byte
		wlhsize  int
		lhoffset int
		lhsize   int
		result   int
	}{
		{[]byte{'f', 'f'}, 1, 1, 1, 2},
		{[]byte{'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f',
			'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f', 'f'}, 0, 0, 0, 2},
	}

	for _, table := range tables {
		testEncbuf.str = table.str
		testEncbuf.lhsize = table.wlhsize
		lh.offset = table.lhoffset
		lh.size = table.lhsize

		testEncbuf.listEnd(lh)
		if testEncbuf.lhsize != table.result {
			t.Errorf("input str %x, wlhsize %d, lhoffset %d lhsize %d, got %d, want %d",
				table.str, table.wlhsize, table.lhoffset, table.lhsize, testEncbuf.lhsize, table.result)
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

type testEncoder struct {
	err error
}

func (e *testEncoder) EncodeRLP(w io.Writer) error {
	if e == nil {
		w.Write([]byte{0, 0, 0, 0})
	} else if e.err != nil {
		return e.err
	} else {
		w.Write([]byte{0, 1, 0, 1, 0, 1, 0, 1, 0, 1})
	}
	return nil
}

type byteEncoder byte

func (e byteEncoder) EncodeRLP(w io.Writer) error {
	w.Write(EmptyList)
	return nil
}

type encodableReader struct {
	A, B uint
}

func (e *encodableReader) Read(b []byte) (int, error) {
	panic("called")
}

type namedByteType byte

var (
	_ = Encoder(&testEncoder{})
	_ = Encoder(byteEncoder(0))

	reader io.Reader = &encodableReader{1, 2}
)

type encTest struct {
	val           interface{}
	output, error string
}

var encTests = []encTest{
	// booleans
	{val: true, output: "01"},
	{val: false, output: "80"},

	// integers
	{val: uint32(0), output: "80"},
	{val: uint32(127), output: "7F"},
	{val: uint32(128), output: "8180"},
	{val: uint32(256), output: "820100"},
	{val: uint32(1024), output: "820400"},
	{val: uint32(0xFFFFFF), output: "83FFFFFF"},
	{val: uint32(0xFFFFFFFF), output: "84FFFFFFFF"},
	{val: uint64(0xFFFFFFFF), output: "84FFFFFFFF"},
	{val: uint64(0xFFFFFFFFFF), output: "85FFFFFFFFFF"},
	{val: uint64(0xFFFFFFFFFFFF), output: "86FFFFFFFFFFFF"},
	{val: uint64(0xFFFFFFFFFFFFFF), output: "87FFFFFFFFFFFFFF"},
	{val: uint64(0xFFFFFFFFFFFFFFFF), output: "88FFFFFFFFFFFFFFFF"},

	// big integers (should match uint for small values)
	{val: big.NewInt(0), output: "80"},
	{val: big.NewInt(1), output: "01"},
	{val: big.NewInt(127), output: "7F"},
	{val: big.NewInt(128), output: "8180"},
	{val: big.NewInt(256), output: "820100"},
	{val: big.NewInt(1024), output: "820400"},
	{val: big.NewInt(0xFFFFFF), output: "83FFFFFF"},
	{val: big.NewInt(0xFFFFFFFF), output: "84FFFFFFFF"},
	{val: big.NewInt(0xFFFFFFFFFF), output: "85FFFFFFFFFF"},
	{val: big.NewInt(0xFFFFFFFFFFFF), output: "86FFFFFFFFFFFF"},
	{val: big.NewInt(0xFFFFFFFFFFFFFF), output: "87FFFFFFFFFFFFFF"},
	{
		val:    big.NewInt(0).SetBytes(unhex("102030405060708090A0B0C0D0E0F2")),
		output: "8F102030405060708090A0B0C0D0E0F2",
	},
	{
		val:    big.NewInt(0).SetBytes(unhex("0100020003000400050006000700080009000A000B000C000D000E01")),
		output: "9C0100020003000400050006000700080009000A000B000C000D000E01",
	},
	{
		val:    big.NewInt(0).SetBytes(unhex("010000000000000000000000000000000000000000000000000000000000000000")),
		output: "A1010000000000000000000000000000000000000000000000000000000000000000",
	},

	// non-pointer big.Int
	{val: *big.NewInt(0), output: "80"},
	{val: *big.NewInt(0xFFFFFF), output: "83FFFFFF"},

	// negative ints are not supported
	{val: big.NewInt(-1), error: "rlp: cannot encode negative *big.Int"},

	// byte slices, strings
	{val: []byte{}, output: "80"},
	{val: []byte{0x7E}, output: "7E"},
	{val: []byte{0x7F}, output: "7F"},
	{val: []byte{0x80}, output: "8180"},
	{val: []byte{1, 2, 3}, output: "83010203"},

	{val: []namedByteType{1, 2, 3}, output: "83010203"},
	{val: [...]namedByteType{1, 2, 3}, output: "83010203"},

	{val: "", output: "80"},
	{val: "\x7E", output: "7E"},
	{val: "\x7F", output: "7F"},
	{val: "\x80", output: "8180"},
	{val: "dog", output: "83646F67"},
	{
		val:    "Lorem ipsum dolor sit amet, consectetur adipisicing eli",
		output: "B74C6F72656D20697073756D20646F6C6F722073697420616D65742C20636F6E7365637465747572206164697069736963696E6720656C69",
	},
	{
		val:    "Lorem ipsum dolor sit amet, consectetur adipisicing elit",
		output: "B8384C6F72656D20697073756D20646F6C6F722073697420616D65742C20636F6E7365637465747572206164697069736963696E6720656C6974",
	},
	{
		val:    "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur mauris magna, suscipit sed vehicula non, iaculis faucibus tortor. Proin suscipit ultricies malesuada. Duis tortor elit, dictum quis tristique eu, ultrices at risus. Morbi a est imperdiet mi ullamcorper aliquet suscipit nec lorem. Aenean quis leo mollis, vulputate elit varius, consequat enim. Nulla ultrices turpis justo, et posuere urna consectetur nec. Proin non convallis metus. Donec tempor ipsum in mauris congue sollicitudin. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Suspendisse convallis sem vel massa faucibus, eget lacinia lacus tempor. Nulla quis ultricies purus. Proin auctor rhoncus nibh condimentum mollis. Aliquam consequat enim at metus luctus, a eleifend purus egestas. Curabitur at nibh metus. Nam bibendum, neque at auctor tristique, lorem libero aliquet arcu, non interdum tellus lectus sit amet eros. Cras rhoncus, metus ac ornare cursus, dolor justo ultrices metus, at ullamcorper volutpat",
		output: "B904004C6F72656D20697073756D20646F6C6F722073697420616D65742C20636F6E73656374657475722061646970697363696E6720656C69742E20437572616269747572206D6175726973206D61676E612C20737573636970697420736564207665686963756C61206E6F6E2C20696163756C697320666175636962757320746F72746F722E2050726F696E20737573636970697420756C74726963696573206D616C6573756164612E204475697320746F72746F7220656C69742C2064696374756D2071756973207472697374697175652065752C20756C7472696365732061742072697375732E204D6F72626920612065737420696D70657264696574206D6920756C6C616D636F7270657220616C6971756574207375736369706974206E6563206C6F72656D2E2041656E65616E2071756973206C656F206D6F6C6C69732C2076756C70757461746520656C6974207661726975732C20636F6E73657175617420656E696D2E204E756C6C6120756C74726963657320747572706973206A7573746F2C20657420706F73756572652075726E6120636F6E7365637465747572206E65632E2050726F696E206E6F6E20636F6E76616C6C6973206D657475732E20446F6E65632074656D706F7220697073756D20696E206D617572697320636F6E67756520736F6C6C696369747564696E2E20566573746962756C756D20616E746520697073756D207072696D697320696E206661756369627573206F726369206C756374757320657420756C74726963657320706F737565726520637562696C69612043757261653B2053757370656E646973736520636F6E76616C6C69732073656D2076656C206D617373612066617563696275732C2065676574206C6163696E6961206C616375732074656D706F722E204E756C6C61207175697320756C747269636965732070757275732E2050726F696E20617563746F722072686F6E637573206E69626820636F6E64696D656E74756D206D6F6C6C69732E20416C697175616D20636F6E73657175617420656E696D206174206D65747573206C75637475732C206120656C656966656E6420707572757320656765737461732E20437572616269747572206174206E696268206D657475732E204E616D20626962656E64756D2C206E6571756520617420617563746F72207472697374697175652C206C6F72656D206C696265726F20616C697175657420617263752C206E6F6E20696E74657264756D2074656C6C7573206C65637475732073697420616D65742065726F732E20437261732072686F6E6375732C206D65747573206163206F726E617265206375727375732C20646F6C6F72206A7573746F20756C747269636573206D657475732C20617420756C6C616D636F7270657220766F6C7574706174",
	},

	// slices
	{val: []uint{}, output: "C0"},
	{val: []uint{1, 2, 3}, output: "C3010203"},
	{
		// [ [], [[]], [ [], [[]] ] ]
		val:    []interface{}{[]interface{}{}, [][]interface{}{{}}, []interface{}{[]interface{}{}, [][]interface{}{{}}}},
		output: "C7C0C1C0C3C0C1C0",
	},
	{
		val:    []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh", "iii", "jjj", "kkk", "lll", "mmm", "nnn", "ooo"},
		output: "F83C836161618362626283636363836464648365656583666666836767678368686883696969836A6A6A836B6B6B836C6C6C836D6D6D836E6E6E836F6F6F",
	},
	{
		val:    []interface{}{uint(1), uint(0xFFFFFF), []interface{}{[]uint{4, 5, 5}}, "abc"},
		output: "CE0183FFFFFFC4C304050583616263",
	},
	{
		val: [][]string{
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
			{"asdf", "qwer", "zxcv"},
		},
		output: "F90200CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376CF84617364668471776572847A786376",
	},

	// RawValue
	{val: RawValue(unhex("01")), output: "01"},
	{val: RawValue(unhex("82FFFF")), output: "82FFFF"},
	{val: []RawValue{unhex("01"), unhex("02")}, output: "C20102"},

	// structs
	{val: simplestruct{}, output: "C28080"},
	{val: simplestruct{A: 3, B: "foo"}, output: "C50383666F6F"},
	{val: &recstruct{5, nil}, output: "C205C0"},
	{val: &recstruct{5, &recstruct{4, &recstruct{3, nil}}}, output: "C605C404C203C0"},
	{val: &tailRaw{A: 1, Tail: []RawValue{unhex("02"), unhex("03")}}, output: "C3010203"},
	{val: &tailRaw{A: 1, Tail: []RawValue{unhex("02")}}, output: "C20102"},
	{val: &tailRaw{A: 1, Tail: []RawValue{}}, output: "C101"},
	{val: &tailRaw{A: 1, Tail: nil}, output: "C101"},
	{val: &hasIgnoredField{A: 1, B: 2, C: 3}, output: "C20103"},

	// nil
	{val: (*uint)(nil), output: "80"},
	{val: (*string)(nil), output: "80"},
	{val: (*[]byte)(nil), output: "80"},
	{val: (*[10]byte)(nil), output: "80"},
	{val: (*big.Int)(nil), output: "80"},
	{val: (*[]string)(nil), output: "C0"},
	{val: (*[10]string)(nil), output: "C0"},
	{val: (*[]interface{})(nil), output: "C0"},
	{val: (*[]struct{ uint })(nil), output: "C0"},
	{val: (*interface{})(nil), output: "C0"},

	// interfaces
	{val: []io.Reader{reader}, output: "C3C20102"}, // the contained value is a struct

	// Encoder
	{val: (*testEncoder)(nil), output: "00000000"},
	{val: &testEncoder{}, output: "00010001000100010001"},
	{val: &testEncoder{errors.New("test error")}, error: "test error"},
	// verify that pointer method testEncoder.EncodeRLP is called for
	// addressable non-pointer values.
	{val: &struct{ TE testEncoder }{testEncoder{}}, output: "CA00010001000100010001"},
	{val: &struct{ TE testEncoder }{testEncoder{errors.New("test error")}}, error: "test error"},
	// verify the error for non-addressable non-pointer Encoder
	{val: testEncoder{}, error: "rlp: game over: unaddressable value of type rlp.testEncoder, EncodeRLP is pointer method"},
	// verify the special case for []byte
	{val: []byteEncoder{0, 1, 2, 3, 4}, output: "C5C0C0C0C0C0"},
}

func runEncTests(t *testing.T, f func(val interface{}) ([]byte, error)) {
	for i, test := range encTests {
		output, err := f(test.val)
		if err != nil && test.error == "" {
			t.Errorf("test %d: unexpected error: %v\nvalue %#v\ntype %T",
				i, err, test.val, test.val)
			continue
		}
		if test.error != "" && fmt.Sprint(err) != test.error {
			t.Errorf("test %d: error mismatch\ngot   %v\nwant  %v\nvalue %#v\ntype  %T",
				i, err, test.error, test.val, test.val)
			continue
		}
		if err == nil && !bytes.Equal(output, unhex(test.output)) {
			t.Errorf("test %d: output mismatch:\ngot   %X\nwant  %s\nvalue %#v\ntype  %T",
				i, output, test.output, test.val, test.val)
		}
	}
}

func TestEncode(t *testing.T) {
	runEncTests(t, func(val interface{}) ([]byte, error) {
		b := new(bytes.Buffer)
		err := Encode(b, val)
		return b.Bytes(), err
	})
}

func TestEncodeToBytes(t *testing.T) {
	runEncTests(t, EncodeToBytes)
}

func TestEncodeToReader(t *testing.T) {
	runEncTests(t, func(val interface{}) ([]byte, error) {
		_, r, err := EncodeToReader(val)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(r)
	})
}

func TestEncodeToReaderPiecewise(t *testing.T) {
	runEncTests(t, func(val interface{}) ([]byte, error) {
		size, r, err := EncodeToReader(val)
		if err != nil {
			return nil, err
		}

		// read output piecewise
		output := make([]byte, size)
		for start, end := 0, 0; start < size; start = end {
			if remaining := size - start; remaining < 3 {
				end += remaining
			} else {
				end = start + 3
			}
			n, err := r.Read(output[start:end])
			end = start + n
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
		}
		return output, nil
	})
}

// This is a regression test verifying that encReader
// returns its encbuf to the pool only once.
func TestEncodeToReaderReturnToPool(t *testing.T) {
	buf := make([]byte, 50)
	wg := new(sync.WaitGroup)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 1000; i++ {
				_, r, _ := EncodeToReader("foo")
				ioutil.ReadAll(r)
				r.Read(buf)
				r.Read(buf)
				r.Read(buf)
				r.Read(buf)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func unhex(str string) []byte {
	b, err := hex.DecodeString(strings.Replace(str, " ", "", -1))
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %q", str))
	}
	return b
}

type simplestruct struct {
	A uint
	B string
}

type recstruct struct {
	I     uint
	Child *recstruct `rlp:"nil"`
}

type tailRaw struct {
	A    uint
	Tail []RawValue `rlp:"tail"`
}

type hasIgnoredField struct {
	A uint
	B uint `rlp:"-"`
	C uint
}

/*

 _____ _   _ _______ ______ _____  _   _          _        ______ _    _ _   _  _____ _______ _____ ____  _   _
|_   _| \ | |__   __|  ____|  __ \| \ | |   /\   | |      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
  | | |  \| |  | |  | |__  | |__) |  \| |  /  \  | |      | |__  | |  | |  \| | |       | |    | || |  | |  \| |
  | | | . ` |  | |  |  __| |  _  /| . ` | / /\ \ | |      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
 _| |_| |\  |  | |  | |____| | \ \| |\  |/ ____ \| |____  | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_____|_| \_|  |_|  |______|_|  \_\_| \_/_/    \_\______| |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
