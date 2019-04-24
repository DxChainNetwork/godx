/*
	Functions Not Tested:
		ReadRequestHeaders -- need getting request from the connection. Moreover, this function's main
		responsibility is to call parseBatchRequest or parseRequest which are tested already


*/

package rpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
)

/*
	Test the function ErrorCode to check if the correct error code was returned
*/
var jsonErrorTest = jsonError{Code: 12, Message: "Test Case"}

func TestErrorCode(t *testing.T) {
	code := (&jsonErrorTest).ErrorCode()
	if code != jsonErrorTest.Code {
		t.Errorf("input %v, got code: %d, expected code: %d",
			jsonErrorTest, code, jsonErrorTest.Code)
	}
}

/*
	Test the function isBatch to check if the RawMessage contains batch message
	Cases:
		1. tab + [] --> true
		2. space + []  --> true
		3. newline + []  --> true
		4. carriage return + []  --> true
		5. []  --> true
		6. message --> false
*/
var jsonMsg1 = json.RawMessage("	[]")
var jsonMsg2 = json.RawMessage(" []")
var jsonMsg3 = json.RawMessage("\n[]")
var jsonMsg4 = json.RawMessage("\r[]")
var jsonMsg5 = json.RawMessage("[]")
var jsonMsg6 = json.RawMessage("message")

func TestIsBatch(t *testing.T) {
	tables := []struct {
		msg     json.RawMessage
		isBatch bool
	}{
		{jsonMsg1, true},
		{jsonMsg2, true},
		{jsonMsg3, true},
		{jsonMsg4, true},
		{jsonMsg5, true},
		{jsonMsg6, false},
	}

	for _, table := range tables {
		result := isBatch(table.msg)
		if result != table.isBatch {
			t.Errorf("input: %s, got %t, expected %t", table.msg, result, table.isBatch)
		}
	}
}

/*
	Test the function checkReqId to check if the passed in request id is valid.
	Criteria:
		* numbers
		* string
*/
var reqId1 = json.RawMessage(`"abc"`)
var reqId2 = json.RawMessage("1234.5")
var reqId3 = json.RawMessage("12345")
var reqId4 = json.RawMessage{}
var reqId5 = json.RawMessage("abc")

func TestCheckReqId(t *testing.T) {
	tables := []struct {
		reqId json.RawMessage
		err   bool
	}{
		{reqId1, false},
		{reqId2, false},
		{reqId3, false},
		{reqId4, true},
		{reqId5, true},
	}

	for _, table := range tables {
		err := checkReqId(table.reqId)
		if err != nil && !table.err {
			t.Errorf("Input %v, got error: %s, expected nil",
				table.reqId, err)
		}

		if err == nil && table.err {
			t.Errorf("Input %v, got error: nil, expected error",
				table.reqId)
		}
	}
}

/*
	Test the function parseRequest to check if the request sent by client can be correctly
	converted into rpcRequest format
*/
var methodCall = jsonrpcMessage{
	Version: "2.0",
	ID:      json.RawMessage("1"),
	Method:  "rpc_modules",
}

var subscriptionCall = jsonrpcMessage{
	Version: "2.0",
	ID:      json.RawMessage("2"),
	Method:  "eth_subscribe",
	Params:  json.RawMessage(`["newHeads"]`),
}

var unsubscribeCall = jsonrpcMessage{
	Version: "2.0",
	ID:      json.RawMessage("3"),
	Method:  "eth_unsubscribe",
}

func TestParseRequest(t *testing.T) {
	tables := []struct {
		msg      jsonrpcMessage
		service  string
		method   string
		isPubSub bool
	}{
		{methodCall, "rpc", "modules", false},
		{subscriptionCall, "eth", "newHeads", true},
		{msg: unsubscribeCall, method: "eth_unsubscribe", isPubSub: true},
	}

	for _, table := range tables {
		req, batch, err := parseRequest(jsonVersionRequest(table.msg))
		if err != nil {
			t.Fatalf("Input Request: %v Error: %s",
				table.msg, err)
		}

		if batch {
			t.Errorf("Input Request: %v should not be batch request",
				table.msg)
		}

		if req[0].service != table.service {
			t.Errorf("Input Request: %v. got service: %s, expected: %s",
				table.msg, req[0].service, table.service)
		}

		if req[0].method != table.method {
			t.Errorf("Input Request: %v. got method: %s, expected: %s",
				table.msg, req[0].method, table.method)
		}
		if req[0].isPubSub != table.isPubSub {
			t.Errorf("Input Request: %v. got isPubSub: %t, expected: %t",
				table.msg, req[0].isPubSub, table.isPubSub)
		}

		if reflect.DeepEqual(req[0].id, table.msg.ID) {
			t.Errorf("Input Request: %v, got id %s, expected: %s",
				table.msg, req[0].id, table.msg.ID)
		}
	}
}

/*
	Test the function parseBatchRequest to check if the batch request sent by client can be correctly
	converted into rpcRequest format
*/
var batchRequestSample = []jsonrpcMessage{methodCall, subscriptionCall, unsubscribeCall}

func TestParseBatchRequest(t *testing.T) {
	results := []struct {
		index    int
		service  string
		method   string
		isPubSub bool
	}{
		{0, "rpc", "modules", false},
		{1, "eth", "newHeads", true},
		{index: 2, method: "eth_unsubscribe", isPubSub: true},
	}
	reqs, batch, err := parseBatchRequest(jsonVersionRequest(batchRequestSample))

	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	if !batch {
		t.Errorf("Input request: batchRequestSample should be batch request")
	}

	for i, req := range reqs {
		if req.service != results[i].service {
			t.Errorf("Input Batch Reuqest Index %v. got service: %s, expected: %s",
				i, req.service, results[i].service)
		}

		if req.method != results[i].method {
			t.Errorf("Input Batch Request Index %v. got method %s, expected %s",
				i, req.method, results[i].service)
		}

		if req.isPubSub != results[i].isPubSub {
			t.Errorf("Input Batch Request Index %v. got method %s, expected %s",
				i, req.method, results[i].service)
		}
	}
}

/*
	Test function parsePositionalArguments to check if the passed in arguments can be
	converted back to their original type and returned back
*/
var integer int
var str string

type ParamType struct {
	name string
}

var rawArgs1 = json.RawMessage(`[1, 2]`)
var rawArgs2 = json.RawMessage(`[1, "rpc_modules"]`)
var rawArgs3 = jsonVersionRequest([]reflect.Value{reflect.ValueOf(ParamType{"mzhang"})})

var intType = reflect.TypeOf(integer)
var strType = reflect.TypeOf(str)
var paramType = reflect.TypeOf(ParamType{})

func TestParsePositionalArguments(t *testing.T) {
	tables := []struct {
		rawArgs json.RawMessage
		types   []reflect.Type
		values  []reflect.Value
	}{
		{rawArgs1, []reflect.Type{intType, intType}, []reflect.Value{reflect.ValueOf(1), reflect.ValueOf(2)}},
		{rawArgs2, []reflect.Type{intType, strType}, []reflect.Value{reflect.ValueOf(1), reflect.ValueOf("rpc_modules")}},
	}

	for _, table := range tables {
		vals, err := parsePositionalArguments(table.rawArgs, table.types)
		if err != nil {
			t.Fatalf("Input Arguments %s. Error: %s", table.rawArgs, err)
		}
		if !reflectSliceComparator(vals, table.values) {
			t.Errorf("Input Arguments %s. Got values: %s, expected: %s",
				table.rawArgs, vals, table.values)
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

func jsonVersionRequest(msg interface{}) json.RawMessage {
	jsonVersion, _ := json.Marshal(msg)
	return jsonVersion
}

func reflectSliceComparator(s1, s2 []reflect.Value) bool {
	for i, v1 := range s1 {
		if v1.Interface() != s2[i].Interface() {
			return false
		}
	}
	return true
}

/*


  ____  _____  _____ _____ _____ _   _          _        _______ ______  _____ _______ _____           _____ ______  _____
 / __ \|  __ \|_   _/ ____|_   _| \ | |   /\   | |      |__   __|  ____|/ ____|__   __/ ____|   /\    / ____|  ____|/ ____|
| |  | | |__) | | || |  __  | | |  \| |  /  \  | |         | |  | |__  | (___    | | | |       /  \  | (___ | |__  | (___
| |  | |  _  /  | || | |_ | | | | . ` | / /\ \ | |         | |  |  __|  \___ \   | | | |      / /\ \  \___ \|  __|  \___ \
| |__| | | \ \ _| || |__| |_| |_| |\  |/ ____ \| |____     | |  | |____ ____) |  | | | |____ / ____ \ ____) | |____ ____) |
 \____/|_|  \_\_____\_____|_____|_| \_/_/    \_\______|    |_|  |______|_____/   |_|  \_____/_/    \_\_____/|______|_____/



*/

type RWC struct {
	*bufio.ReadWriter
}

func (rwc *RWC) Close() error {
	return nil
}

func TestJSONRequestParsing(t *testing.T) {
	server := NewServer()
	service := new(Service)

	if err := server.RegisterName("calc", service); err != nil {
		t.Fatalf("%v", err)
	}

	req := bytes.NewBufferString(`{"id": 1234, "jsonrpc": "2.0", "method": "calc_add", "params": [11, 22]}`)
	var str string
	reply := bytes.NewBufferString(str)
	rw := &RWC{bufio.NewReadWriter(bufio.NewReader(req), bufio.NewWriter(reply))}

	codec := NewJSONCodec(rw)

	requests, batch, err := codec.ReadRequestHeaders()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if batch {
		t.Fatalf("Request isn't a batch")
	}

	if len(requests) != 1 {
		t.Fatalf("Expected 1 request but got %d requests - %v", len(requests), requests)
	}

	if requests[0].service != "calc" {
		t.Fatalf("Expected service 'calc' but got '%s'", requests[0].service)
	}

	if requests[0].method != "add" {
		t.Fatalf("Expected method 'Add' but got '%s'", requests[0].method)
	}

	if rawId, ok := requests[0].id.(*json.RawMessage); ok {
		id, e := strconv.ParseInt(string(*rawId), 0, 64)
		if e != nil {
			t.Fatalf("%v", e)
		}
		if id != 1234 {
			t.Fatalf("Expected id 1234 but got %d", id)
		}
	} else {
		t.Fatalf("invalid request, expected *json.RawMesage got %T", requests[0].id)
	}

	var arg int
	args := []reflect.Type{reflect.TypeOf(arg), reflect.TypeOf(arg)}

	v, err := codec.ParseRequestArguments(args, requests[0].params)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if len(v) != 2 {
		t.Fatalf("Expected 2 argument values, got %d", len(v))
	}

	if v[0].Int() != 11 || v[1].Int() != 22 {
		t.Fatalf("expected %d == 11 && %d == 22", v[0].Int(), v[1].Int())
	}
}

func TestJSONRequestParamsParsing(t *testing.T) {

	var (
		stringT = reflect.TypeOf("")
		intT    = reflect.TypeOf(0)
		intPtrT = reflect.TypeOf(new(int))

		stringV = reflect.ValueOf("abc")
		i       = 1
		intV    = reflect.ValueOf(i)
		intPtrV = reflect.ValueOf(&i)
	)

	var validTests = []struct {
		input    string
		argTypes []reflect.Type
		expected []reflect.Value
	}{
		{`[]`, []reflect.Type{}, []reflect.Value{}},
		{`[]`, []reflect.Type{intPtrT}, []reflect.Value{intPtrV}},
		{`[1]`, []reflect.Type{intT}, []reflect.Value{intV}},
		{`[1,"abc"]`, []reflect.Type{intT, stringT}, []reflect.Value{intV, stringV}},
		{`[null]`, []reflect.Type{intPtrT}, []reflect.Value{intPtrV}},
		{`[null,"abc"]`, []reflect.Type{intPtrT, stringT, intPtrT}, []reflect.Value{intPtrV, stringV, intPtrV}},
		{`[null,"abc",null]`, []reflect.Type{intPtrT, stringT, intPtrT}, []reflect.Value{intPtrV, stringV, intPtrV}},
	}

	codec := jsonCodec{}

	for _, test := range validTests {
		params := (json.RawMessage)([]byte(test.input))
		args, err := codec.ParseRequestArguments(test.argTypes, params)

		if err != nil {
			t.Fatal(err)
		}

		var match []interface{}
		json.Unmarshal([]byte(test.input), &match)

		if len(args) != len(test.argTypes) {
			t.Fatalf("expected %d parsed args, got %d", len(test.argTypes), len(args))
		}

		for i, arg := range args {
			expected := test.expected[i]

			if arg.Kind() != expected.Kind() {
				t.Errorf("expected type for param %d in %s", i, test.input)
			}

			if arg.Kind() == reflect.Int && arg.Int() != expected.Int() {
				t.Errorf("expected int(%d), got int(%d) in %s", expected.Int(), arg.Int(), test.input)
			}

			if arg.Kind() == reflect.String && arg.String() != expected.String() {
				t.Errorf("expected string(%s), got string(%s) in %s", expected.String(), arg.String(), test.input)
			}
		}
	}

	var invalidTests = []struct {
		input    string
		argTypes []reflect.Type
	}{
		{`[]`, []reflect.Type{intT}},
		{`[null]`, []reflect.Type{intT}},
		{`[1]`, []reflect.Type{stringT}},
		{`[1,2]`, []reflect.Type{stringT}},
		{`["abc", null]`, []reflect.Type{stringT, intT}},
	}

	for i, test := range invalidTests {
		if _, err := codec.ParseRequestArguments(test.argTypes, test.input); err == nil {
			t.Errorf("expected test %d - %s to fail", i, test.input)
		}
	}
}
