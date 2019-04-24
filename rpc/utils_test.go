/*
	Function Not Tested:
		idGenerator -- randomly generate id, no need to test since there were no patterns
*/

package rpc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

/*
	Test function isExported to check if the first letter of method name is upper case or lower case

	Cases:
		name == "Exported"   --> true
		name == "isExported" --> false
		name == ""           --> false
*/
func TestIsExported(t *testing.T) {
	tables := []struct {
		name   string
		result bool
	}{
		{"Exported", true},
		{"isExported", false},
		{"", false},
	}

	for _, table := range tables {
		exportedMethod := isExported(table.name)
		if exportedMethod != table.result {
			t.Errorf("input %s, got %t, expected %t", table.name, exportedMethod, table.result)
		}
	}
}

/*
	Test function isExportedOrBuiltinType to check if the variable type if exported, builtin, or private
	Cases:
		builtInType: []byte{}            -- true
		*builtInType: &[]byte{}          -- true
		exportedType: ExportedDataType   -- true
		*exportedType: &ExportedDataType -- true
		privateType: privateDataType     -- false
		*privateType: &privateDataType   -- false
*/
type ExportDataType uint
type privateDataType bool

var testData1 ExportDataType = 10
var testData2 privateDataType = false
var testData3 = []byte{1, 2, 3}

func TestIsExportedOrBuiltinType(t *testing.T) {
	tables := []struct {
		dataType reflect.Type
		result   bool
	}{
		{reflect.TypeOf(testData3), true},
		{reflect.TypeOf(&testData3), true},
		{reflect.TypeOf(testData1), true},
		{reflect.TypeOf(&testData1), true},
		{reflect.TypeOf(testData2), false},
		{reflect.TypeOf(&testData2), false},
	}

	for _, table := range tables {
		returnResult := isExportedOrBuiltinType(table.dataType)
		if returnResult != table.result {
			t.Errorf("input type %s, got %t, expected %t",
				table.dataType, returnResult, table.result)
		}
	}
}

/*
	Test function isContextType to check if the data type is context
	Cases:
		EmptyContextTypeData     -- false the type is context.emptyCtx
		*EmptyContextTypeData    -- true
		NonContextTypeData       -- false
		*NonContextTypeData      -- false

	Discovery: only the pointer context will get type Context
*/
var emptyCtx = context.Background()
var nonCtx = []byte{1, 2, 3}

func TestIsContextType(t *testing.T) {
	tables := []struct {
		dataType reflect.Type
		result   bool
	}{
		{reflect.TypeOf(emptyCtx), false},
		{reflect.TypeOf(&emptyCtx), true},
		{reflect.TypeOf(nonCtx), false},
		{reflect.TypeOf(&nonCtx), false},
	}

	for _, table := range tables {
		returnResult := isContextType(table.dataType)
		if returnResult != table.result {
			t.Errorf("input type %s, got %t, expected %t",
				table.dataType, returnResult, table.result)
		}
	}
}

/*
	Test function isErrorType to check if the data type implemented error interface
	Cases:
		ErrorType (implements error interface)   --> True
		*ErrorType   --> True
		ErrorsType    --> False
		NonErrorType   --> False
		ErrorPtrType   --> False
		*ErrorPtrType  --> False
*/

type ErrorType uint
type ErrorPtrType uint

func (e ErrorType) Error() string {
	e += 1
	return "true"
}

func (e *ErrorPtrType) Error() string {
	*e += 1
	return "false"
}

var (
	ErrorPtrObject ErrorPtrType = 11
	ErrorObject    ErrorType    = 10
	ErrorsType                  = errors.New("")
	NonError                    = 45
)

func TestIsErrorType(t *testing.T) {
	tables := []struct {
		dataType reflect.Type
		isError  bool
	}{
		{reflect.TypeOf(ErrorObject), true},
		{reflect.TypeOf(&ErrorObject), true},
		{reflect.TypeOf(ErrorsType), false},
		{reflect.TypeOf(NonError), false},
		{reflect.TypeOf(ErrorPtrObject), false},
		{reflect.TypeOf(&ErrorPtrObject), false},
	}

	for _, table := range tables {
		result := isErrorType(table.dataType)
		if result != table.isError {
			t.Errorf("input type %s, got result %t, expected result %t",
				table.dataType, result, table.isError)
		}
	}
}

/*
	Test function isSubscriptionType to check if the data type is subscription
	Cases:
		nilSubscriptionPtr --> True
		SubscriptionObj    --> True
		NonSubscription    --> False
*/
var nilSubscriptionPtr = new(Subscription)
var SubscriptionObj = Subscription{"10", "Random", make(chan error)}
var NonSubscription = []string{"test1", "test2"}

func TestIsSubscriptionType(t *testing.T) {
	tables := []struct {
		dataType       reflect.Type
		isSubscription bool
	}{
		{reflect.TypeOf(nilSubscriptionPtr), true},
		{reflect.TypeOf(SubscriptionObj), true},
		{reflect.TypeOf(NonSubscription), false},
	}

	for _, table := range tables {
		result := isSubscriptionType(table.dataType)
		if result != table.isSubscription {
			t.Errorf("input type: %s, got result: %t, expected result: %t",
				table.dataType, result, table.isSubscription)
		}
	}
}

/*
	Test function isPubSub (public subscription) to check if the input method has two returns
	which are type subscription and type error. Moreover, check if the first parameter is context type
	and to check if the function has receiver, which is passed in as

	Cases (included as private methods at the end):
		(receiver) method1(context.Context) {subscription, error}     --> True
		method2(context.Context) {subscription, error}               --> False
		(receiver) method3(context.Context) {error, subscription}     --> False
		(receiver) method4() {subscription, error}     --> False
*/
type testType struct {
	name string
}

func TestIsPubSub(t *testing.T) {
	test := testType{"mzhang"}
	tables := []struct {
		methodType reflect.Type
		isPubSub   bool
	}{
		{reflect.TypeOf(test).Method(0).Type, true},
		{reflect.TypeOf(method2), false},
		{reflect.TypeOf(test).Method(1).Type, false},
		{reflect.TypeOf(test).Method(2).Type, false},
	}

	for _, table := range tables {
		result := isPubSub(table.methodType)
		if result != table.isPubSub {
			t.Errorf("input type: %s, got result: %t, expected result: %t",
				table.methodType, result, table.isPubSub)
		}
	}
}

/*
	Test function formatName to check if the first character of a string will be converted to lower case
	Case:
		name: ""        -- ""
		name: "DxChain" -- "dxChain"
		name: "dxchain" -- "dxchain"
*/
func TestFormatName(t *testing.T) {
	tables := []struct {
		name       string
		conversion string
	}{
		{"", ""},
		{"DxChain", "dxChain"},
		{"dxchain", "dxchain"},
	}

	for _, table := range tables {
		result := formatName(table.name)
		if result != table.conversion {
			t.Errorf("input string %s, got %s, expected %s",
				table.name, result, table.conversion)
		}
	}
}

/*
	Test function NewId to check if the ID generated by the function meets all the requirement
	1. The return result is stringified hex representation of byte slice (all characters are hex)
	2. The length should be smaller than 34, but bigger than 0
		(byte slice is 16 byte long, after hexEncode, *2)
	3. The prefix is 0x
*/
func TestNewId(t *testing.T) {
	hexChars := "0123456789abcdefABCDEF"
	id := string(NewID())

	// if the generated ID is not started with 0x, which is the prefix added to the ID
	if !strings.HasPrefix(id, "0x") {
		t.Fatalf("Invalid id prefix, got id: %s, expected id started with 0x",
			id)
	}

	id = id[2:]

	// check the id length
	if len(id) == 0 || len(id) > 32 {
		t.Fatalf("Invalid id length, got: %d, expected 32", len(id))
	}

	// check id's characters
	for i := 0; i < len(id); i++ {
		// check if the byte can be found within hexChars
		if strings.IndexByte(hexChars, id[i]) == -1 {
			t.Fatalf("unexpected byte, got %c, expected valid hex character", id[i])
		}
	}
}

/*
	Test function suitableCallbacks to check if the methods satisfied the criteria to be called remotely will be
	saved and returned
*/
type callbacksTest struct {
	testing string
}

func TestSuitableCallBacks(t *testing.T) {
	var testVariable = callbacksTest{"test"}
	tables := []struct {
		methodName string
		isCallback bool
		argNum     int
		hasCtx     bool
		errPos     int
	}{
		{"callbackNoReturn", true, 2, false, -1},
		{"callbackOneReturn", true, 2, false, 0},
		{"callbackTwoReturn", true, 2, false, 1},
		{"callbackNoArgs", true, 0, false, -1},
		{"nCallbackNonExported", false, 0, false, -1},
		{"nCallbackErrorReturn", false, 0, false, -1},
		{"nCallbackPrivateTypeArg", false, 0, false, -1},
		{"nCallbackPrivateTypeReturn", false, 0, false, -1},
	}

	for _, table := range tables {
		callbacks, _ := suitableCallbacks(reflect.ValueOf(testVariable), reflect.TypeOf(testVariable))
		if table.isCallback {
			callback := callbacks[table.methodName]
			if len(callback.argTypes) != table.argNum {
				t.Errorf("Input Method Name: %s, got argument length: %d, expected: %d",
					table.methodName, len(callback.argTypes), table.argNum)
			}
			if callback.hasCtx != table.hasCtx {
				t.Errorf("Input Mehtod Name: %s, got hasCtx: %t, expected: %t",
					table.methodName, callback.hasCtx, table.hasCtx)
			}
			if callback.errPos != table.errPos {
				t.Errorf("Input Method Name: %s, got errPoS: %d, expected: %d",
					table.methodName, callback.errPos, table.errPos)
			}
			if callback.isSubscribe {
				t.Errorf("Input Method %s, should not be subscripble",
					table.methodName)
			}
		} else {
			_, ok := callbacks[table.methodName]
			if ok {
				t.Errorf("Input Method %s, should not be callback method",
					table.methodName)
			}
		}
	}
}

/*
	Test function suitableCallbacks to check if the methods satisfied the criteria to be subscribed will be
	saved and returned properly
*/
func TestSuitableSubscribe(t *testing.T) {
	var testSubscribe = testType{"testing"}
	tables := []struct {
		methodName  string
		isSubscribe bool
		argNum      int
		hasCtx      bool
	}{
		{"method1", true, 0, true},
		{"method3", false, 0, true},
		{"method4", false, 0, true},
		{"method5", true, 1, true},
		{"method6", false, 1, true},
	}

	for _, table := range tables {
		_, subscriptions := suitableCallbacks(reflect.ValueOf(testSubscribe), reflect.TypeOf(testSubscribe))
		if table.isSubscribe {
			subscription, ok := subscriptions[table.methodName]
			if !ok {
				t.Fatalf("Input Method: %s, should be subscripable", table.methodName)
			}
			if len(subscription.argTypes) != table.argNum {
				t.Errorf("Input Method: %s, got arg nums: %d, expected: %d",
					table.methodName, len(subscription.argTypes), table.argNum)
			}
			if subscription.hasCtx != table.hasCtx {
				t.Errorf("Input Method: %s, got hasCtx %t, expected: %t",
					table.methodName, subscription.hasCtx, table.hasCtx)
			}
			if subscription.errPos != -1 {
				t.Errorf("Expected input method error position to be -1")
			}

		} else {
			_, ok := subscriptions[table.methodName]
			if ok {
				t.Errorf("Input Method: %s, should not be subscripable", table.methodName)
			}
		}
	}
}

/*
 _____ _   _ _______ ______ _____  _   _          _        ______ _    _ _   _  _____ _______ _____ ____  _   _
|_   _| \ | |__   __|  ____|  __ \| \ | |   /\   | |      |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
  | | |  \| |  | |  | |__  | |__) |  \| |  /  \  | |      | |__  | |  | |  \| | |       | |    | || |  | |  \| |
  | | | . ` |  | |  |  __| |  _  /| . ` | / /\ \ | |      |  __| | |  | | . ` | |       | |    | || |  | | . ` |
 _| |_| |\  |  | |  | |____| | \ \| |\  |/ ____ \| |____  | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_____|_| \_|  |_|  |______|_|  \_\_| \_/_/    \_\______| |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func (t testType) Method1(ctx context.Context) (Subscription, error) {
	return Subscription{"10", "20", make(chan error)}, nil
}

func method2(ctx context.Context) (Subscription, error) {
	return Subscription{"10", "20", make(chan error)}, nil
}

func (t testType) Method3(ctx context.Context) (error, Subscription) {
	return nil, Subscription{"10", "20", make(chan error)}
}

func (t testType) Method4() (Subscription, error) {
	return Subscription{"10", "20", make(chan error)}, nil
}

func (t testType) Method5(ctx context.Context, a int) (Subscription, error) {
	return Subscription{"10", "20", make(chan error)}, nil
}

func (t testType) Method6(ctx context.Context, a callbacksTest) (Subscription, error) {
	return Subscription{"10", "20", make(chan error)}, nil
}

// test suitableCallBacks function callbacks
func (c callbacksTest) CallbackNoReturn(a, b int) {
	fmt.Println("returns nothing")
}

func (c callbacksTest) CallbackOneReturn(a, b int) error {
	return nil
}

func (c callbacksTest) CallbackTwoReturn(a, b int) (int, error) {
	return 0, nil
}

func (c callbacksTest) CallbackNoArgs() {
	fmt.Println("no args, no return")
}

func (c callbacksTest) nCallbackNonExported(a, b int) error {
	return nil
}

func (c callbacksTest) nCallbackErrorReturn(a, b int) (error, int) {
	return nil, 0
}

func (c callbacksTest) nCallbackPrivateTypeArg(a callbacksTest) error {
	return nil
}

func (c callbacksTest) nCallbackPrivateTypeReturn(a int) callbacksTest {
	return c
}
