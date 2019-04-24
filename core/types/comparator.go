package types

import (
	"bytes"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"math/big"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
)

var checkEqualityHandler map[reflect.Type]func(*testing.T, string, string, reflect.Value, reflect.Value)

var dumper = spew.ConfigState{DisableMethods: true, Indent: "    "}

func init() {
	checkEqualityHandler = make(map[reflect.Type]func(*testing.T, string, string, reflect.Value, reflect.Value))
	checkEqualityHandler[reflect.TypeOf(new(big.Int))] = checkBigIntEqual
	checkEqualityHandler[reflect.TypeOf([]byte{})] = checkByteSliceEqual
	checkEqualityHandler[reflect.TypeOf(common.Hash{})] = checkHashEqual
	checkEqualityHandler[reflect.TypeOf(common.Address{})] = checkAddressEqual
	checkEqualityHandler[reflect.TypeOf(txdata{})] = checkTxdataEqual
	checkEqualityHandler[reflect.TypeOf(Transaction{})] = checkTransactionEqual
	checkEqualityHandler[reflect.TypeOf(Log{})] = checkLogEqual
	checkEqualityHandler[reflect.TypeOf(Receipt{})] = checkReceiptEqual
	checkEqualityHandler[reflect.TypeOf(Header{})] = checkHeaderEqual
	checkEqualityHandler[reflect.TypeOf(Body{})] = checkBodyEqual
	checkEqualityHandler[reflect.TypeOf(Block{})] = checkBlockEqual
}

func CheckError(t *testing.T, dataName string, got, want error) {
	if want == nil && got != nil {
		t.Fatalf("%s Got unexpected error.\nGot %s\nWant nil", dataName, got.Error())
	}
	if want != nil && got == nil {
		t.Fatalf("%s does not get expected error.\nGot nil\nWant %s", dataName, want.Error())
	}
	if want != nil && got != nil && want.Error() != got.Error() {
		t.Fatalf("%s does not get expected error.\nGot %s\nWant %s", dataName, got.Error(), want.Error())
	}
}

func CheckEquality(t *testing.T, inputName, fieldName string, got, want interface{}) {
	va := reflect.ValueOf(got)
	vb := reflect.ValueOf(want)
	checkEquality(t, inputName, fieldName, va, vb)
}

// checkEquality Check equality for two variables a and b. If not equal, print the error message
// testName: name of the test e.g TestMyFunction
// inputName: name of the input data e.g OK
// fieldName: name of the field e.g TxHash
// This function depend on some predefined equal function.
// Notice when comparing two structures, only the first field difference will be printed as error.
func checkEquality(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	if got.Type() != want.Type() {
		t.Fatalf("%s.%s have uncomparable types\n%v / %v", inputName, fieldName, got.Type(),
			want.Type())
	}
	if !got.CanInterface() || !want.CanInterface() {
		// unexported fields are not compared
		return
	}
	if got.Kind() == reflect.Ptr && got.Type() != reflect.TypeOf(new(big.Int)) {
		// Deal with ptr
		if got.IsNil() {
			got = reflect.New(got.Type())
		}
		if want.IsNil() {
			want = reflect.New(want.Type())
		}

		// If the pointer is the same, underlying struct must be equal.
		if got.Pointer() == want.Pointer() {
			return
		}
		checkEquality(t, inputName, fieldName, got.Elem(), want.Elem())
	} else if got.Kind() == reflect.Slice && got.Type() != reflect.TypeOf([]byte{}) {
		// deal with slice
		if got.Len() != want.Len() {
			t.Fatalf("%s.%s have unexpected length\nGot %d\nWant %d", inputName,
				fieldName, got.Len(), want.Len())
		}
		for i := 0; i != got.Len(); i++ {
			checkEquality(t, inputName, fieldName+fmt.Sprintf("[%d]", i),
				got.Index(i), want.Index(i))
		}
	} else if fn, ok := checkEqualityHandler[got.Type()]; ok {
		// type comparison method is defined
		fn(t, inputName, fieldName, got, want)

	} else if got.Kind() == reflect.Struct {
		// By default struct check all public fields
		checkStructFullEqual(t, inputName, fieldName, got, want)

	} else {
		// Other types use reflect.DeepEqual to check equality.
		if !reflect.DeepEqual(got.Interface(), want.Interface()) {
			t.Fatalf("%s.%s unexpected value\nGot %sWant %s", inputName, fieldName,
				dumper.Sdump(got.Interface()), dumper.Sdump(want.Interface()))
		}
	}
}

func checkBigIntEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	got.Interface()

	a := got.Interface().(*big.Int)
	b := want.Interface().(*big.Int)
	if a == nil {
		a = big.NewInt(0)
	}
	if b == nil {
		b = big.NewInt(0)
	}
	if a.Cmp(b) != 0 {
		t.Fatalf("%s.%s unexpected value\nGot %v\nWant %v", inputName, fieldName, a, b)
	}
}

// checkByteSliceEqual check whether two byte slices are equal.
// If not equal, raise error.
func checkByteSliceEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	a := got.Interface().([]byte)
	b := want.Interface().([]byte)
	if a == nil {
		a = []byte{}
	}
	if b == nil {
		b = []byte{}
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("%s.%s unexpected value\nGot %x\nWant %x", inputName, fieldName, a, b)
	}
}

func checkHashEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	a := got.Interface().(common.Hash)
	b := want.Interface().(common.Hash)
	if a != b {
		t.Fatalf("%s.%s unexpected value\nGot %x\nWant %x", inputName, fieldName, a, b)
	}
}

func checkAddressEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	a := got.Interface().(common.Address)
	b := want.Interface().(common.Address)
	if a != b {
		t.Fatalf("%s.%s unexpected value\nGot %x\nWant %x", inputName, fieldName, a, b)
	}
}

// checkStructFullEqual check got == want in all fields
func checkStructFullEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	checkStructEqual(t, inputName, fieldName, got, want, nil)
}

// checkStructEqual will check whether got equals want without skipFields
func checkStructEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value, skipField map[string]struct{}) {
	for i := 0; i != got.NumField(); i++ {
		field := got.Type().Field(i).Name
		if _, ok := skipField[field]; ok || got.Field(i).CanInterface() {
			// The field is skipped
			continue
		}
		fa := got.Field(i)
		fb := want.Field(i)
		checkEquality(t, inputName, fieldName+"."+field, fa, fb)
	}
}

// checkTxdataEqual recursively call checkEquality to check equality for
// AccountNonce, Price, GasLimit, Recipient, Amount, Payload, V, R, S field
// Do not check the Hash value
func checkTxdataEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	skip := map[string]struct{}{"Hash": {}}
	checkStructEqual(t, inputName, fieldName, got, want, skip)
}

// Transaction just check for data
func checkTransactionEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	dataA := got.Interface().(Transaction)
	dataB := want.Interface().(Transaction)
	checkTxdataEqual(t, inputName, fieldName+".data", reflect.ValueOf(dataA.data),
		reflect.ValueOf(dataB.data))
}

// For receipt, check all fields equals
func checkReceiptEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	checkStructFullEqual(t, inputName, fieldName, got, want)
}

// For log, compare all fields
func checkLogEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	checkStructFullEqual(t, inputName, fieldName, got, want)
}

func checkHeaderEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	checkStructFullEqual(t, inputName, fieldName, got, want)
}

func checkBodyEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	checkStructFullEqual(t, inputName, fieldName, got, want)
}

func checkBlockEqual(t *testing.T, inputName, fieldName string, got, want reflect.Value) {
	ba := got.Interface().(Block)
	bb := want.Interface().(Block)
	checkEquality(t, inputName, fieldName+".header", reflect.ValueOf(ba.header),
		reflect.ValueOf(bb.header))

	checkEquality(t, inputName, fieldName+".uncles", reflect.ValueOf(ba.uncles),
		reflect.ValueOf(bb.uncles))

	checkEquality(t, inputName, fieldName+".transactions", reflect.ValueOf(ba.transactions),
		reflect.ValueOf(bb.transactions))

	checkEquality(t, inputName, fieldName+".td", reflect.ValueOf(ba.td),
		reflect.ValueOf(bb.td))
}
